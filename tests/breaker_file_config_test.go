package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBreakerFromConfigFile(t *testing.T) {
	// Create a temporary TOML config file for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.toml")

	// Basic configuration to write to the file
	configContent := `
memory_threshold = 80.0
latency_threshold = 500
latency_window_size = 100
percentile = 95.0
wait_time = 60
trend_analysis_enabled = true
trend_analysis_min_sample_count = 5

[opsgenie]
enabled = false
`

	// Write the configuration to the file
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Failed to write test config file")

	// Test loading the breaker from the config file
	b, err := breaker.NewBreakerFromConfigFile(configPath)
	require.NoError(t, err, "Failed to create breaker from config file")
	require.NotNil(t, b, "Breaker should not be nil")

	// Test basic breaker functionality to verify it was configured properly
	assert.True(t, b.IsEnabled(), "Breaker should be enabled by default")

	// Test with non-existent file
	_, err = breaker.NewBreakerFromConfigFile("/path/does/not/exist.toml")
	assert.Error(t, err, "Should error with non-existent file")

	// Test with invalid TOML content
	invalidConfigPath := filepath.Join(tmpDir, "invalid_config.toml")
	err = os.WriteFile(invalidConfigPath, []byte("this is not a valid TOML file"), 0644)
	require.NoError(t, err, "Failed to write invalid test config file")

	_, err = breaker.NewBreakerFromConfigFile(invalidConfigPath)
	assert.Error(t, err, "Should error with invalid TOML")
}

// TestBreakerWithOpsGenieEnabled verifies the case when OpsGenie is enabled in the config
func TestBreakerWithOpsGenieEnabled(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Main configuration with OpsGenie enabled but without API Key
	mainConfigPath := filepath.Join(tmpDir, "main_config.toml")
	mainConfig := `
memory_threshold = 80.0
latency_threshold = 500
latency_window_size = 100
percentile = 95.0
wait_time = 60

[opsgenie]
enabled = true
trigger_on_breaker_open = true
trigger_on_breaker_reset = true
alert_cooldown_seconds = 300
`
	err := os.WriteFile(mainConfigPath, []byte(mainConfig), 0644)
	require.NoError(t, err, "Failed to write main config file")

	// Create separate OpsGenie file with API Key
	opsGenieConfigPath := filepath.Join(tmpDir, "opsgenie.toml")
	opsGenieConfig := `
enabled = true
api_key = "test-api-key"
region = "us"
priority = "P2"
team = "test-team"
tags = ["test", "circuit-breaker"]
trigger_on_breaker_open = true
trigger_on_breaker_reset = true
alert_cooldown_seconds = 300
api_name = "Test API"
api_version = "v1.0"
`
	err = os.WriteFile(opsGenieConfigPath, []byte(opsGenieConfig), 0644)
	require.NoError(t, err, "Failed to write OpsGenie config file")

	// Save original OpsGenie file path
	originalOpsGeniePath := breaker.GetOpsGenieConfigPath()
	// Restore at the end of the test
	defer breaker.SetOpsGenieConfigPath(originalOpsGeniePath)

	// Set new path for the test
	breaker.SetOpsGenieConfigPath(opsGenieConfigPath)

	// Now load the breaker from the configuration file
	b, err := breaker.NewBreakerFromConfigFile(mainConfigPath)
	require.NoError(t, err, "Failed to create breaker from config file with OpsGenie enabled")
	require.NotNil(t, b, "Breaker should not be nil")

	// Since initialization happens in a goroutine, wait a bit
	time.Sleep(100 * time.Millisecond)

	// We cannot verify directly if OpsGenie initialized correctly
	// because the internal structure is not accessible, but we can verify
	// that the breaker was created successfully with the correct configuration
	assert.True(t, b.IsEnabled(), "Breaker should be enabled")
}
