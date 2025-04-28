package tests

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigEnvironmentSettings verifies that environment settings for OpsGenie
// are loaded correctly from the TOML configuration file.
func TestConfigEnvironmentSettings(t *testing.T) {
	// Get the absolute path to example_config.toml
	configPath, err := filepath.Abs("../example_config.toml")
	require.NoError(t, err, "Failed to get absolute path to example_config.toml")

	// Load the configuration from the file
	config, err := breaker.LoadConfig(configPath)
	require.NoError(t, err, "Failed to load configuration from example_config.toml")
	require.NotNil(t, config, "Configuration should not be nil")

	// Verify that OpsGenie is configured
	require.NotNil(t, config.OpsGenie, "OpsGenie configuration should not be nil")

	// Verify that the use_environments option is enabled
	assert.True(t, config.OpsGenie.UseEnvironments, "UseEnvironments should be true")

	// Verify that environment_settings is present and not nil
	require.NotNil(t, config.OpsGenie.EnvironmentSettings,
		"EnvironmentSettings should not be nil")

	// Verify each specific environment in the settings
	envSettings := config.OpsGenie.EnvironmentSettings

	// Verify development environment
	devSettings, exists := envSettings["dev"]
	assert.True(t, exists, "Development environment settings should exist")
	if exists {
		assert.False(t, devSettings.Enabled, "Dev environment should have alerts disabled")
		assert.Equal(t, "P5", devSettings.Priority, "Dev environment should have P5 priority")
	}

	// Verify UAT environment
	uatSettings, exists := envSettings["uat"]
	assert.True(t, exists, "UAT environment settings should exist")
	if exists {
		assert.True(t, uatSettings.Enabled, "UAT environment should have alerts enabled")
		assert.Equal(t, "P3", uatSettings.Priority, "UAT environment should have P3 priority")
	}

	// Verify production environment
	prodSettings, exists := envSettings["production"]
	assert.True(t, exists, "Production environment settings should exist")
	if exists {
		assert.True(t, prodSettings.Enabled, "Production environment should have alerts enabled")
		assert.Equal(t, "P2", prodSettings.Priority, "Production environment should have P2 priority")
	}

	// Print settings for debugging
	fmt.Printf("OpsGenie UseEnvironments: %v\n", config.OpsGenie.UseEnvironments)
	fmt.Printf("Environment Settings: %+v\n", config.OpsGenie.EnvironmentSettings)

	// Save the configuration to a temporary file
	tempPath := filepath.Join(t.TempDir(), "saved_config.toml")
	err = breaker.SaveConfig(tempPath, config)
	require.NoError(t, err, "Failed to save configuration")

	// Reload the saved configuration to verify that settings were preserved
	reloadedConfig, err := breaker.LoadConfig(tempPath)
	require.NoError(t, err, "Failed to reload saved configuration")
	require.NotNil(t, reloadedConfig.OpsGenie, "Reloaded OpsGenie configuration should not be nil")

	// Verify that settings were preserved when saving and reloading
	assert.Equal(t, config.OpsGenie.UseEnvironments, reloadedConfig.OpsGenie.UseEnvironments,
		"UseEnvironments should be preserved after save and reload")

	assert.NotNil(t, reloadedConfig.OpsGenie.EnvironmentSettings,
		"EnvironmentSettings should not be nil after reload")

	// Verify that all environments were preserved
	for env, settings := range config.OpsGenie.EnvironmentSettings {
		reloadedSettings, exists := reloadedConfig.OpsGenie.EnvironmentSettings[env]
		assert.True(t, exists, "Environment '%s' should exist after reload", env)

		if exists {
			assert.Equal(t, settings.Enabled, reloadedSettings.Enabled,
				"Enabled setting for environment '%s' should be preserved", env)
			assert.Equal(t, settings.Priority, reloadedSettings.Priority,
				"Priority setting for environment '%s' should be preserved", env)
		}
	}

	// Print reloaded settings for debugging
	fmt.Printf("Reloaded OpsGenie UseEnvironments: %v\n", reloadedConfig.OpsGenie.UseEnvironments)
	fmt.Printf("Reloaded Environment Settings: %+v\n", reloadedConfig.OpsGenie.EnvironmentSettings)
}
