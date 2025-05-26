package breaker

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
)

// Environment variable names
const (
	EnvOpsGenieAPIKey = "OPSGENIE_API_KEY"
	EnvOpsGenieRegion = "OPSGENIE_REGION"
	EnvOpsGenieAPIURL = "OPSGENIE_API_URL"
	EnvEnvironment    = "Environment" // Exact variable name as requested
)

// MandatoryFieldsValidationError represents validation errors for mandatory fields
type MandatoryFieldsValidationError struct {
	MissingFields []string
	InvalidFields map[string]string
}

func (e *MandatoryFieldsValidationError) Error() string {
	return fmt.Sprintf("Mandatory fields validation failed - Missing: %v, Invalid: %v",
		e.MissingFields, e.InvalidFields)
}

// MemoryStatus represents the current memory status of the application
type MemoryStatus struct {
	CurrentUsage float64
	Threshold    float64
	TotalMemory  uint64
	UsedMemory   uint64
	OK           bool
}

// Variables for implementing the singleton pattern
var (
	opsgenieClientInstance *OpsGenieClient
	opsgenieClientOnce     sync.Once
	opsgenieClientMutex    sync.Mutex
)

// GetOpsGenieClient returns a singleton instance of the OpsGenie client
func GetOpsGenieClient(config *OpsGenieConfig) *OpsGenieClient {
	opsgenieClientMutex.Lock()
	defer opsgenieClientMutex.Unlock()

	needsNew := opsgenieClientInstance == nil

	if !needsNew && opsgenieClientInstance.config != nil {
		if opsgenieClientInstance.config.AlertCooldownSeconds != config.AlertCooldownSeconds {
			log.Printf("OpsGenie configuration has changed, recreating client")
			needsNew = true
		}
	}

	if needsNew {
		opsgenieClientInstance = NewOpsGenieClient(config)
		err := opsgenieClientInstance.Initialize()
		if err != nil {
			log.Printf("Error initializing OpsGenie client: %v", err)
		}
	}

	return opsgenieClientInstance
}

// OpsGenieClient wraps the OpsGenie SDK client and provides methods to interact with OpsGenie
type OpsGenieClient struct {
	config        *OpsGenieConfig
	alertClient   *alert.Client
	lastAlertTime map[string]time.Time
	alertSent     map[string]bool
	mutex         sync.RWMutex
	initialized   bool
	environment   Environment
}

// NewOpsGenieClient creates a new OpsGenie client with the given configuration
func NewOpsGenieClient(config *OpsGenieConfig) *OpsGenieClient {
	if config == nil {
		config = &OpsGenieConfig{Enabled: false}
	}

	return &OpsGenieClient{
		config:        config,
		lastAlertTime: make(map[string]time.Time),
		alertSent:     make(map[string]bool),
	}
}

// ValidateMandatoryFields validates that all mandatory fields are present and valid
func (o *OpsGenieClient) ValidateMandatoryFields() *MandatoryFieldsValidationError {
	if o == nil || o.config == nil {
		return &MandatoryFieldsValidationError{
			MissingFields: []string{"config"},
		}
	}

	var missingFields []string
	invalidFields := make(map[string]string)

	// Validate Team
	if o.getTeamNameWithFallback() == "unknown-team" || o.config.Team == "" {
		missingFields = append(missingFields, "team")
	}

	// Validate Environment
	env := o.getEnvironmentWithFallback()
	if env == "unknown" || env == "" {
		missingFields = append(missingFields, "environment")
	}

	// Validate BookmakerID
	bookmakerID := o.getBookmakerIDWithFallback()
	if bookmakerID == "unknown" || bookmakerID == "" {
		missingFields = append(missingFields, "bookmaker_id")
	}

	// Validate Host
	hostname := o.getHostnameWithFallback()
	if hostname == "unknown" || hostname == "" {
		missingFields = append(missingFields, "hostname")
	}

	// Validate Business
	business := o.getBusinessWithFallback()
	if business == "" {
		missingFields = append(missingFields, "business")
	}

	// Additional validations for field formats
	if len(env) > 20 {
		invalidFields["environment"] = "too long (max 20 characters)"
	}

	if len(bookmakerID) > 50 {
		invalidFields["bookmaker_id"] = "too long (max 50 characters)"
	}

	// Return error if any issues found
	if len(missingFields) > 0 || len(invalidFields) > 0 {
		return &MandatoryFieldsValidationError{
			MissingFields: missingFields,
			InvalidFields: invalidFields,
		}
	}

	return nil
}

