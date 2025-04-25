package breaker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
)

// Environment variable names
const (
	EnvOpsGenieAPIKey = "OPSGENIE_API_KEY"
	EnvOpsGenieRegion = "OPSGENIE_REGION"
	EnvOpsGenieAPIURL = "OPSGENIE_API_URL"
)

// MemoryStatus represents the current memory status of the application
type MemoryStatus struct {
	CurrentUsage float64
	Threshold    float64
	TotalMemory  uint64
	UsedMemory   uint64
	OK           bool
}

// OpsGenieClient wraps the OpsGenie SDK client and provides methods to interact with OpsGenie
type OpsGenieClient struct {
	config        *OpsGenieConfig
	alertClient   *alert.Client
	lastAlertTime map[string]time.Time // Map to track when each alert type was last sent
	initialized   bool
}

// NewOpsGenieClient creates a new OpsGenie client with the given configuration
func NewOpsGenieClient(config *OpsGenieConfig) *OpsGenieClient {
	if config == nil {
		config = &OpsGenieConfig{Enabled: false}
	}

	return &OpsGenieClient{
		config:        config,
		lastAlertTime: make(map[string]time.Time),
		initialized:   false,
	}
}

// Initialize sets up the OpsGenie client and validates the API key
func (o *OpsGenieClient) Initialize() error {
	if o == nil {
		return fmt.Errorf("OpsGenieClient is nil")
	}

	if !o.config.Enabled {
		log.Println("OpsGenie integration is disabled")
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
		return true
	}

	lastSent, exists := o.lastAlertTime[alertType]
	if !exists {
		return false // Never sent before
	}

	cooldownDuration := time.Duration(o.config.AlertCooldownSeconds) * time.Second
	return time.Since(lastSent) < cooldownDuration
}

