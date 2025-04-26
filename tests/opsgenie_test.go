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
	// We cannot directly access client.config.Enabled since it's a private field
	// but we can verify that IsInitialized() works correctly
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
	// Test with nil client
	var client *breaker.OpsGenieClient
	err := client.Initialize()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpsGenieClient is nil")

	// Test with disabled client
	client = breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{Enabled: false})
	err = client.Initialize()
	assert.NoError(t, err)
	assert.False(t, client.IsInitialized())

	// Save original env vars
	originalAPIKey := os.Getenv(breaker.EnvOpsGenieAPIKey)
	originalRegion := os.Getenv(breaker.EnvOpsGenieRegion)
	originalAPIURL := os.Getenv(breaker.EnvOpsGenieAPIURL)
	defer func() {
		os.Setenv(breaker.EnvOpsGenieAPIKey, originalAPIKey)
		os.Setenv(breaker.EnvOpsGenieRegion, originalRegion)
		os.Setenv(breaker.EnvOpsGenieAPIURL, originalAPIURL)
	}()

	// Test with missing API key
	client = breaker.NewOpsGenieClient(&breaker.OpsGenieConfig{Enabled: true})
	os.Unsetenv(breaker.EnvOpsGenieAPIKey)
	err = client.Initialize()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OpsGenie API key not found")

	// Skipping actual API connection tests as they would need real credentials
	// Those would be integration tests rather than unit tests
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

// TestSendingAlerts verifies that alert sending methods return errors
// when the client is not initialized
func TestSendingAlerts(t *testing.T) {
	// Test with nil client
	var nilClient *breaker.OpsGenieClient

	// Test breaker open alert
	err := nilClient.SendBreakerOpenAlert(100, true, 60)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test breaker reset alert
	err = nilClient.SendBreakerResetAlert()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test memory threshold alert
	memStatus := &breaker.MemoryStatus{
		CurrentUsage: 85.0,
		Threshold:    80.0,
		TotalMemory:  1024 * 1024 * 1024,
		UsedMemory:   850 * 1024 * 1024,
		OK:           false,
	}
	err = nilClient.SendMemoryThresholdAlert(memStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")

	// Test latency threshold alert
	err = nilClient.SendLatencyThresholdAlert(100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not initialized")
}

// TestWithDisabledTriggers verifies that disabled triggers don't send alerts
func TestWithDisabledTriggers(t *testing.T) {
	// We create a client but don't initialize it to avoid making real calls
	// We only test the initial validations

	// We can't do higher level tests without completely mocking the OpsGenie client
	// or without using an HTTP mocking library to intercept calls to the real API
}