// Enhanced getter methods with better fallbacks
func (o *OpsGenieClient) getTeamNameWithFallback() string {
	if o == nil || o.config == nil {
		return "unknown-team"
	}

	if o.config.Team != "" {
		return o.config.Team
	}

	if envTeam := os.Getenv("OPSGENIE_TEAM"); envTeam != "" {
		return envTeam
	}

	return "unknown-team"
}

func (o *OpsGenieClient) getEnvironmentWithFallback() string {
	if o == nil || o.config == nil {
		return "unknown"
	}

	// Priority order with better fallbacks
	if o.config.Environment != "" {
		return strings.ToUpper(o.config.Environment)
	}

	// Check for the specific "Environment" environment variable
	if envVar := os.Getenv("Environment"); envVar != "" {
		return strings.ToUpper(envVar)
	}

	// Additional fallbacks
	if envVar := os.Getenv("DEPLOYMENT_ENV"); envVar != "" {
		return strings.ToUpper(envVar)
	}

	if o.config.APINamespace != "" {
		return strings.ToUpper(o.config.APINamespace)
	}

	// Try to detect from hostname patterns
	if hostname, err := os.Hostname(); err == nil {
		if strings.Contains(hostname, "prod") {
			return "PROD"
		}
		if strings.Contains(hostname, "staging") {
			return "STAGING"
		}
		if strings.Contains(hostname, "dev") {
			return "DEV"
		}
	}

	return "unknown"
}

func (o *OpsGenieClient) getBookmakerIDWithFallback() string {
	if o == nil || o.config == nil {
		return "unknown"
	}

	// Priority order with environment variable fallbacks
	if o.config.BookmakerID != "" {
		return o.config.BookmakerID
	}

	if o.config.ProjectID != "" {
		return o.config.ProjectID
	}

	// Try multiple environment variables
	for _, envVar := range []string{"BOOKMAKER_ID", "PROJECT_ID", "CLIENT_ID", "SERVICE_ID"} {
		if value := os.Getenv(envVar); value != "" {
			return value
		}
	}

	// Use API name as fallback
	if o.config.APIName != "" {
		return o.config.APIName
	}

	return "unknown"
}

func (o *OpsGenieClient) getHostnameWithFallback() string {
	if o == nil || o.config == nil {
		return "unknown"
	}

	// Priority order with multiple fallbacks
	if o.config.HostOverride != "" {
		return o.config.HostOverride
	}

	if o.config.Hostname != "" {
		return o.config.Hostname
	}

	// Try multiple environment variables
	for _, envVar := range []string{"HOSTNAME", "HOST", "CONTAINER_NAME", "POD_NAME"} {
		if value := os.Getenv(envVar); value != "" {
			return value
		}
	}

	// Try to get system hostname
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}

	// Last resort - try to get from /etc/hostname
	if data, err := os.ReadFile("/etc/hostname"); err == nil {
		if hostname := strings.TrimSpace(string(data)); hostname != "" {
			return hostname
		}
	}

	return "unknown"
}

func (o *OpsGenieClient) getBusinessWithFallback() string {
	if o == nil || o.config == nil {
		return "internal" // Safe default
	}

	if o.config.Business != "" {
		return o.config.Business
	}

	if o.config.BusinessUnit != "" {
		return o.config.BusinessUnit
	}

	// Try environment variables
	for _, envVar := range []string{"BUSINESS_UNIT", "BUSINESS", "DEPARTMENT"} {
		if value := os.Getenv(envVar); value != "" {
			return value
		}
	}

	return "internal" // Safe default
}

func (o *OpsGenieClient) getAdditionalContext() string {
	if o == nil || o.config == nil {
		return ""
	}
	return o.config.AdditionalContext
}

func (o *OpsGenieClient) getSourceWithFallback() string {
	if o == nil || o.config == nil {
		return "go-breaker"
	}

	if o.config.Source != "" {
		return o.config.Source
	}

	return "go-breaker"
}

// buildMandatoryFieldsWithFallbacks creates mandatory fields with intelligent fallbacks
func (o *OpsGenieClient) buildMandatoryFieldsWithFallbacks() map[string]string {
	fields := map[string]string{
		"Team":        o.getTeamNameWithFallback(),
		"Environment": o.getEnvironmentWithFallback(),
		"BookmakerId": o.getBookmakerIDWithFallback(),
		"Host":        o.getHostnameWithFallback(),
		"Business":    o.getBusinessWithFallback(),
	}

	// Add AdditionalContext if provided
	if additionalContext := o.getAdditionalContext(); additionalContext != "" {
		fields["AdditionalContext"] = additionalContext
	}

	return fields
}

