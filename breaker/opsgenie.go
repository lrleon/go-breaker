package breaker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
)

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

	if o.config.APIKey == "" {
		return fmt.Errorf("OpsGenie API key is required when integration is enabled")
	}

	// Configure the client based on the configuration
	cfg := client.Config{
		ApiKey: o.config.APIKey,
	}

	// Create the alert client
	alertClient, err := alert.NewClient(&cfg)
	if err != nil {
		return fmt.Errorf("failed to create OpsGenie alert client: %v", err)
	}

	o.alertClient = alertClient
	o.initialized = true

	// Test the connection to verify the API key is valid
	return o.TestConnection()
}

// TestConnection checks if the API key is valid by making a simple request to OpsGenie
func (o *OpsGenieClient) TestConnection() error {
	if o == nil {
		return fmt.Errorf("OpsGenieClient is nil")
	}

	if !o.config.Enabled {
		return nil // Skip if disabled
	}

	if !o.initialized || o.alertClient == nil {
		return fmt.Errorf("OpsGenie client is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Let's try a simple "get" operation to test connectivity
	// We'll try to get alerts with a very specific filter that's unlikely to return anything
	// but will validate if the API is accessible and the key is valid
	listRequest := &alert.ListAlertRequest{
		Limit: 1, // Just request a single alert to minimize data transfer
	}

	_, err := o.alertClient.List(ctx, listRequest)
	if err != nil {
		return fmt.Errorf("failed to connect to OpsGenie (API key may be invalid): %v", err)
	}

	log.Println("Successfully connected to OpsGenie API")
	return nil
}

// SendBreakerOpenAlert sends an alert when the circuit breaker opens
func (o *OpsGenieClient) SendBreakerOpenAlert(latency int64, memoryOK bool, waitTime int) error {
	if o == nil || !o.initialized {
		return fmt.Errorf("OpsGenieClient is not properly initialized")
	}

	if !o.config.Enabled || !o.config.TriggerOnOpen || o.alertClient == nil {
		return nil // Skip if disabled or not configured to send this alert
	}

	// Check cooldown to prevent alert storms
	if !o.isAlertAllowed("breaker_open") {
		log.Println("Skipping OpsGenie alert for breaker open due to cooldown period")
		return nil
	}

	// Create the alert request
	message := "Circuit Breaker Opened"
	description := fmt.Sprintf("The circuit breaker has been opened.\n\n")

	if o.config.IncludeLatencyMetrics {
		description += fmt.Sprintf("Current Latency: %d ms\n", latency)
	}

	if o.config.IncludeMemoryMetrics {
		description += fmt.Sprintf("Memory Status OK: %v\n", memoryOK)
	}

	description += fmt.Sprintf("Wait Time: %d seconds before retry\n", waitTime)

	// Build the alert request
	req := &alert.CreateAlertRequest{
		Message:     message,
		Description: description,
		Priority:    o.getPriority(),
		Source:      o.getSource(),
	}

	// Add tags if configured
	if o.config.Tags != nil && len(o.config.Tags) > 0 {
		req.Tags = append([]string{}, o.config.Tags...)
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

	// Send the alert
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send breaker open alert to OpsGenie: %v", err)
	}

	// Update the last alert time for this alert type
	o.lastAlertTime["breaker_open"] = time.Now()
	log.Println("Successfully sent breaker open alert to OpsGenie")
	return nil
}

// SendBreakerResetAlert sends an alert when the circuit breaker resets
func (o *OpsGenieClient) SendBreakerResetAlert() error {
	if o == nil || !o.initialized {
		return fmt.Errorf("OpsGenieClient is not properly initialized")
	}

	if !o.config.Enabled || !o.config.TriggerOnReset || o.alertClient == nil {
		return nil // Skip if disabled or not configured to send this alert
	}

	// Check cooldown
	if !o.isAlertAllowed("breaker_reset") {
		log.Println("Skipping OpsGenie alert for breaker reset due to cooldown period")
		return nil
	}

	// Create the alert request
	req := &alert.CreateAlertRequest{
		Message:     "Circuit Breaker Reset",
		Description: "The circuit breaker has been reset and is now allowing traffic.",
		Priority:    o.getPriority(),
		Source:      o.getSource(),
	}

	// Add tags if configured
	if o.config.Tags != nil && len(o.config.Tags) > 0 {
		req.Tags = append([]string{}, o.config.Tags...)
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

	// Send the alert
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := o.alertClient.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send breaker reset alert to OpsGenie: %v", err)
	}

	// Update the last alert time
	o.lastAlertTime["breaker_reset"] = time.Now()
	log.Println("Successfully sent breaker reset alert to OpsGenie")
	return nil
}

// Helper methods

// isAlertAllowed checks if an alert can be sent based on the cooldown period
func (o *OpsGenieClient) isAlertAllowed(alertType string) bool {
	if o == nil {
		return false
	}

	// If no cooldown is configured, always allow alerts
	if o.config.AlertCooldownSeconds <= 0 {
		return true
	}

	lastTime, exists := o.lastAlertTime[alertType]
	if !exists {
		return true // First time sending this alert type
	}

	// Check if cooldown period has elapsed
	cooldownDuration := time.Duration(o.config.AlertCooldownSeconds) * time.Second
	return time.Since(lastTime) >= cooldownDuration
}

// getPriority returns the configured priority or a default
func (o *OpsGenieClient) getPriority() alert.Priority {
	if o == nil || o.config == nil {
		return alert.P3 // Default to medium priority if config is nil
	}

	// Map the configured priority string to the alert.Priority enum
	switch o.config.Priority {
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
		return alert.P3 // Default to medium priority
	}
}

// getSource returns the configured source or a default
func (o *OpsGenieClient) getSource() string {
	if o == nil || o.config == nil || o.config.Source == "" {
		return "go-breaker" // Default source
	}
	return o.config.Source
}
