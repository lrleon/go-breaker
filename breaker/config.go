package breaker

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"sync"
)

// OpsGenieConfig represents the OpsGenie integration configuration
type OpsGenieConfig struct {
	Enabled               bool                    `toml:"enabled"`                      // Enable OpsGenie alerts
	APIKey                string                  `toml:"api_key"`                      // OpsGenie API key
	Region                string                  `toml:"region"`                       // OpsGenie region: "us" or "eu"
	APIURL                string                  `toml:"api_url"`                      // Custom API URL (optional)
	Priority              string                  `toml:"priority"`                     // Default priority (P1-P5)
	Source                string                  `toml:"source"`                       // Source identifier
	Team                  string                  `toml:"team"`                         // Team to assign alerts to
	Tags                  []string                `toml:"tags"`                         // Alert tags
	TriggerOnOpen         bool                    `toml:"trigger_on_breaker_open"`      // Alert when breaker opens
	TriggerOnReset        bool                    `toml:"trigger_on_breaker_reset"`     // Alert when breaker resets
	TriggerOnMemory       bool                    `toml:"trigger_on_memory_threshold"`  // Alert on memory threshold breach
	TriggerOnLatency      bool                    `toml:"trigger_on_latency_threshold"` // Alert on latency threshold breach
	IncludeLatencyMetrics bool                    `toml:"include_latency_metrics"`      // Include latency metrics in alert
	IncludeMemoryMetrics  bool                    `toml:"include_memory_metrics"`       // Include memory metrics in alert
	IncludeSystemInfo     bool                    `toml:"include_system_info"`          // Include system info in alert
	AlertCooldownSeconds  int                     `toml:"alert_cooldown_seconds"`       // Minimum time between alerts
	EnvironmentSettings   map[string]EnvOpsConfig `toml:"environment_settings"`         // Environment-specific settings
	UseEnvironments       bool                    `toml:"use_environments"`             // Whether to use environment-specific settings

	// API Information
	APINamespace        string            `toml:"api_namespace"`         // Namespace/environment of the API (e.g., production, staging)
	APIName             string            `toml:"api_name"`              // Name of the API being protected
	APIVersion          string            `toml:"api_version"`           // Version of the API
	APIOwner            string            `toml:"api_owner"`             // Team or individual responsible for the API
	APIDependencies     []string          `toml:"api_dependencies"`      // List of dependencies this API relies on
	APIEndpoints        []string          `toml:"api_endpoints"`         // List of important endpoints being protected
	APIDescription      string            `toml:"api_description"`       // Brief description of the API's purpose
	APIPriority         string            `toml:"api_priority"`          // Business priority of the API (critical, high, medium, low)
	APICustomAttributes map[string]string `toml:"api_custom_attributes"` // Any custom attributes for the API
}

// EnvOpsConfig contains environment-specific OpsGenie settings
type EnvOpsConfig struct {
	Enabled  bool   `toml:"enabled"`  // Whether alerts are enabled in this environment
	Priority string `toml:"priority"` // Alert priority for this environment (P1-P5)
}

// Environment types for the application
type Environment string

const (
	EnvDevelopment Environment = "dev"
	EnvUAT         Environment = "uat"
	EnvProduction  Environment = "production"
)

// Config This Config ios read from a toml file
type Config struct {
	MemoryThreshold             float64         `toml:"memory_threshold"`                // Percentage of memory usage
	LatencyThreshold            int64           `toml:"latency_threshold"`               // In milliseconds
	LatencyWindowSize           int             `toml:"latency_window_size"`             // Number of latencies to keep
	Percentile                  float64         `toml:"percentile"`                      // Percentile to use
	WaitTime                    int             `toml:"wait_time"`                       // Time to wait before reset LatencyWindow in seconds
	TrendAnalysisEnabled        bool            `toml:"trend_analysis_enabled"`          // If true, breaker activates only if trend is positive
	TrendAnalysisMinSampleCount int             `toml:"trend_analysis_min_sample_count"` // Minimum number of samples for trend analysis
	OpsGenie                    *OpsGenieConfig `toml:"opsgenie"`                        // OpsGenie configuration
}

const configPath = "breakers.toml"

// Deprecated: Now using the opsgenie section in the main config file
var opsGenieConfigPath = "opsgenie.toml"
var opsGeniePathMu sync.RWMutex

// GetOpsGenieConfigPath returns the current path to the OpsGenie configuration file
func GetOpsGenieConfigPath() string {
	opsGeniePathMu.RLock()
	defer opsGeniePathMu.RUnlock()
	return opsGenieConfigPath
}

// SetOpsGenieConfigPath sets the path to the OpsGenie configuration file
// This is useful for testing or when the file is in a non-standard location
func SetOpsGenieConfigPath(path string) {
	opsGeniePathMu.Lock()
	defer opsGeniePathMu.Unlock()
	opsGenieConfigPath = path
}

var config *Config

