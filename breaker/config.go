package breaker

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
)

// Config This Config ios read from a toml file
type Config struct {
	MemoryThreshold      float64 `toml:"memory_threshold"`        // Percentage of memory usage
	LatencyThreshold     int64   `toml:"latency_threshold"`       // In milliseconds
	LatencyWindowSize    int     `toml:"latency_window_size"`     // Number of latencies to keep
	Percentile           float64 `toml:"percentile"`              // Percentile to use
	WaitTime             int     `toml:"wait_time"`               // Time to wait before reset LatencyWindow in seconds
	MaxLatencyAgeMinutes int     `toml:"max_latency_age_minutes"` // Maximum time in minutes to consider a latency valid
}

const configPath = "BreakerDriver-Config.toml"

var config *Config

func LoadConfig(path string) (*Config, error) {
	var config *Config = &Config{}
	_, err := toml.DecodeFile(path, config)
	return config, err
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
