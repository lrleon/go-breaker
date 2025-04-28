package tests

import (
	"testing"
	"time"

	"github.com/lrleon/go-breaker/breaker"
)

// TestCooldownPreventsRepeatedAlerts verifies that:
// 1. If an alert has been sent and the cooldown time has not elapsed, it is not sent again
// 2. If the cooldown has passed, then the alert is sent again
func TestCooldownPreventsRepeatedAlerts(t *testing.T) {
	// Create a configuration for tests with a cooldown time of 2 seconds
	config := &breaker.OpsGenieConfig{
		Enabled:              true,
		AlertCooldownSeconds: 2, // Short time so the test doesn't take long
		APIKey:               "test-key",
		Team:                 "test-team",
		TriggerOnLatency:     true,
		TriggerOnMemory:      true,
		TriggerOnOpen:        true,
		TriggerOnReset:       true,
		APIName:              "test-api",
		APIVersion:           "1.0",
		Source:               "test",
		UseEnvironments:      false,
	}

	// Create an independent OpsGenie client for this test (don't use the singleton)
	client := breaker.NewOpsGenieClient(config)

	// We don't initialize the real client to avoid API calls

	// Shared context for the tests
	alertKey := "test-api-test-alert-specific-details"

	// Test cases
	tests := []struct {
		name             string
		setupFunc        func()
		expectOnCooldown bool
		wantError        bool
	}{
		{
			name: "First time the alert is sent - should be sent",
			setupFunc: func() {
				// No previous configuration - it's the first time
			},
			expectOnCooldown: false,
			wantError:        false,
		},
		{
			name: "Alert sent recently - should be in cooldown",
			setupFunc: func() {
				// Record that it was sent 1 second ago (less than the cooldown)
				client.RecordAlert(alertKey)
				// Wait a brief moment to ensure the recording happened
				time.Sleep(100 * time.Millisecond)
			},
			expectOnCooldown: true,
			wantError:        false,
		},
		{
			name: "Alert sent a while ago - cooldown has passed, should be sent",
			setupFunc: func() {
				// Record that it was sent 3 seconds ago (more than the cooldown)
				client.RecordAlert(alertKey)
				// Wait longer than the cooldown time
				time.Sleep(time.Duration(config.AlertCooldownSeconds+1) * time.Second)
			},
			expectOnCooldown: false,
			wantError:        false,
		},
	}

	// Run the test cases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set up the initial state according to the case
			tc.setupFunc()

			// Verify the cooldown state using the exported method
			onCooldown := client.IsOnCooldown(alertKey)

			// Check that the result is as expected
			if onCooldown != tc.expectOnCooldown {
				t.Errorf("IsOnCooldown() = %v, want %v", onCooldown, tc.expectOnCooldown)
			}
		})
	}
}

// TestAlertDeduplicationWithRealClient performs a more realistic test with the current implementation
func TestAlertDeduplicationWithRealClient(t *testing.T) {
	// Configuration with very short cooldown for quick tests
	config := &breaker.OpsGenieConfig{
		Enabled:              true,
		AlertCooldownSeconds: 1, // 1 second cooldown
		APIKey:               "test-key",
		Team:                 "test-team",
		APIName:              "test-api",
		APIVersion:           "1.0",
		Source:               "test",
	}

	// Create an uninitialized client to avoid real calls
	client := breaker.NewOpsGenieClient(config)

	// Define an alert type for the test
	alertType := "circuit-open"
	alertDetails := "test-service"

	// Verify that the first alert is not in cooldown
	if client.IsOnCooldown(alertType + "-" + alertDetails) {
		t.Fatal("Unexpected cooldown state before any alerts sent")
	}

	// Record an alert
	client.RecordAlert(alertType + "-" + alertDetails)

	// Verify that it is now in cooldown
	if !client.IsOnCooldown(alertType + "-" + alertDetails) {
		t.Fatal("Alert should be in cooldown immediately after sending")
	}

	// Wait for the cooldown to pass
	time.Sleep(time.Duration(config.AlertCooldownSeconds+1) * time.Second)

	// Verify that it is no longer in cooldown
	if client.IsOnCooldown(alertType + "-" + alertDetails) {
		t.Fatal("Cooldown should have expired after waiting")
	}
}