// Initialize sets up the OpsGenie client and validates the API key
func (o *OpsGenieClient) Initialize() error {
	if o == nil {
		return fmt.Errorf("OpsGenieClient is nil")
	}

	// If it is disabled, do nothing
	if o.config == nil || !o.config.Enabled {
		log.Printf("OpsGenie client is disabled, skipping initialization")
		return nil // Return without error but without initializing
	}

	// Validate mandatory fields at initialization
	if err := o.ValidateMandatoryFields(); err != nil {
		log.Printf("WARNING: Mandatory fields validation failed during initialization: %v", err)
		log.Printf("Alerts will be sent with fallback values. Please configure missing fields.")
	}

	o.validateTagsConfiguration()

	// Log current mandatory fields status
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()
	log.Printf("OpsGenie initialized with mandatory fields: %+v", mandatoryFields)

	// First check for API key in environment variables
	apiKey := os.Getenv(EnvOpsGenieAPIKey)
	if apiKey == "" {
		apiKey = o.config.APIKey
		if apiKey == "" {
			return fmt.Errorf("OpsGenie API key not found in environment or config")
		}
		log.Println("Warning: Using OpsGenie API key from config file. For security, consider using the OPSGENIE_API_KEY environment variable instead.")
	}

	// Set up the client configuration
	cfg := &client.Config{ApiKey: apiKey}

	// Check for region in environment
	region := os.Getenv(EnvOpsGenieRegion)
	if region == "" {
		region = o.config.Region
	}

	// Set the API URL based on region
	apiUrl := "https://api.opsgenie.com" // Default to US region
	if region == "eu" {
		apiUrl = "https://api.eu.opsgenie.com"
	}

	// Allow custom API URL override
	customURL := os.Getenv(EnvOpsGenieAPIURL)
	if customURL != "" {
		apiUrl = customURL
	} else if o.config.APIURL != "" {
		apiUrl = o.config.APIURL
	}

	log.Printf("Using OpsGenie API URL: %s", apiUrl)

	// Create the alert client
	alertClient, err := alert.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create OpsGenie alert client: %v", err)
	}

	o.alertClient = alertClient

	// Test the connection to validate API key
	err = o.TestConnection()
	if err != nil {
		return fmt.Errorf("failed to connect to OpsGenie: %v", err)
	}

	log.Println("Successfully connected to OpsGenie API")
	o.initialized = true
	return nil
}

// getPriorityForEnvironment returns the appropriate priority for the current environment
func (o *OpsGenieClient) getPriorityForEnvironment() alert.Priority {
	if o == nil || o.config == nil {
		return alert.P3
	}

	var priorityStr = o.config.Priority

	if priorityStr == "" {
		priorityStr = "P3"
	}

	switch priorityStr {
	case "P1":
		return alert.P1
	case "P2":
		return alert.P2
	case "P3":
		return alert.P3
	case "P4":
		return alert.P4
	case "P5":
		return alert.P5
	default:
		log.Printf("Invalid priority '%s', defaulting to P3", priorityStr)
		return alert.P3
	}
}