// recordAlert records when an alert was sent to enforce cooldown periods
func (o *OpsGenieClient) recordAlert(alertType string) {
	if o == nil {
		return
	}
	o.lastAlertTime[alertType] = time.Now()
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

// SendBreakerOpenAlert sends an alert when the circuit breaker opens
func (o *OpsGenieClient) SendBreakerOpenAlert(latency int64, memoryOK bool, waitTime int) error {
	if o == nil || !o.initialized {
		return errors.New("OpsGenie client not initialized")
	}

	if !o.config.TriggerOnOpen {
		return nil
	}

	// Check if we've sent this alert too recently
	if o.isOnCooldown("breaker_open") {
		return nil
	}

	// Get API identifier for the message
	apiID := o.getAPIIdentifier()

	// Create request
	req := &alert.CreateAlertRequest{
		Message:     fmt.Sprintf("[ALERT] Circuit Breaker Opened for %s", apiID),
		Description: fmt.Sprintf("The circuit breaker for %s has been triggered due to high latency or memory usage. All requests will be blocked for %d seconds or until manually reset.", apiID, waitTime),
		Priority:    alert.Priority(o.config.Priority),
		Source:      o.config.Source,
		Tags:        append(o.config.Tags, "circuit-breaker-open"),
	}

	// Add API details to description if available
	apiDetails := o.buildAPIInfoDetails()
	if apiDetails != "" {
		req.Description += "\n\n=== API INFORMATION ===\n" + apiDetails
	}

	// Add latency and memory metrics if enabled
	if o.config.IncludeLatencyMetrics {
		req.Description += fmt.Sprintf("\n\nLatency: %d ms (above threshold)", latency)
	}

	if o.config.IncludeMemoryMetrics {
		memStatus := "OK"
		if !memoryOK {
			memStatus = "EXCEEDED"
		}
		req.Description += fmt.Sprintf("\nMemory Status: %s", memStatus)
	}

	// Add team assignment if configured
	if o.config.Team != "" {
		req.Responders = []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send the alert
	result, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record when we sent this alert
	o.recordAlert("breaker_open")

	log.Printf("OpsGenie alert sent successfully. RequestID: %s", result.RequestId)
	return nil
}

// SendBreakerResetAlert sends an alert when the circuit breaker resets
func (o *OpsGenieClient) SendBreakerResetAlert() error {
	if o == nil || !o.initialized {
		return errors.New("OpsGenie client not initialized")
	}

	if !o.config.TriggerOnReset {
		return nil
	}

	// Check if we've sent this alert too recently
	if o.isOnCooldown("breaker_reset") {
		return nil
	}

	// Get API identifier for the message
	apiID := o.getAPIIdentifier()

	// Create request
	req := &alert.CreateAlertRequest{
		Message:     fmt.Sprintf("[RESOLVED] Circuit Breaker Reset for %s", apiID),
		Description: fmt.Sprintf("The circuit breaker for %s has been reset. Traffic is now flowing normally.", apiID),
		Priority:    alert.Priority(o.config.Priority),
		Source:      o.config.Source,
		Tags:        append(o.config.Tags, "circuit-breaker-reset"),
	}

	// Add API details to description if available
	apiDetails := o.buildAPIInfoDetails()
	if apiDetails != "" {
		req.Description += "\n\n=== API INFORMATION ===\n" + apiDetails
	}

	// Add system information if enabled
	if o.config.IncludeSystemInfo {
		req.Description += "\n\nThe system appears to have recovered and latency is back to normal levels."
	}

	// Add team assignment if configured
	if o.config.Team != "" {
		req.Responders = []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send the alert
	result, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record when we sent this alert
	o.recordAlert("breaker_reset")

	log.Printf("OpsGenie alert sent successfully. RequestID: %s", result.RequestId)
	return nil
}

// SendMemoryThresholdAlert sends an alert when memory usage exceeds the threshold
func (o *OpsGenieClient) SendMemoryThresholdAlert(memoryStatus *MemoryStatus) error {
	if o == nil || !o.initialized || memoryStatus == nil {
		return errors.New("OpsGenie client not initialized or invalid memory status")
	}

	if !o.config.TriggerOnMemory {
		return nil
	}

	// Check if we've sent this alert too recently
	if o.isOnCooldown("memory_threshold") {
		return nil
	}

	// Get API identifier for the message
	apiID := o.getAPIIdentifier()

	// Create request
	req := &alert.CreateAlertRequest{
		Message:     fmt.Sprintf("[WARNING] High Memory Usage for %s", apiID),
		Description: fmt.Sprintf("Memory usage for %s has exceeded the threshold of %.1f%%", apiID, memoryStatus.Threshold),
		Priority:    alert.Priority(o.config.Priority),
		Source:      o.config.Source,
		Tags:        append(o.config.Tags, "memory-threshold"),
	}

	// Add API details to description if available
	apiDetails := o.buildAPIInfoDetails()
	if apiDetails != "" {
		req.Description += "\n\n=== API INFORMATION ===\n" + apiDetails
	}

	// Add memory metrics if enabled
	if o.config.IncludeMemoryMetrics && memoryStatus != nil {
		req.Description += fmt.Sprintf("\n\nCurrent Memory Usage: %.1f%%", memoryStatus.CurrentUsage)
		req.Description += fmt.Sprintf("\nMemory Threshold: %.1f%%", memoryStatus.Threshold)
		req.Description += fmt.Sprintf("\nTotal Memory: %d MB", memoryStatus.TotalMemory/(1024*1024))
		req.Description += fmt.Sprintf("\nUsed Memory: %d MB", memoryStatus.UsedMemory/(1024*1024))
	}

	// Add team assignment if configured
	if o.config.Team != "" {
		req.Responders = []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send the alert
	result, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record when we sent this alert
	o.recordAlert("memory_threshold")

	log.Printf("OpsGenie alert sent successfully. RequestID: %s", result.RequestId)
	return nil
}

// SendLatencyThresholdAlert sends an alert when latency exceeds the threshold
func (o *OpsGenieClient) SendLatencyThresholdAlert(latency int64) error {
	if o == nil || !o.initialized {
		return errors.New("OpsGenie client not initialized")
	}

	if !o.config.TriggerOnLatency {
		return nil
	}

	// Check if we've sent this alert too recently
	if o.isOnCooldown("latency_threshold") {
		return nil
	}

	// Get API identifier for the message
	apiID := o.getAPIIdentifier()

	// Create request
	req := &alert.CreateAlertRequest{
		Message:     fmt.Sprintf("[WARNING] High Latency for %s", apiID),
		Description: fmt.Sprintf("Request latency for %s has exceeded the configured threshold", apiID),
		Priority:    alert.Priority(o.config.Priority),
		Source:      o.config.Source,
		Tags:        append(o.config.Tags, "latency-threshold"),
	}

	// Add API details to description if available
	apiDetails := o.buildAPIInfoDetails()
	if apiDetails != "" {
		req.Description += "\n\n=== API INFORMATION ===\n" + apiDetails
	}

	// Add latency metrics if enabled
	if o.config.IncludeLatencyMetrics {
		req.Description += fmt.Sprintf("\n\nCurrent Latency: %d ms", latency)
	}

	// Add team assignment if configured
	if o.config.Team != "" {
		req.Responders = []alert.Responder{
			{
				Type: "team",
				Name: o.config.Team,
			},
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send the alert
	result, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send OpsGenie alert: %v", err)
	}

	// Record when we sent this alert
	o.recordAlert("latency_threshold")

	log.Printf("OpsGenie alert sent successfully. RequestID: %s", result.RequestId)
	return nil
}
