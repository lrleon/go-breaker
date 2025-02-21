package breaker

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
)

// Config This config ios read from a toml file
type Config struct {
	MemoryThreshold   float64 `toml:"memory_threshold"`    // Percentage of memory usage
	LatencyThreshold  int64   `toml:"latency_threshold"`   // In milliseconds
	LatencyWindowSize int     `toml:"latency_window_size"` // Number of latencies to keep
	Percentile        float64 `toml:"percentile"`          // Percentile to use
	WaitTime          int     `toml:"wait_time"`           // Time to wait before reset latencyWindow in seconds
}

const configPath = "breaker-config.toml"

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

func initConfig() {
	var err error

	config, err = LoadConfig(configPath)
	if err != nil {
		log.Printf("Error loading config file: %v", err)
		log.Printf("Using default config")
		config = &Config{
			MemoryThreshold:   80,
			LatencyThreshold:  1500,
			LatencyWindowSize: 64,
			Percentile:        0.95,
		}
		// Save the default config to the file
		err := SaveConfig(configPath, config)
		if err != nil {
			log.Panicf("Error saving default config: %v", err)
		}
	}

	log.Printf("Config: %v", config)
}