// TestConnection tests the connection to OpsGenie by listing alerts
func (o *OpsGenieClient) TestConnection() error {
	if o == nil || o.alertClient == nil {
		return fmt.Errorf("OpsGenie client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	listReq := &alert.ListAlertRequest{
		Limit: 1,
	}

	_, err := o.alertClient.List(ctx, listReq)
	return err
}

// IsInitialized returns whether the OpsGenie client has been successfully initialized
func (o *OpsGenieClient) IsInitialized() bool {
	if o == nil {
		return false
	}
	return o.initialized
}

// IsOnCooldown checks if an alert type is still in its cooldown period
func (o *OpsGenieClient) IsOnCooldown(alertType string) bool {
	if o == nil {
		log.Printf("Cannot check cooldown for %s: OpsGenie client is nil", alertType)
		return false
	}

	if o.config.AlertCooldownSeconds <= 0 {
		log.Printf("COOLDOWN CHECK: No cooldown for %s (cooldown disabled)", alertType)
		return false
	}

	o.mutex.RLock()
	defer o.mutex.RUnlock()

	lastAlertTime, exists := o.lastAlertTime[alertType]
	if !exists {
		log.Printf("COOLDOWN CHECK: No cooldown for %s (first occurrence)", alertType)
		return false
	}

	cooldownDuration := time.Duration(o.config.AlertCooldownSeconds) * time.Second
	now := time.Now()
	cooldownEnds := lastAlertTime.Add(cooldownDuration)
	stillInCooldown := now.Before(cooldownEnds)

	if stillInCooldown {
		timeRemaining := cooldownEnds.Sub(now).Seconds()
		log.Printf("COOLDOWN CHECK: Alert %s is still in cooldown for %.1f more seconds", alertType, timeRemaining)
	} else {
		log.Printf("COOLDOWN CHECK: Cooldown period for %s has expired", alertType)
	}

	return stillInCooldown
}

// RecordAlert records when an alert was sent to enforce cooldown periods
func (o *OpsGenieClient) RecordAlert(alertType string) {
	if o == nil {
		log.Printf("Cannot record alert time for %s: OpsGenie client is nil", alertType)
		return
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	now := time.Now()
	o.lastAlertTime[alertType] = now
	o.alertSent[alertType] = true
	log.Printf("COOLDOWN START: Recorded alert %s at %v with %d second cooldown",
		alertType, now.Format(time.RFC3339), o.config.AlertCooldownSeconds)
}

// hasAlertBeenSent checks if this alert type has been sent before
func (o *OpsGenieClient) hasAlertBeenSent(alertType string) bool {
	if o == nil {
		return false
	}

	o.mutex.RLock()
	defer o.mutex.RUnlock()

	sent, exists := o.alertSent[alertType]
	return exists && sent
}

// determineAlertKey creates a unique key for different alerts
func (o *OpsGenieClient) determineAlertKey(alertType string, details string) string {
	return fmt.Sprintf("%s-%s-%s", o.getAPIIdentifier(), alertType, details)
}

// getAPIIdentifier gets a string that uniquely identifies the API for alerts
func (o *OpsGenieClient) getAPIIdentifier() string {
	if o == nil || o.config == nil {
		return "unknown-api"
	}

	if o.config.APIName != "" {
		if o.config.APINamespace != "" {
			return fmt.Sprintf("%s/%s", o.config.APINamespace, o.config.APIName)
		}
		return o.config.APIName
	}

	return o.config.Source
}

// processAndValidateTags Process the simple tags and marks those that have no key format: Value
func (o *OpsGenieClient) processAndValidateTags(alertType string) []string {
	var processedTags []string

	//Process configuration tags
	for _, tag := range o.config.Tags {
		processedTag := o.processTag(tag)
		processedTags = append(processedTags, processedTag)
	}

	// Add automatic system tags
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	systemTags := []string{
		fmt.Sprintf("Environment:%s", strings.ToLower(mandatoryFields["Environment"])),
		fmt.Sprintf("BookmakerId:%s", mandatoryFields["BookmakerId"]),
		fmt.Sprintf("Host:%s", mandatoryFields["Host"]),
		fmt.Sprintf("Business:%s", mandatoryFields["Business"]),
		fmt.Sprintf("Team:%s", mandatoryFields["Team"]),
		fmt.Sprintf("AlertType:%s", alertType),
	}

	// Add system tags
	processedTags = append(processedTags, systemTags...)

	// Add specific service tags if available
	if o.config.APIName != "" {
		processedTags = append(processedTags, fmt.Sprintf("Service:%s", o.config.APIName))
	}

	if o.config.ServiceTier != "" {
		processedTags = append(processedTags, fmt.Sprintf("Tier:%s", o.config.ServiceTier))
	}

	// Add additional context if available
	if additionalContext := o.getAdditionalContext(); additionalContext != "" {
		processedTags = append(processedTags, fmt.Sprintf("Context:%s", additionalContext))
	}

	return processedTags
}

// processTag Process an individual tag and the brand if it does not have key format: Value
func (o *OpsGenieClient) processTag(tag string) string {
	// Verify if the tag has "Key: Value" format
	if strings.Contains(tag, ":") {
		parts := strings.SplitN(tag, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
			// Valid Tag with Key format: Value
			return tag
		}
	}

	// Tag without Key format: Value - Mark it
	return fmt.Sprintf("**TAG_KEY_UNDEFINED**:%s", tag)
}

// buildEnhancedTags processes and validates tags for an alert, adding both user-defined
// // and system-generated tags. It ensures that all tags follow a valid "key:value" format
// // and marks those without a key as "**TAG_KEY_UNDEFINED**".
// //
// // Parameters:
// //   - alertType: A string representing the type of alert (e.g., "circuit-open", "memory-threshold").
// //
// // Returns:
// //   - A slice of strings containing the processed and validated tags, including both
// //     user-defined and system-generated tags.
func (o *OpsGenieClient) buildEnhancedTags(alertType string) []string {
	return o.processAndValidateTags(alertType)
}

// validateTagsConfiguration Valida the tag configuration and shows warnings
func (o *OpsGenieClient) validateTagsConfiguration() {
	if o == nil || o.config == nil {
		return
	}

	var undefinedTags []string
	var validTags []string

	for _, tag := range o.config.Tags {
		if strings.Contains(tag, ":") {
			parts := strings.SplitN(tag, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != "" {
				validTags = append(validTags, tag)
			} else {
				undefinedTags = append(undefinedTags, tag)
			}
		} else {
			undefinedTags = append(undefinedTags, tag)
		}
	}

	if len(validTags) > 0 {
		log.Printf("✅ Valid key:value tags: %v", validTags)
	}

	if len(undefinedTags) > 0 {
		log.Printf("⚠️  Tags without key:value format (will be marked as **TAG_KEY_UNDEFINED**): %v", undefinedTags)
		log.Printf("💡 Consider using format like 'Component:circuit-breaker' instead of just 'circuit-breaker'")
	}
}

// buildEnhancedDetails creates comprehensive details map with mandatory and optional fields
func (o *OpsGenieClient) buildEnhancedDetails(alertType string, specificDetails map[string]string) map[string]string {
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	details := make(map[string]string)

	// Add mandatory fields
	for key, value := range mandatoryFields {
		details[key] = value
	}

	// Add standard API information
	details["API Name"] = o.config.APIName
	details["API Version"] = o.config.APIVersion
	details["API Namespace"] = o.config.APINamespace
	details["API Owner"] = o.config.APIOwner
	details["API Priority"] = o.config.APIPriority
	details["Alert Type"] = alertType
	details["Source"] = o.config.Source

	// Add system information
	details["Go Version"] = runtime.Version()
	details["Architecture"] = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	details["Goroutines"] = fmt.Sprintf("%d", runtime.NumGoroutine())

	// Add timestamp
	details["Alert Timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Add specific alert details
	for key, value := range specificDetails {
		details[key] = value
	}

	return details
}

// buildEnhancedDescription creates detailed description with all context
func (o *OpsGenieClient) buildEnhancedDescription() string {
	if o == nil || o.config == nil {
		return ""
	}

	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	// Prepare additional context section
	additionalContextSection := ""
	if additionalContext := mandatoryFields["AdditionalContext"]; additionalContext != "" {
		additionalContextSection = fmt.Sprintf("• Additional Context: %s\n", additionalContext)
	}

	description := fmt.Sprintf(`Circuit Breaker Alert Details:

MANDATORY FIELDS:
• Team: %s
• Environment: %s  
• Bookmaker ID: %s
• Host: %s
• Business: %s
%s
SERVICE INFORMATION:
• API Name: %s
• API Version: %s
• Namespace: %s
• Owner: %s
• Priority: %s

SYSTEM INFORMATION:
• Hostname: %s
• Runtime: Go %s
• Architecture: %s/%s
`,
		mandatoryFields["Team"],
		mandatoryFields["Environment"],
		mandatoryFields["BookmakerId"],
		mandatoryFields["Host"],
		mandatoryFields["Business"],
		additionalContextSection,
		o.config.APIName,
		o.config.APIVersion,
		o.config.APINamespace,
		o.config.APIOwner,
		o.config.APIPriority,
		mandatoryFields["Host"],
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)

	// Add dependencies if available
	if len(o.config.APIDependencies) > 0 {
		description += "\nDEPENDENCIES:\n"
		for _, dep := range o.config.APIDependencies {
			description += fmt.Sprintf("• %s\n", dep)
		}
	}

	// Add endpoints if available
	if len(o.config.APIEndpoints) > 0 {
		description += "\nPROTECTED ENDPOINTS:\n"
		for _, endpoint := range o.config.APIEndpoints {
			description += fmt.Sprintf("• %s\n", endpoint)
		}
	}

	// Add contact information if available
	if o.config.ContactDetails.PrimaryContact != "" {
		description += fmt.Sprintf("\nCONTACT INFORMATION:\n")
		description += fmt.Sprintf("• Primary Contact: %s\n", o.config.ContactDetails.PrimaryContact)

		if o.config.ContactDetails.EscalationTeam != "" {
			description += fmt.Sprintf("• Escalation Team: %s\n", o.config.ContactDetails.EscalationTeam)
		}

		if o.config.ContactDetails.SlackChannel != "" {
			description += fmt.Sprintf("• Slack Channel: %s\n", o.config.ContactDetails.SlackChannel)
		}
	}

	return description
}

// createValidatedAlertRequest creates an alert request with all mandatory fields validated
func (o *OpsGenieClient) createValidatedAlertRequest(alertType, message, description string, specificDetails map[string]string) (*alert.CreateAlertRequest, error) {
	// First validate mandatory fields
	if err := o.ValidateMandatoryFields(); err != nil {
		log.Printf("WARNING: Mandatory fields validation failed: %v", err)
		// Continue with warning but use fallback values
	}

	// Build mandatory fields (with fallbacks)
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	// Build enhanced tags
	tags := o.buildEnhancedTags(alertType)

	// Build enhanced details
	details := o.buildEnhancedDetails(alertType, specificDetails)

	// Get priority
	priority := o.getPriorityForEnvironment()

	// Create the alert request
	req := &alert.CreateAlertRequest{
		Message:     message,
		Description: description,
		Alias:       o.createUniqueAlertIdentifier(alertType),
		Source:      o.getSourceWithFallback(),
		Priority:    priority,
		Tags:        tags,
		Details:     details,
	}

	// Add team responder if valid
	teamName := mandatoryFields["Team"]
	if teamName != "unknown-team" && teamName != "" {
		req.Responders = []alert.Responder{
			{
				Type: "team",
				Name: teamName,
			},
		}
	}

	// Log the alert details for debugging
	log.Printf("Creating OpsGenie alert with mandatory fields: Team=%s, Environment=%s, BookmakerID=%s, Host=%s, Business=%s",
		mandatoryFields["Team"],
		mandatoryFields["Environment"],
		mandatoryFields["BookmakerId"],
		mandatoryFields["Host"],
		mandatoryFields["Business"])

	return req, nil
}

// createUniqueAlertIdentifier creates a unique identifier for the alert
func (o *OpsGenieClient) createUniqueAlertIdentifier(alertType string) string {
	apiID := o.getAPIIdentifier()
	return fmt.Sprintf("%s-%s", apiID, alertType)
}

// memoryStatusString returns a string representation of memory status
func memoryStatusString(memoryOK bool) string {
	if memoryOK {
		return "OK"
	}
	return "THRESHOLD EXCEEDED"
}

// SendBreakerOpenAlert sends an alert when the circuit breaker opens
func (o *OpsGenieClient) SendBreakerOpenAlert(latency int64, memoryOK bool, waitTime int) error {
	if o == nil || !o.config.Enabled || !o.config.TriggerOnOpen {
		return nil
	}

	if !o.IsInitialized() {
		log.Printf("OpsGenie client not initialized or not enabled for environment, skipping alert")
		return nil
	}

	// Check cooldown
	alertType := "circuit-open"
	details := fmt.Sprintf("latency-%dms-%s-wait%ds", latency, memoryStatusString(memoryOK), waitTime)
	alertKey := o.determineAlertKey(alertType, details)

	if o.IsOnCooldown(alertKey) {
		log.Printf("Skipping alert for %s due to cooldown period", alertKey)
		return nil
	}

	// Build mandatory fields for message
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	// Create enhanced message with business context
	message := fmt.Sprintf("[%s] Circuit Breaker OPEN - %s",
		mandatoryFields["Environment"],
		o.getAPIIdentifier())

	// Build description
	description := o.buildEnhancedDescription()

	// Create validated alert request
	specificDetails := map[string]string{
		"Latency":       fmt.Sprintf("%d", latency),
		"Memory OK":     fmt.Sprintf("%t", memoryOK),
		"Wait Time":     fmt.Sprintf("%d", waitTime),
		"Alert Type":    alertType,
		"Alert Details": details,
	}

	req, err := o.createValidatedAlertRequest(alertType, message, description, specificDetails)
	if err != nil {
		log.Printf("Failed to create validated alert request: %v", err)
		return err
	}

	// Send the alert
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		log.Printf("Error sending OpsGenie alert: %v", err)
		return err
	}

	// Record the alert time for cooldown
	o.RecordAlert(alertKey)

	log.Printf("ALERT SENT: Circuit breaker OPEN alert sent to OpsGenie. RequestID: %s, Priority: %s, Key: %s",
		resp.RequestId, req.Priority, alertKey)
	log.Printf("Alert sent with fields: %+v", mandatoryFields)

	return nil
}

// SendBreakerResetAlert sends an alert when the circuit breaker resets
func (o *OpsGenieClient) SendBreakerResetAlert() error {
	if o == nil || !o.config.Enabled || !o.config.TriggerOnReset {
		return nil
	}

	if !o.IsInitialized() {
		log.Printf("OpsGenie client not initialized or not enabled for environment, skipping alert")
		return nil
	}

	alertType := "circuit-reset"
	alertKey := o.determineAlertKey(alertType, "reset")

	if o.IsOnCooldown(alertKey) {
		log.Printf("Skipping alert for %s due to cooldown period", alertKey)
		return nil
	}

	// Build mandatory fields for message
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	message := fmt.Sprintf("[%s] Circuit Breaker RESET - %s",
		mandatoryFields["Environment"],
		o.getAPIIdentifier())

	description := o.buildEnhancedDescription()

	specificDetails := map[string]string{
		"Alert Type": alertType,
	}

	req, err := o.createValidatedAlertRequest(alertType, message, description, specificDetails)
	if err != nil {
		log.Printf("Failed to create validated alert request: %v", err)
		return err
	}

	// Send the alert
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		log.Printf("Error sending OpsGenie alert: %v", err)
		return err
	}

	o.RecordAlert(alertKey)

	log.Printf("ALERT SENT: Circuit breaker RESET alert sent to OpsGenie. RequestID: %s, Priority: %s, Key: %s",
		resp.RequestId, req.Priority, alertKey)
	log.Printf("Alert sent with fields: %+v", mandatoryFields)

	return nil
}

// SendMemoryThresholdAlert sends an alert when memory usage exceeds the threshold
func (o *OpsGenieClient) SendMemoryThresholdAlert(memoryStatus *MemoryStatus) error {
	if o == nil || !o.config.Enabled || !o.config.TriggerOnMemory {
		return nil
	}

	if !o.IsInitialized() {
		log.Printf("OpsGenie client not initialized or not enabled for environment, skipping alert")
		return nil
	}

	alertType := "memory-threshold"
	details := fmt.Sprintf("%.1f-percent", memoryStatus.CurrentUsage)
	alertKey := o.determineAlertKey(alertType, details)

	if o.IsOnCooldown(alertKey) {
		log.Printf("Skipping alert for %s due to cooldown period", alertKey)
		return nil
	}

	// Build mandatory fields for message
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	message := fmt.Sprintf("[%s] Memory Threshold Exceeded - %s (%.2f%%)",
		mandatoryFields["Environment"],
		o.getAPIIdentifier(),
		memoryStatus.CurrentUsage)

	description := o.buildEnhancedDescription()

	specificDetails := map[string]string{
		"Current Usage":   fmt.Sprintf("%.2f%%", memoryStatus.CurrentUsage),
		"Threshold":       fmt.Sprintf("%.2f%%", memoryStatus.Threshold),
		"Total Memory MB": fmt.Sprintf("%.2f", float64(memoryStatus.TotalMemory)/(1024*1024)),
		"Used Memory MB":  fmt.Sprintf("%.2f", float64(memoryStatus.UsedMemory)/(1024*1024)),
		"Alert Type":      alertType,
		"Alert Details":   details,
	}

	req, err := o.createValidatedAlertRequest(alertType, message, description, specificDetails)
	if err != nil {
		log.Printf("Failed to create validated alert request: %v", err)
		return err
	}

	// Send the alert
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		log.Printf("Error sending OpsGenie alert: %v", err)
		return err
	}

	o.RecordAlert(alertKey)

	log.Printf("ALERT SENT: Memory threshold alert sent to OpsGenie. RequestID: %s, Priority: %s, Usage: %.2f%%, Key: %s",
		resp.RequestId, req.Priority, memoryStatus.CurrentUsage, alertKey)
	log.Printf("Alert sent with fields: %+v", mandatoryFields)

	return nil
}

// SendLatencyThresholdAlert sends an alert when latency exceeds the threshold
func (o *OpsGenieClient) SendLatencyThresholdAlert(latency int64, thresholdMs int64) error {
	if o == nil || !o.config.Enabled || !o.config.TriggerOnLatency {
		return nil
	}

	if !o.IsInitialized() {
		log.Printf("OpsGenie client not initialized or not enabled for environment, skipping alert")
		return nil
	}

	alertType := "latency-threshold"
	details := fmt.Sprintf("%dms", latency)
	alertKey := o.determineAlertKey(alertType, details)

	if o.IsOnCooldown(alertKey) {
		log.Printf("Skipping alert for %s due to cooldown period", alertKey)
		return nil
	}

	// Build mandatory fields for message
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()

	message := fmt.Sprintf("[%s] High Latency Detected - %s (%dms)",
		mandatoryFields["Environment"],
		o.getAPIIdentifier(),
		latency)

	description := o.buildEnhancedDescription()

	specificDetails := map[string]string{
		"Latency":       fmt.Sprintf("%dms", latency),
		"Threshold":     fmt.Sprintf("%dms", thresholdMs),
		"Alert Type":    alertType,
		"Alert Details": details,
	}

	req, err := o.createValidatedAlertRequest(alertType, message, description, specificDetails)
	if err != nil {
		log.Printf("Failed to create validated alert request: %v", err)
		return err
	}

	// Send the alert
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		log.Printf("Error sending OpsGenie alert: %v", err)
		return err
	}

	o.RecordAlert(alertKey)

	log.Printf("ALERT SENT: Latency threshold alert sent to OpsGenie. RequestID: %s, Priority: %s, Latency: %dms, Key: %s",
		resp.RequestId, req.Priority, latency, alertKey)
	log.Printf("Alert sent with fields: %+v", mandatoryFields)

	return nil
}

