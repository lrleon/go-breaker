package breaker

import (
	"context"

	"fmt"
	"log"
	"os"
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
	EnvEnvironment    = "ENVIRONMENT" // Environment variable to determine the current environment
)

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
// Ensures that only one shared instance of the client exists across the application
func GetOpsGenieClient(config *OpsGenieConfig) *OpsGenieClient {
	opsgenieClientMutex.Lock()
	defer opsgenieClientMutex.Unlock()

	// Check if instance exists or if configuration has changed
	needsNew := opsgenieClientInstance == nil

	// If the instance exists, check if the configuration has changed
	if !needsNew && opsgenieClientInstance.config != nil {
		// Only compare the parameters relevant to cooldown
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
	lastAlertTime map[string]time.Time // Map to track when each alert type was last sent
	initialized   bool
	mutex         sync.RWMutex // Mutex to protect concurrent access to the lastAlertTime map
	environment   Environment  // Current runtime environment
}

// NewOpsGenieClient creates a new OpsGenie client with the given configuration
func NewOpsGenieClient(config *OpsGenieConfig) *OpsGenieClient {
	if config == nil {
		config = &OpsGenieConfig{Enabled: false}
	}

	// Determine the current environment only if we're using environments
	var env Environment
	if config.UseEnvironments {
		env = determineEnvironment()
	}

	return &OpsGenieClient{
		config:        config,
		lastAlertTime: make(map[string]time.Time),
		initialized:   false,
		environment:   env,
	}
}

// determineEnvironment detects the current runtime environment
func determineEnvironment() Environment {
	// Check environment variable first
	envValue := os.Getenv(EnvEnvironment)

	switch envValue {
	case string(EnvDevelopment):
		return EnvDevelopment
	case string(EnvUAT):
		return EnvUAT
	case string(EnvProduction):
		return EnvProduction
	default:
		// Default to development if not specified
		if envValue == "" {
			log.Println("Environment not specified, defaulting to development")
		} else {
			log.Printf("Unknown environment '%s', defaulting to development", envValue)
		}
		return EnvDevelopment
	}
}

// Initialize sets up the OpsGenie client and validates the API key
func (o *OpsGenieClient) Initialize() error {
	if o == nil {
		return fmt.Errorf("OpsGenieClient is nil")
	}

	// Check if alerts are enabled for the current environment
	if !o.isEnabledForEnvironment() {
		log.Printf("OpsGenie integration is disabled for environment: %s", o.environment)
		return nil
	}

	// First check for API key in environment variables
	apiKey := os.Getenv(EnvOpsGenieAPIKey)
	if apiKey == "" {
		// Fall back to config file if not in environment
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

	// Allow custom API URL override for on-prem or other deployments
	customURL := os.Getenv(EnvOpsGenieAPIURL)
	if customURL != "" {
		apiUrl = customURL
	} else if o.config.APIURL != "" {
		apiUrl = o.config.APIURL
	}

	// The OpsGenie Go SDK doesn't have a direct field for setting the API URL
	// in the config directly, so we log it for debugging
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

// isEnabledForEnvironment checks if OpsGenie is enabled for the current environment
func (o *OpsGenieClient) isEnabledForEnvironment() bool {
	if o == nil || o.config == nil {
		return false
	}

	// If we're not using environments, just use the global enabled flag
	if !o.config.UseEnvironments {
		return o.config.Enabled
	}

	// Check if we have environment-specific settings
	if o.config.EnvironmentSettings != nil {
		if envConfig, exists := o.config.EnvironmentSettings[string(o.environment)]; exists {
			return envConfig.Enabled
		}
	}

	// Fall back to global enabled setting
	return o.config.Enabled
}

// getPriorityForEnvironment returns the appropriate priority for the current environment
func (o *OpsGenieClient) getPriorityForEnvironment() alert.Priority {
	if o == nil || o.config == nil {
		return alert.P3 // Default to P3 if no config
	}

	// If we're not using environments, just use the global priority
	var priorityStr string
	if !o.config.UseEnvironments {
		priorityStr = o.config.Priority
	} else {
		// Check if we have environment-specific settings
		if o.config.EnvironmentSettings != nil {
			if envConfig, exists := o.config.EnvironmentSettings[string(o.environment)]; exists && envConfig.Priority != "" {
				priorityStr = envConfig.Priority
			}
		}

		// Fall back to global priority
		if priorityStr == "" && o.config.Priority != "" {
			priorityStr = o.config.Priority
		}

		// If still empty, default based on environment
		if priorityStr == "" {
			switch o.environment {
			case EnvProduction:
				priorityStr = "P1" // Critical for production
			case EnvUAT:
				priorityStr = "P3" // Medium for UAT
			default:
				priorityStr = "P5" // Low for development
			}
		}
	}

	// If priorityStr is still empty at this point, use a default
	if priorityStr == "" {
		priorityStr = "P3"
	}

	// Convert string to alert.Priority
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

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to list one alert to test connection
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

// isOnCooldown checks if an alert type is still in its cooldown period
func (o *OpsGenieClient) isOnCooldown(alertType string) bool {
	if o == nil || o.config == nil {
		log.Printf("OpsGenie client or config is nil when checking cooldown for %s", alertType)
		return true
	}

	// Acquire lock for safe reading of the map
	o.mutex.RLock()
	defer o.mutex.RUnlock()

	lastSent, exists := o.lastAlertTime[alertType]
	if !exists {
		log.Printf("Alert type %s has never been sent before", alertType)
		return false // Never sent before
	}

	cooldownDuration := time.Duration(o.config.AlertCooldownSeconds) * time.Second
	timeSinceLastAlert := time.Since(lastSent)
	isOnCooldown := timeSinceLastAlert < cooldownDuration

	// Add more detailed logging about cooldown status
	log.Printf("COOLDOWN CHECK: Alert %s - Last sent: %v, Cooldown duration: %v, Time since last: %v, On cooldown: %v",
		alertType, lastSent.Format(time.RFC3339), cooldownDuration, timeSinceLastAlert, isOnCooldown)

	return isOnCooldown
}

// recordAlert records when an alert was sent to enforce cooldown periods
func (o *OpsGenieClient) recordAlert(alertType string) {
	if o == nil {
		log.Printf("Cannot record alert time for %s: OpsGenie client is nil", alertType)
		return
	}

	// Acquire lock for safe writing to the map
	o.mutex.Lock()
	defer o.mutex.Unlock()

	now := time.Now()
	o.lastAlertTime[alertType] = now
	log.Printf("COOLDOWN START: Recorded alert %s at %v with %d second cooldown",
		alertType, now.Format(time.RFC3339), o.config.AlertCooldownSeconds)
}

// Helper function to guarantee alert uniqueness by API identifier and alert type
func (o *OpsGenieClient) createUniqueAlertIdentifier(alertType string) string {
	apiID := o.getAPIIdentifier()
	return fmt.Sprintf("%s-%s", apiID, alertType)
}

// buildAPIInfoDetails creates a structured description of the API for alert details
func (o *OpsGenieClient) buildAPIInfoDetails() string {
	if o == nil || o.config == nil {
		return ""
	}

	// Build a detailed description including all available API information
	details := ""

	// Add API basic information
	if o.config.APIName != "" {
		details += fmt.Sprintf("API: %s\n", o.config.APIName)
	}

	if o.config.APIVersion != "" {
		details += fmt.Sprintf("Version: %s\n", o.config.APIVersion)
	}

	if o.config.APINamespace != "" {
		details += fmt.Sprintf("Environment: %s\n", o.config.APINamespace)
	}

	if o.config.APIDescription != "" {
		details += fmt.Sprintf("\nDescription: %s\n", o.config.APIDescription)
	}

	if o.config.APIOwner != "" {
		details += fmt.Sprintf("Owner: %s\n", o.config.APIOwner)
	}

	if o.config.APIPriority != "" {
		details += fmt.Sprintf("Business Priority: %s\n", o.config.APIPriority)
	}

	// Add dependencies if available
	if len(o.config.APIDependencies) > 0 {
		details += "\nDependencies:\n"
		for _, dep := range o.config.APIDependencies {
			details += fmt.Sprintf("- %s\n", dep)
		}
	}

	// Add protected endpoints if available
	if len(o.config.APIEndpoints) > 0 {
		details += "\nProtected Endpoints:\n"
		for _, endpoint := range o.config.APIEndpoints {
			details += fmt.Sprintf("- %s\n", endpoint)
		}
	}

	// Add custom attributes if available
	if len(o.config.APICustomAttributes) > 0 {
		details += "\nAdditional Information:\n"
		for k, v := range o.config.APICustomAttributes {
			details += fmt.Sprintf("- %s: %s\n", k, v)
		}
	}

	return details
}

// getAPIIdentifier gets a string that uniquely identifies the API for alerts
func (o *OpsGenieClient) getAPIIdentifier() string {
	if o == nil || o.config == nil {
		return "unknown-api"
	}

	if o.config.APIName != "" {
		// If we have namespace and name, combine them
		if o.config.APINamespace != "" {
			return fmt.Sprintf("%s/%s", o.config.APINamespace, o.config.APIName)
		}
		return o.config.APIName
	}

	// Fallback to source if no API name is set
	return o.config.Source
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
	alertType := o.createUniqueAlertIdentifier("circuit_breaker_open")

	// Check if OpsGenie is properly initialized
	if !o.IsInitialized() {
		return fmt.Errorf("OpsGenie client not initialized")
	}

	// Check if this type of alert is enabled in the config
	if !o.config.TriggerOnOpen {
		log.Printf("Breaker open alerts are disabled in config")
		return nil
	}

	// Check if we're in a cooldown period for this alert type
	if o.isOnCooldown(alertType) {
		log.Printf("Skipping alert for %s due to cooldown period", alertType)
		return nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Begin building the alert message
	apiIdentifier := o.getAPIIdentifier()

	message := fmt.Sprintf("Circuit Breaker has OPENED for %s", apiIdentifier)
	description := fmt.Sprintf(
		"The circuit breaker has been triggered and is now OPEN for %s.\n\n"+
			"- Latency: %d ms\n"+
			"- Memory Status: %s\n"+
			"- Waiting Time: %d seconds before attempting reset\n\n"+
			"%s",
		apiIdentifier,
		latency,
		memoryStatusString(memoryOK),
		waitTime,
		o.buildAPIInfoDetails(),
	)

	// Create the request
	req := &alert.CreateAlertRequest{
		Message:     message,
		Description: description,
		Responders: []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		},
		Tags:     o.config.Tags,
		Entity:   apiIdentifier,
		Source:   o.config.Source,
		Priority: o.getPriorityForEnvironment(), // Use environment-specific priority
	}

	// Send the alert
	log.Printf("Sending circuit breaker OPEN alert to OpsGenie with priority: %s", req.Priority)
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record the alert time for cooldown
	o.recordAlert(alertType)

	log.Printf("ALERT SENT: Circuit breaker OPEN alert sent to OpsGenie. RequestID: %s, Priority: %s", resp.RequestId, req.Priority)
	return nil
}

// SendBreakerResetAlert sends an alert when the circuit breaker resets
func (o *OpsGenieClient) SendBreakerResetAlert() error {
	alertType := o.createUniqueAlertIdentifier("circuit_breaker_reset")

	// Check if OpsGenie is properly initialized
	if !o.IsInitialized() {
		return fmt.Errorf("OpsGenie client not initialized")
	}

	// Check if this type of alert is enabled in the config
	if !o.config.TriggerOnReset {
		log.Printf("Breaker reset alerts are disabled in config")
		return nil
	}

	// Check if we're in a cooldown period for this alert type
	if o.isOnCooldown(alertType) {
		log.Printf("Skipping alert for %s due to cooldown period", alertType)
		return nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Begin building the alert message
	apiIdentifier := o.getAPIIdentifier()

	message := fmt.Sprintf("Circuit Breaker has RESET for %s", apiIdentifier)
	description := fmt.Sprintf(
		"The circuit breaker has been reset and is now CLOSED for %s.\n\n"+
			"Requests are now being processed normally.\n\n"+
			"%s",
		apiIdentifier,
		o.buildAPIInfoDetails(),
	)

	// Create the request
	req := &alert.CreateAlertRequest{
		Message:     message,
		Description: description,
		Responders: []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		},
		Tags:     o.config.Tags,
		Entity:   apiIdentifier,
		Source:   o.config.Source,
		Priority: o.getPriorityForEnvironment(), // Use environment-specific priority
	}

	// Send the alert
	log.Printf("Sending circuit breaker RESET alert to OpsGenie with priority: %s", req.Priority)
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record the alert time for cooldown
	o.recordAlert(alertType)

	log.Printf("ALERT SENT: Circuit breaker RESET alert sent to OpsGenie. RequestID: %s, Priority: %s", resp.RequestId, req.Priority)
	return nil
}

// SendMemoryThresholdAlert sends an alert when memory usage exceeds the threshold
func (o *OpsGenieClient) SendMemoryThresholdAlert(memoryStatus *MemoryStatus) error {
	alertType := o.createUniqueAlertIdentifier("memory_threshold")

	// Check if OpsGenie is properly initialized
	if !o.IsInitialized() {
		return fmt.Errorf("OpsGenie client not initialized")
	}

	// Check if this type of alert is enabled in the config
	if !o.config.TriggerOnMemory {
		log.Printf("Memory threshold alerts are disabled in config")
		return nil
	}

	// Check if we're in a cooldown period for this alert type
	if o.isOnCooldown(alertType) {
		log.Printf("Skipping alert for %s due to cooldown period", alertType)
		return nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Begin building the alert message
	apiIdentifier := o.getAPIIdentifier()

	message := fmt.Sprintf("Memory Threshold Exceeded for %s", apiIdentifier)
	description := fmt.Sprintf(
		"The memory usage has exceeded the threshold for %s.\n\n"+
			"- Current Memory Usage: %.2f%%\n"+
			"- Threshold: %.2f%%\n"+
			"- Used Memory: %d bytes\n"+
			"- Total Memory: %d bytes\n\n"+
			"%s",
		apiIdentifier,
		memoryStatus.CurrentUsage,
		memoryStatus.Threshold,
		memoryStatus.UsedMemory,
		memoryStatus.TotalMemory,
		o.buildAPIInfoDetails(),
	)

	// Create the request
	req := &alert.CreateAlertRequest{
		Message:     message,
		Description: description,
		Responders: []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		},
		Tags:     o.config.Tags,
		Entity:   apiIdentifier,
		Source:   o.config.Source,
		Priority: o.getPriorityForEnvironment(), // Use environment-specific priority
	}

	// Send the alert
	log.Printf("Sending memory threshold alert to OpsGenie with priority: %s", req.Priority)
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record the alert time for cooldown
	o.recordAlert(alertType)

	log.Printf("ALERT SENT: Memory threshold alert sent to OpsGenie. RequestID: %s, Priority: %s, Usage: %.2f%%",
		resp.RequestId, req.Priority, memoryStatus.CurrentUsage)
	return nil
}

// SendLatencyThresholdAlert sends an alert when latency exceeds the threshold
func (o *OpsGenieClient) SendLatencyThresholdAlert(latency int64) error {
	alertType := o.createUniqueAlertIdentifier("latency_threshold")

	// Check if OpsGenie is properly initialized
	if !o.IsInitialized() {
		return fmt.Errorf("OpsGenie client not initialized")
	}

	// Check if this type of alert is enabled in the config
	if !o.config.TriggerOnLatency {
		log.Printf("Latency threshold alerts are disabled in config")
		return nil
	}

	// Check if we're in a cooldown period for this alert type
	if o.isOnCooldown(alertType) {
		log.Printf("Skipping alert for %s due to cooldown period", alertType)
		return nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Begin building the alert message
	apiIdentifier := o.getAPIIdentifier()

	message := fmt.Sprintf("Latency Threshold Exceeded for %s", apiIdentifier)
	description := fmt.Sprintf(
		"The latency has exceeded the threshold for %s.\n\n"+
			"- Current Latency: %d ms\n\n"+
			"%s",
		apiIdentifier,
		latency,
		o.buildAPIInfoDetails(),
	)

	// Create the request
	req := &alert.CreateAlertRequest{
		Message:     message,
		Description: description,
		Responders: []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		},
		Tags:     o.config.Tags,
		Entity:   apiIdentifier,
		Source:   o.config.Source,
		Priority: o.getPriorityForEnvironment(), // Use environment-specific priority
	}

	// Send the alert
	log.Printf("Sending latency threshold alert to OpsGenie with priority: %s", req.Priority)
	resp, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record the alert time for cooldown
	o.recordAlert(alertType)

	log.Printf("ALERT SENT: Latency threshold alert sent to OpsGenie. RequestID: %s, Priority: %s, Latency: %dms",
		resp.RequestId, req.Priority, latency)
	return nil
}