func LoadConfig(path string) (*Config, error) {
	// Default config with reasonable values
	defaultConfig := Config{
		MemoryThreshold:             85.0,
		LatencyThreshold:            3000,
		LatencyWindowSize:           256,
		Percentile:                  0.95,
		WaitTime:                    4,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 10,
	}

	// First try to parse with the root-level structure
	var config Config
	_, err := toml.DecodeFile(path, &config)

	// If we failed to load or all values are zero, try the [circuit_breaker] format
	if err != nil || (config.MemoryThreshold == 0 && config.LatencyThreshold == 0 &&
		config.LatencyWindowSize == 0 && config.Percentile == 0) {

		// Try to parse with the section-based structure
		type ConfigWithSections struct {
			CircuitBreaker Config          `toml:"circuit_breaker"`
			OpsGenie       *OpsGenieConfig `toml:"opsgenie"`
		}

		var sectionConfig ConfigWithSections
		_, sectionErr := toml.DecodeFile(path, &sectionConfig)

		if sectionErr == nil {
			// Use values from the circuit_breaker section
			config = sectionConfig.CircuitBreaker

			// Preserve OpsGenie config if it was loaded in the section format
			if sectionConfig.OpsGenie != nil {
				config.OpsGenie = sectionConfig.OpsGenie
			}
			log.Printf("Loaded configuration using [circuit_breaker] section format")
		} else if err != nil {
			log.Printf("Warning: Error loading config from %s: %v. Using default values.", path, err)
			return &defaultConfig, err
		}
	}

	// Validate and set defaults for any zero values
	if config.MemoryThreshold <= 0 {
		log.Printf("Warning: Invalid memory_threshold (%f). Using default value of %f.",
			config.MemoryThreshold, defaultConfig.MemoryThreshold)
		config.MemoryThreshold = defaultConfig.MemoryThreshold
	}

	if config.LatencyThreshold <= 0 {
		config.LatencyThreshold = defaultConfig.LatencyThreshold
	}

	if config.LatencyWindowSize <= 0 {
		config.LatencyWindowSize = defaultConfig.LatencyWindowSize
	}

	if config.Percentile <= 0 || config.Percentile > 1 {
		config.Percentile = defaultConfig.Percentile
	}

	if config.WaitTime <= 0 {
		config.WaitTime = defaultConfig.WaitTime
	}

	if config.TrendAnalysisMinSampleCount <= 0 {
		config.TrendAnalysisMinSampleCount = defaultConfig.TrendAnalysisMinSampleCount
	}

	// If OpsGenie config is nil, initialize with default values
	if config.OpsGenie == nil {
		config.OpsGenie = &OpsGenieConfig{Enabled: false}
	}

	log.Printf("Config loaded: Memory threshold: %.2f%%, Latency threshold: %dms",
		config.MemoryThreshold, config.LatencyThreshold)

	return &config, nil
}

// LoadOpsGenieConfig loads the OpsGenie configuration from the given path
// Deprecated: Use LoadConfig instead, which now loads the OpsGenie configuration from the [opsgenie] section
func LoadOpsGenieConfig(path string) (*OpsGenieConfig, error) {
	var config OpsGenieConfig
	_, err := toml.DecodeFile(path, &config)
	return &config, err
}

// LoadFullConfig loads both the main configuration and the OpsGenie configuration
// For backward compatibility, it will first check if OpsGenie is configured in the main file,
// and if not, it will try to load from the separate file.
func LoadFullConfig(mainPath, opsGeniePath string) (*Config, error) {
	config, err := LoadConfig(mainPath)
	if err != nil {
		return nil, err
	}

	if config.OpsGenie == nil || (!config.OpsGenie.Enabled && config.OpsGenie.APIKey == "" && config.OpsGenie.Region == "" && len(config.OpsGenie.Tags) == 0) {
		log.Printf("OpsGenie configuration not found in main config, checking separate file...")

		// Try to load OpsGenie config from separate file, but don't fail if it doesn't exist
		opsGenieConfig, err := LoadOpsGenieConfig(opsGeniePath)
		if err != nil {
			log.Printf("Warning: Separate OpsGenie configuration not loaded: %v", err)
			// Make sure we have a non-nil OpsGenie config
			if config.OpsGenie == nil {
				config.OpsGenie = &OpsGenieConfig{Enabled: false}
			}
		} else {
			config.OpsGenie = opsGenieConfig
		}
	} else {
		log.Printf("Using OpsGenie configuration from main config file")
	}

	return config, nil
}

func SaveConfig(path string, config *Config) error {
	file, err := os.Create(path)

	log.Printf("Saving config to %s", path)

	// print the config to the console for debugging
	log.Printf("Config: %+v", config)

	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	encoder := toml.NewEncoder(file)
	err = encoder.Encode(config)
	if err != nil {
		return err
	}

	log.Printf("Config saved: Memory threshold: %.2f%%, Latency threshold: %dms, Latency window size: %d, Percentile: %.2f, Wait time: %d, Trend analysis min sample count: %d",
		config.MemoryThreshold, config.LatencyThreshold, config.LatencyWindowSize, config.Percentile, config.WaitTime, config.TrendAnalysisMinSampleCount)

	return nil
}

// SaveOpsGenieConfig saves the OpsGenie configuration to the given path
// Deprecated: Consider using SaveConfig to save the full configuration including OpsGenie
func SaveOpsGenieConfig(path string, config *OpsGenieConfig) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	encoder := toml.NewEncoder(file)
	return encoder.Encode(config)
}