// ValidateConfigurationAtStartup validates the configuration when the service starts
func (o *OpsGenieClient) ValidateConfigurationAtStartup() error {
	if o == nil || o.config == nil {
		return fmt.Errorf("OpsGenie client or configuration is nil")
	}

	log.Printf("Validating OpsGenie configuration...")

	// Perform mandatory fields validation
	if err := o.ValidateMandatoryFields(); err != nil {
		log.Printf("❌ Mandatory fields validation failed:")
		for _, field := range err.MissingFields {
			log.Printf("   - Missing field: %s", field)
		}
		for field, reason := range err.InvalidFields {
			log.Printf("   - Invalid field %s: %s", field, reason)
		}

		log.Printf("⚠️  Alerts will be sent with fallback values. Please review configuration.")
	} else {
		log.Printf("✅ All mandatory fields are properly configured")
	}

	// Show current field values
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()
	log.Printf("Current mandatory field values:")
	for field, value := range mandatoryFields {
		log.Printf("   - %s: %s", field, value)
	}

	// Validate OpsGenie connectivity if enabled
	if o.config.Enabled {
		if err := o.TestConnection(); err != nil {
			log.Printf("❌ OpsGenie connectivity test failed: %v", err)
			return fmt.Errorf("OpsGenie connectivity test failed: %v", err)
		}
		log.Printf("✅ OpsGenie connectivity test passed")
	}

	return nil
}

