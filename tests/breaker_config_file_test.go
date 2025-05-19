package tests

import (
	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_Breaker_Saves_To_Original_Config_File(t *testing.T) {
	// Create a temporary configuration file for the test
	tempConfigFile := "temp_breaker_config.toml"

	// Initial configuration
	initialConfig := &breaker.Config{
		MemoryThreshold:   0.85,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10,
	}

	// Save the initial configuration in the temporary file
	err := breaker.SaveConfig(tempConfigFile, initialConfig)
	assert.NoError(t, err, "Failed to save initial config")

	// Create a break from the configuration file
	b, err := breaker.NewBreakerFromConfigFile(tempConfigFile)
	assert.NoError(t, err, "Failed to create breaker from config file")

	// Verify that the configuration file has been saved correctly
	driver := b.(*breaker.BreakerDriver)
	assert.Equal(t, tempConfigFile, driver.GetConfigFile(), "Config file path was not stored correctly")

	// Modify the configuration
	newLatencyThreshold := int64(800)

	// Get the breakrapi to simulate a call from an endpoint
	api := &breaker.BreakerAPI{
		Config: *initialConfig,
		Driver: b,
	}

	// Modify the configuration through the API (simulating an endpoint)
	api.Config.LatencyThreshold = newLatencyThreshold
	configFile := api.Driver.GetConfigFile()
	err = breaker.SaveConfig(configFile, &api.Config)
	assert.NoError(t, err, "Failed to save modified config")

	// Load the configuration again to verify that the changes were saved
	loadedConfig, err := breaker.LoadConfig(tempConfigFile)
	assert.NoError(t, err, "Failed to load config")
	assert.Equal(t, newLatencyThreshold, loadedConfig.LatencyThreshold, "Latency threshold was not updated in the config file")

	// Cleanup
	os.Remove(tempConfigFile)
}
