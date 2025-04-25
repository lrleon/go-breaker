package breaker

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
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
}

// Config This Config ios read from a toml file
type Config struct {
	MemoryThreshold             float64 `toml:"memory_threshold"`                // Percentage of memory usage
	LatencyThreshold            int64   `toml:"latency_threshold"`               // In milliseconds
	LatencyWindowSize           int     `toml:"latency_window_size"`             // Number of latencies to keep
	Percentile                  float64 `toml:"percentile"`                      // Percentile to use
	WaitTime                    int     `toml:"wait_time"`                       // Time to wait before reset LatencyWindow in seconds
	TrendAnalysisEnabled        bool    `toml:"trend_analysis_enabled"`          // If true, breaker activates only if trend is positive
	TrendAnalysisMinSampleCount int     `toml:"trend_analysis_min_sample_count"` // Minimum number of samples for trend analysis

	// OpsGenie configuration (optional, loaded separately)
	OpsGenie *OpsGenieConfig `toml:"-"`
}

const configPath = "BreakerDriver-Config.toml"
const opsGenieConfigPath = "opsgenie.toml"

var config *Config

func LoadConfig(path string) (*Config, error) {
	var config *Config = &Config{}
	_, err := toml.DecodeFile(path, config)
	return config, err
}

// LoadOpsGenieConfig loads the OpsGenie configuration from the given path
func LoadOpsGenieConfig(path string) (*OpsGenieConfig, error) {
	var config *OpsGenieConfig = &OpsGenieConfig{}
	_, err := toml.DecodeFile(path, config)
	return config, err
}

// LoadFullConfig loads both the main configuration and the OpsGenie configuration
func LoadFullConfig(mainPath, opsGeniePath string) (*Config, error) {
	config, err := LoadConfig(mainPath)
	if err != nil {
		return nil, err
	}

	// Try to load OpsGenie config, but don't fail if it doesn't exist
	opsGenieConfig, err := LoadOpsGenieConfig(opsGeniePath)
	if err != nil {
		log.Printf("Warning: OpsGenie configuration not loaded: %v", err)
		// Set default empty config to avoid nil pointer
		config.OpsGenie = &OpsGenieConfig{Enabled: false}
	} else {
		config.OpsGenie = opsGenieConfig
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