// GenerateConfigurationReport generates a configuration validation report
func (o *OpsGenieClient) GenerateConfigurationReport() string {
	if o == nil || o.config == nil {
		return "❌ OpsGenie client or configuration is nil"
	}

	report := "OpsGenie Configuration Report:\n"
	report += "================================\n\n"

	// Basic configuration
	report += fmt.Sprintf("Enabled: %t\n", o.config.Enabled)
	report += fmt.Sprintf("Region: %s\n", o.config.Region)
	report += fmt.Sprintf("Priority: %s\n", o.config.Priority)
	report += fmt.Sprintf("Source: %s\n", o.config.Source)
	report += "\n"

	// Mandatory fields
	mandatoryFields := o.buildMandatoryFieldsWithFallbacks()
	report += "Mandatory Fields:\n"
	report += "-----------------\n"
	for field, value := range mandatoryFields {
		status := "✅"
		if value == "unknown" || value == "unknown-team" || value == "" {
			status = "⚠️ "
		}
		report += fmt.Sprintf("%s %s: %s\n", status, field, value)
	}
	report += "\n"

	// Validation status
	if err := o.ValidateMandatoryFields(); err != nil {
		report += "Validation Issues:\n"
		report += "------------------\n"
		for _, field := range err.MissingFields {
			report += fmt.Sprintf("❌ Missing: %s\n", field)
		}
		for field, reason := range err.InvalidFields {
			report += fmt.Sprintf("❌ Invalid %s: %s\n", field, reason)
		}
	} else {
		report += "✅ All mandatory fields validated successfully\n"
	}

	return report
}
