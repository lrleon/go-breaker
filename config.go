package breaker

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
)

// Config This config ios read from a toml file
type Config struct {
	MemoryThreshold   float64 `toml:"memory_threshold"`
	LatencyThreshold  int64   `toml:"latency_threshold"`
	LatencyWindowSize int     `toml:"latency_window_size"`
	Percentile        float64 `toml:"percentile"`
}

const configPath = "config.toml"

var config *Config

func loadConfig(path string) (*Config, error) {
	var config *Config = &Config{}
	_, err := toml.DecodeFile(path, config)
	return config, err
}

func saveConfig(path string, config *Config) error {
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

	config, err = loadConfig(configPath)
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
		err := saveConfig(configPath, config)
		if err != nil {
			log.Panicf("Error saving default config: %v", err)
		}
	}

	log.Printf("Config: %v", config)
}
