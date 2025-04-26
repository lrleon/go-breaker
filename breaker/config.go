package breaker

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"sync"
)

// OpsGenieConfig represents the OpsGenie integration configuration
type OpsGenieConfig struct {
	Enabled               bool     `toml:"enabled"`                      // Enable OpsGenie alerts
	APIKey                string   `toml:"api_key"`                      // OpsGenie API key
	Region                string   `toml:"region"`                       // OpsGenie region: "us" or "eu"
	APIURL                string   `toml:"api_url"`                      // Custom API URL (optional)
	Priority              string   `toml:"priority"`                     // Default priority (P1-P5)
	Source                string   `toml:"source"`                       // Source identifier
	Team                  string   `toml:"team"`                         // Team to assign alerts to
	Tags                  []string `toml:"tags"`                         // Alert tags
	TriggerOnOpen         bool     `toml:"trigger_on_breaker_open"`      // Alert when breaker opens
	TriggerOnReset        bool     `toml:"trigger_on_breaker_reset"`     // Alert when breaker resets
	TriggerOnMemory       bool     `toml:"trigger_on_memory_threshold"`  // Alert on memory threshold breach
	TriggerOnLatency      bool     `toml:"trigger_on_latency_threshold"` // Alert on latency threshold breach
	IncludeLatencyMetrics bool     `toml:"include_latency_metrics"`      // Include latency metrics in alert
	IncludeMemoryMetrics  bool     `toml:"include_memory_metrics"`       // Include memory metrics in alert
	IncludeSystemInfo     bool     `toml:"include_system_info"`          // Include system info in alert
	AlertCooldownSeconds  int      `toml:"alert_cooldown_seconds"`       // Minimum time between alerts

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
	var config Config
	_, err := toml.DecodeFile(path, &config)

	// If OpsGenie config is nil, initialize with default values
	if config.OpsGenie == nil {
		config.OpsGenie = &OpsGenieConfig{Enabled: false}
	}

	return &config, err
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
