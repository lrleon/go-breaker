package tests

import (
	"context"
	"os"
	"testing"

	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Create an interface that abstracts the operations we perform on the OpsGenie client
type alertClientInterface interface {
	Create(ctx context.Context, request interface{}) (interface{}, error)
	List(ctx context.Context, request interface{}) (interface{}, error)
}

// MockAlertClient implements the alertClientInterface for testing
type MockAlertClient struct {
	mock.Mock
}

func (m *MockAlertClient) Create(ctx context.Context, request interface{}) (interface{}, error) {
	args := m.Called(ctx, request)
	return args.Get(0), args.Error(1)
}

func (m *MockAlertClient) List(ctx context.Context, request interface{}) (interface{}, error) {
	args := m.Called(ctx, request)
	return args.Get(0), args.Error(1)
}

// TestNewOpsGenieClient tests the NewOpsGenieClient function
func TestNewOpsGenieClient(t *testing.T) {
	// Test with nil config
	client := breaker.NewOpsGenieClient(nil)
	assert.NotNil(t, client)
	assert.False(t, client.IsInitialized())

	// Test with valid config
	config := &breaker.OpsGenieConfig{
		Enabled: true,
		APIKey:  "test-key",
	}
	client = breaker.NewOpsGenieClient(config)
	assert.NotNil(t, client)
	assert.False(t, client.IsInitialized())
}

// TestInitialize tests the Initialize method
func TestInitialize(t *testing.T) {
	// Save original env vars
	originalAPIKey := os.Getenv(breaker.EnvOpsGenieAPIKey)
	originalRegion := os.Getenv(breaker.EnvOpsGenieRegion)
	originalAPIURL := os.Getenv(breaker.EnvOpsGenieAPIURL)
	defer func() {
		// Restore original values
		if originalAPIKey == "" {
			os.Unsetenv(breaker.EnvOpsGenieAPIKey)
		} else {
			os.Setenv(breaker.EnvOpsGenieAPIKey, originalAPIKey)
		}
		if originalRegion == "" {
			os.Unsetenv(breaker.EnvOpsGenieRegion)
		} else {
			os.Setenv(breaker.EnvOpsGenieRegion, originalRegion)
		}
		if originalAPIURL == "" {
			os.Unsetenv(breaker.EnvOpsGenieAPIURL)
		} else {
			os.Setenv(breaker.EnvOpsGenieAPIURL, originalAPIURL)
		}
	}()

	t.Run("NilClient", func(t *testing.T) {
		var client *breaker.OpsGenieClient
		err := client.Initialize()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpsGenieClient is nil")
	})

	t.Run("DisabledClient", func(t *testing.T) {
		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled: false, // Client explicitly disabled
		})

		// A disabled client should not initialize
		err := client.Initialize()

		// Verify there's no error (it's valid to have a disabled client)
		assert.NoError(t, err, "Disabled client should not return error on Initialize()")

		// The client should NOT be initialized when disabled
		assert.False(t, client.IsInitialized(), "Disabled client should not be initialized")

		t.Logf("Disabled client correctly remained uninitialized")
	})

	t.Run("MissingAPIKeyInBothPlaces", func(t *testing.T) {
		// Clear environment variable
		os.Unsetenv(breaker.EnvOpsGenieAPIKey)

		// Create config without API key
		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled:     true,
			Team:        "test-team",
			Environment: "test",
			BookmakerID: "test-bookmaker",
		})

		err := client.Initialize()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "OpsGenie API key not found")
		assert.False(t, client.IsInitialized())
	})

	t.Run("APIKeyInEnvironment", func(t *testing.T) {
		// Skip this test if no real API key is available
		realAPIKey := os.Getenv(breaker.EnvOpsGenieAPIKey)
		if realAPIKey == "" {
			t.Skip("Skipping test - no real OpsGenie API key available in environment")
		}

		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled:     true,
			Team:        "test-team",
			Environment: "test",
			BookmakerID: "test-bookmaker",
		})

		err := client.Initialize()
		if err != nil {
			// If there's an error, it should NOT be about missing API key
			assert.NotContains(t, err.Error(), "API key not found")
			t.Logf("Initialization failed (expected with test data): %v", err)
		} else {
			// If initialization succeeds, client should be initialized
			assert.True(t, client.IsInitialized())
			t.Logf("OpsGenie client initialized successfully")
		}
	})

	t.Run("APIKeyInConfig", func(t *testing.T) {
		// Clear environment variable to force using config API key
		os.Unsetenv(breaker.EnvOpsGenieAPIKey)

		// Check if we have a real API key to use
		realAPIKey := originalAPIKey
		if realAPIKey == "" {
			t.Skip("Skipping test - no real OpsGenie API key available")
		}

		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled:     true,
			APIKey:      realAPIKey, // Use the real API key
			Team:        "test-team",
			Environment: "test",
			BookmakerID: "test-bookmaker",
		})

		err := client.Initialize()
		if err != nil {
			// If there's an error, it should NOT be about missing API key
			assert.NotContains(t, err.Error(), "API key not found")
			t.Logf("Initialization failed (expected with test data): %v", err)
		} else {
			// If initialization succeeds, client should be initialized
			assert.True(t, client.IsInitialized())
			t.Logf("OpsGenie client initialized successfully")
		}
	})

	t.Run("FakeAPIKey", func(t *testing.T) {
		// Test with completely fake API key - should fail on connection
		os.Unsetenv(breaker.EnvOpsGenieAPIKey)

		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled:     true,
			APIKey:      "fake-api-key-for-testing",
			Team:        "test-team",
			Environment: "test",
			BookmakerID: "test-bookmaker",
		})

		err := client.Initialize()
		// Should fail, but not because of missing API key
		assert.Error(t, err)
		assert.NotContains(t, err.Error(), "API key not found")
		assert.False(t, client.IsInitialized())
		t.Logf("Fake API key correctly rejected: %v", err)
	})
}

// Test IsInitialized
func TestIsInitialized(t *testing.T) {
	// Test with nil client
	var client *breaker.OpsGenieClient
	assert.False(t, client.IsInitialized())

	// Test with non-initialized client
	client = breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{Enabled: true})
	assert.False(t, client.IsInitialized())

	// We cannot test the case of an initialized client directly
	// since initialization requires a real connection to OpsGenie
}

// TestSendingAlerts verifies the behavior of alert sending methods
// when the client is not initialized
func TestSendingAlerts(t *testing.T) {
	// Create an uninitialized client
	client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
		Enabled: true,
		APIKey:  "test-key",
	})
	// We don't call Initialize() to ensure it remains uninitialized

	// Test breaker open alert - should return nil when uninitialized
	err := client.SendBreakerOpenAlert(100, true, 60)
	assert.NoError(t, err, "SendBreakerOpenAlert should return nil for uninitialized client")

	// Test breaker reset alert - should return nil when uninitialized
	err = client.SendBreakerResetAlert()
	assert.NoError(t, err, "SendBreakerResetAlert should return nil for uninitialized client")

	// Test memory threshold alert - should return nil when uninitialized
	memStatus := &breaker.MemoryStatus{
		CurrentUsage: 85.0,
		Threshold:    80.0,
		TotalMemory:  1024 * 1024 * 1024,
		UsedMemory:   850 * 1024 * 1024,
		OK:           false,
	}
	err = client.SendMemoryThresholdAlert(memStatus)
	assert.NoError(t, err, "SendMemoryThresholdAlert should return nil for uninitialized client")

	// Test latency threshold alert - should return nil when uninitialized
	err = client.SendLatencyThresholdAlert(100, 50)
	assert.NoError(t, err, "SendLatencyThresholdAlert should return nil for uninitialized client")
}

// TestMandatoryFieldsValidation tests the validation of mandatory fields
func TestMandatoryFieldsValidation(t *testing.T) {
	t.Run("AllFieldsMissing", func(t *testing.T) {
		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled: true,
			APIKey:  "test-key",
		})

		err := client.ValidateMandatoryFields()
		assert.Error(t, err)

		// Check the error message contains expected content instead of type assertion
		errorMsg := err.Error()
		assert.Contains(t, errorMsg, "team")
		assert.Contains(t, errorMsg, "environment")
		assert.Contains(t, errorMsg, "bookmaker_id")
		assert.Contains(t, errorMsg, "Missing")
	})

	t.Run("AllFieldsPresent", func(t *testing.T) {
		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled:     true,
			APIKey:      "test-key",
			Team:        "test-team",
			Environment: "test",
			BookmakerID: "test-bookmaker",
			Business:    "internal",
			// Add hostname explicitly to ensure it's present
			Hostname: "test-host",
		})

		err := client.ValidateMandatoryFields()

		// Detailed debugging
		t.Logf("Validation result: err = %v", err)
		t.Logf("err == nil: %v", err == nil)
		t.Logf("err != nil: %v", err != nil)

		if err != nil {
			t.Logf("Error details: %v", err)
			t.Logf("Error type: %T", err)
			t.Logf("Error string: %q", err.Error())
			t.Errorf("Expected no error but got: %v", err)
		} else {
			t.Logf("SUCCESS: Validation passed as expected")
		}
	})

	t.Run("SomeFieldsMissing", func(t *testing.T) {
		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled: true,
			APIKey:  "test-key",
			Team:    "test-team",
			// Missing environment and bookmaker_id
			Business: "internal",
		})

		err := client.ValidateMandatoryFields()
		assert.Error(t, err)

		// Check the error message contains expected content
		errorMsg := err.Error()
		assert.Contains(t, errorMsg, "environment")
		assert.Contains(t, errorMsg, "bookmaker_id")
		assert.NotContains(t, errorMsg, "team") // team should not be in missing fields
	})

	t.Run("AllFieldsPresentWithFallbacks", func(t *testing.T) {
		// Test that should pass because it has all required fields
		// but using fallback methods
		client := breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{
			Enabled:     true,
			APIKey:      "test-key",
			Team:        "test-team",
			Environment: "TEST",
			BookmakerID: "test-bookmaker-123",
			Business:    "internal",
			// Don't include hostname so it uses automatic fallback
		})

		err := client.ValidateMandatoryFields()

		// Detailed debugging
		t.Logf("Fallback validation result: err = %v", err)
		t.Logf("err == nil: %v", err == nil)

		if err != nil {
			t.Logf("Fallback error details: %v", err)
			t.Logf("Fallback error type: %T", err)
			t.Errorf("Expected no error with fallbacks but got: %v", err)
		} else {
			t.Logf("SUCCESS: Fallback validation passed as expected")
		}
	})
}

// TestWithDisabledTriggers verifies that disabled triggers don't send alerts
func TestWithDisabledTriggers(t *testing.T) {
	// We create a client but don't initialize it to avoid making real calls
	// We only test the initial validations

	// We can't do higher level tests without completely mocking the OpsGenie client
	// or without using an HTTP mocking library to intercept calls to the real API
}
