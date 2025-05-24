package breaker

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"log"
	"os"
	"sync"
)

// ContactInfo for escalation and contact details
type ContactInfo struct {
	PrimaryContact   string   `toml:"primary_contact"`   // Primary contact email/username
	EscalationTeam   string   `toml:"escalation_team"`   // Team to escalate to
	SlackChannel     string   `toml:"slack_channel"`     // Slack channel for notifications
	PhoneNumber      string   `toml:"phone_number"`      // Emergency contact number
	AdditionalEmails []string `toml:"additional_emails"` // Additional notification emails
}

// OpsGenieConfig represents the OpsGenie integration configuration with all mandatory fields
type OpsGenieConfig struct {
	// Basic OpsGenie Settings
	Enabled  bool     `toml:"enabled"`  // Enable OpsGenie alerts
	APIKey   string   `toml:"api_key"`  // OpsGenie API key
	Region   string   `toml:"region"`   // OpsGenie region: "us" or "eu"
	APIURL   string   `toml:"api_url"`  // Custom API URL (optional)
	Priority string   `toml:"priority"` // Default priority (P1-P5)
	Source   string   `toml:"source"`   // Source identifier
	Tags     []string `toml:"tags"`     // Alert tags

	// Alert Triggers
	TriggerOnOpen    bool `toml:"trigger_on_breaker_open"`      // Alert when breaker opens
	TriggerOnReset   bool `toml:"trigger_on_breaker_reset"`     // Alert when breaker resets
	TriggerOnMemory  bool `toml:"trigger_on_memory_threshold"`  // Alert on memory threshold breach
	TriggerOnLatency bool `toml:"trigger_on_latency_threshold"` // Alert on latency threshold breach

	// Alert Content
	IncludeLatencyMetrics bool `toml:"include_latency_metrics"` // Include latency metrics in alert
	IncludeMemoryMetrics  bool `toml:"include_memory_metrics"`  // Include memory metrics in alert
	IncludeSystemInfo     bool `toml:"include_system_info"`     // Include system info in alert

	// Rate Limiting
	AlertCooldownSeconds int `toml:"alert_cooldown_seconds"` // Minimum time between alerts

	// Environment Settings
	EnvironmentSettings map[string]EnvOpsConfig `toml:"environment_settings"` // Environment-specific settings
	UseEnvironments     bool                    `toml:"use_environments"`     // Whether to use environment-specific settings

	// MANDATORY FIELDS - Required for all alerts
	Team         string `toml:"team"`          // ✅ MANDATORY: OpsGenie team name (must match OpsGenie)
	Environment  string `toml:"environment"`   // ✅ MANDATORY: DEV, CI, UAT, PROD, etc. (fallback to "Environment" env var)
	BookmakerID  string `toml:"bookmaker_id"`  // ✅ MANDATORY: Project/Client ID
	ProjectID    string `toml:"project_id"`    // Alternative to bookmaker_id
	Hostname     string `toml:"hostname"`      // ✅ MANDATORY: Machine hostname (auto-detected if empty)
	HostOverride string `toml:"host_override"` // Manual hostname override
	Business     string `toml:"business"`      // ✅ MANDATORY: Business unit (internal, external, etc.)
	BusinessUnit string `toml:"business_unit"` // More specific business unit

	// ADDITIONAL CONTEXT - Custom information tag
	AdditionalContext string `toml:"additional_context"` // ✅ Optional: Any additional context you want to include

	// API Information (Enhanced)
	APINamespace        string            `toml:"api_namespace"`         // Namespace/environment of the API
	APIName             string            `toml:"api_name"`              // Name of the API being protected
	APIVersion          string            `toml:"api_version"`           // Version of the API
	APIOwner            string            `toml:"api_owner"`             // Team or individual responsible for the API
	APIDependencies     []string          `toml:"api_dependencies"`      // List of dependencies this API relies on
	APIEndpoints        []string          `toml:"api_endpoints"`         // List of important endpoints being protected
	APIDescription      string            `toml:"api_description"`       // Brief description of the API's purpose
	APIPriority         string            `toml:"api_priority"`          // Business priority of the API (critical, high, medium, low)
	APICustomAttributes map[string]string `toml:"api_custom_attributes"` // Any custom attributes for the API

	// Service Configuration
	ServiceTier    string      `toml:"service_tier"`    // critical, high, medium, low
	ContactDetails ContactInfo `toml:"contact_details"` // Contact information
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

// Config represents the main circuit breaker configuration
type Config struct {
	// Core Circuit Breaker Settings
	MemoryThreshold             float64 `toml:"memory_threshold"`                // Percentage of memory usage
	LatencyThreshold            int64   `toml:"latency_threshold"`               // In milliseconds
	LatencyWindowSize           int     `toml:"latency_window_size"`             // Number of latencies to keep
	Percentile                  float64 `toml:"percentile"`                      // Percentile to use
	WaitTime                    int     `toml:"wait_time"`                       // Time to wait before reset in seconds
	TrendAnalysisEnabled        bool    `toml:"trend_analysis_enabled"`          // If true, breaker activates only if trend is positive
	TrendAnalysisMinSampleCount int     `toml:"trend_analysis_min_sample_count"` // Minimum number of samples for trend analysis

	// OpsGenie Integration
	OpsGenie *OpsGenieConfig `toml:"opsgenie"` // OpsGenie configuration
}

// Configuration file paths
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
func SetOpsGenieConfigPath(path string) {
	opsGeniePathMu.Lock()
	defer opsGeniePathMu.Unlock()
	opsGenieConfigPath = path
}

// createDefaultConfig creates a default configuration with reasonable values
func createDefaultConfig() *Config {
	return &Config{
		MemoryThreshold:             85.0,
		LatencyThreshold:            3000,
		LatencyWindowSize:           256,
		Percentile:                  0.95,
		WaitTime:                    4,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 10,
		OpsGenie: &OpsGenieConfig{
			Enabled:               false,
			Region:                "us",
			Priority:              "P3",
			Source:                "go-breaker",
			Tags:                  []string{"circuit-breaker"},
			TriggerOnOpen:         true,
			TriggerOnReset:        false,
			TriggerOnMemory:       true,
			TriggerOnLatency:      true,
			IncludeLatencyMetrics: true,
			IncludeMemoryMetrics:  true,
			IncludeSystemInfo:     true,
			AlertCooldownSeconds:  300,
			UseEnvironments:       false,
			// Mandatory fields with fallback indicators
			Team:              "",         // Will be validated and use fallbacks
			Environment:       "",         // Will read from "Environment" env var if empty
			BookmakerID:       "",         // Will use environment variables if empty
			Business:          "internal", // Default value
			AdditionalContext: "",         // Optional field
			// API Information defaults
			APINamespace:   "",
			APIName:        "",
			APIVersion:     "",
			APIOwner:       "",
			APIDescription: "",
			APIPriority:    "",
			ServiceTier:    "",
		},
	}
}

// LoadConfig loads configuration from a TOML file with enhanced error handling and validation
func LoadConfig(path string) (*Config, error) {
	// Start with default config
	defaultConfig := createDefaultConfig()

	// Try to parse with the root-level structure
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
			return defaultConfig, err
		}
	}

	// Validate and set defaults for any zero or invalid values
	if config.MemoryThreshold <= 0 || config.MemoryThreshold > 100 {
		log.Printf("Warning: Invalid memory_threshold (%.2f). Using default value of %.2f.",
			config.MemoryThreshold, defaultConfig.MemoryThreshold)
		config.MemoryThreshold = defaultConfig.MemoryThreshold
	}

	if config.LatencyThreshold <= 0 {
		log.Printf("Warning: Invalid latency_threshold (%d). Using default value of %d.",
			config.LatencyThreshold, defaultConfig.LatencyThreshold)
		config.LatencyThreshold = defaultConfig.LatencyThreshold
	}

	if config.LatencyWindowSize <= 0 {
		log.Printf("Warning: Invalid latency_window_size (%d). Using default value of %d.",
			config.LatencyWindowSize, defaultConfig.LatencyWindowSize)
		config.LatencyWindowSize = defaultConfig.LatencyWindowSize
	}

	if config.Percentile <= 0 || config.Percentile > 1 {
		log.Printf("Warning: Invalid percentile (%.2f). Using default value of %.2f.",
			config.Percentile, defaultConfig.Percentile)
		config.Percentile = defaultConfig.Percentile
	}

	if config.WaitTime <= 0 {
		log.Printf("Warning: Invalid wait_time (%d). Using default value of %d.",
			config.WaitTime, defaultConfig.WaitTime)
		config.WaitTime = defaultConfig.WaitTime
	}

	if config.TrendAnalysisMinSampleCount <= 0 {
		config.TrendAnalysisMinSampleCount = defaultConfig.TrendAnalysisMinSampleCount
	}

	// Initialize OpsGenie config if nil
	if config.OpsGenie == nil {
		log.Printf("No OpsGenie configuration found, using defaults")
		config.OpsGenie = defaultConfig.OpsGenie
	} else {
		// Validate and set defaults for OpsGenie config
		validateAndSetOpsGenieDefaults(config.OpsGenie, defaultConfig.OpsGenie)
	}

	log.Printf("Config loaded successfully:")
	log.Printf("  - Memory threshold: %.2f%%", config.MemoryThreshold)
	log.Printf("  - Latency threshold: %dms", config.LatencyThreshold)
	log.Printf("  - Latency window size: %d", config.LatencyWindowSize)
	log.Printf("  - Percentile: %.2f", config.Percentile)
	log.Printf("  - Wait time: %ds", config.WaitTime)
	log.Printf("  - Trend analysis: %t", config.TrendAnalysisEnabled)

	if config.OpsGenie != nil {
		log.Printf("  - OpsGenie enabled: %t", config.OpsGenie.Enabled)
		if config.OpsGenie.Enabled {
			log.Printf("    - Team: %s", config.OpsGenie.Team)
			log.Printf("    - Environment: %s", config.OpsGenie.Environment)
			log.Printf("    - BookmakerID: %s", config.OpsGenie.BookmakerID)
			log.Printf("    - Business: %s", config.OpsGenie.Business)
			if config.OpsGenie.AdditionalContext != "" {
				log.Printf("    - Additional Context: %s", config.OpsGenie.AdditionalContext)
			}
		}
	}

	return &config, nil
}

// validateAndSetOpsGenieDefaults validates OpsGenie configuration and sets defaults
func validateAndSetOpsGenieDefaults(config *OpsGenieConfig, defaults *OpsGenieConfig) {
	if config.Region == "" {
		config.Region = defaults.Region
	}

	if config.Priority == "" {
		config.Priority = defaults.Priority
	}

	if config.Source == "" {
		config.Source = defaults.Source
	}

	if len(config.Tags) == 0 {
		config.Tags = defaults.Tags
	}

	if config.AlertCooldownSeconds <= 0 {
		config.AlertCooldownSeconds = defaults.AlertCooldownSeconds
	}

	// Validate mandatory field defaults
	if config.Business == "" {
		config.Business = defaults.Business // "internal"
	}

	// Validate priority format
	validPriorities := map[string]bool{
		"P1": true, "P2": true, "P3": true, "P4": true, "P5": true,
	}
	if !validPriorities[config.Priority] {
		log.Printf("Warning: Invalid OpsGenie priority '%s'. Using default '%s'",
			config.Priority, defaults.Priority)
		config.Priority = defaults.Priority
	}

	// Validate region
	validRegions := map[string]bool{
		"us": true, "eu": true,
	}
	if !validRegions[config.Region] {
		log.Printf("Warning: Invalid OpsGenie region '%s'. Using default '%s'",
			config.Region, defaults.Region)
		config.Region = defaults.Region
	}
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

	if config.OpsGenie == nil || (!config.OpsGenie.Enabled && config.OpsGenie.APIKey == "" &&
		config.OpsGenie.Region == "" && len(config.OpsGenie.Tags) == 0) {
		log.Printf("OpsGenie configuration not found in main config, checking separate file...")

		// Try to load OpsGenie config from separate file, but don't fail if it doesn't exist
		opsGenieConfig, err := LoadOpsGenieConfig(opsGeniePath)
		if err != nil {
			log.Printf("Warning: Separate OpsGenie configuration not loaded: %v", err)
			// Make sure we have a non-nil OpsGenie config
			if config.OpsGenie == nil {
				config.OpsGenie = createDefaultConfig().OpsGenie
			}
		} else {
			config.OpsGenie = opsGenieConfig
		}
	} else {
		log.Printf("Using OpsGenie configuration from main config file")
	}

	return config, nil
}

// SaveConfig saves the configuration to a TOML file with enhanced validation
func SaveConfig(path string, config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate configuration before saving
	if err := ValidateConfig(config); err != nil {
		log.Printf("Warning: Configuration validation failed: %v", err)
		// Continue saving but log warnings
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()

	log.Printf("Saving config to %s", path)

	encoder := toml.NewEncoder(file)
	err = encoder.Encode(config)
	if err != nil {
		return fmt.Errorf("failed to encode config: %v", err)
	}

	log.Printf("Config saved successfully:")
	log.Printf("  - Memory threshold: %.2f%%", config.MemoryThreshold)
	log.Printf("  - Latency threshold: %dms", config.LatencyThreshold)
	log.Printf("  - Latency window size: %d", config.LatencyWindowSize)
	log.Printf("  - Percentile: %.2f", config.Percentile)
	log.Printf("  - Wait time: %ds", config.WaitTime)
	log.Printf("  - Trend analysis enabled: %t", config.TrendAnalysisEnabled)

	if config.OpsGenie != nil && config.OpsGenie.Enabled {
		log.Printf("  - OpsGenie configuration saved with mandatory fields")
	}

	return nil
}

// ValidateConfig validates the entire configuration
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	var errors []string

	// Validate core breaker settings
	if config.MemoryThreshold <= 0 || config.MemoryThreshold > 100 {
		errors = append(errors, fmt.Sprintf("invalid memory_threshold: %.2f (must be between 0 and 100)", config.MemoryThreshold))
	}

	if config.LatencyThreshold <= 0 {
		errors = append(errors, fmt.Sprintf("invalid latency_threshold: %d (must be positive)", config.LatencyThreshold))
	}

	if config.LatencyWindowSize <= 0 {
		errors = append(errors, fmt.Sprintf("invalid latency_window_size: %d (must be positive)", config.LatencyWindowSize))
	}

	if config.Percentile <= 0 || config.Percentile > 1 {
		errors = append(errors, fmt.Sprintf("invalid percentile: %.2f (must be between 0 and 1)", config.Percentile))
	}

	if config.WaitTime < 0 {
		errors = append(errors, fmt.Sprintf("invalid wait_time: %d (must be non-negative)", config.WaitTime))
	}

	// Validate OpsGenie config if present
	if config.OpsGenie != nil {
		if err := ValidateOpsGenieConfig(config.OpsGenie); err != nil {
			errors = append(errors, fmt.Sprintf("OpsGenie config validation failed: %v", err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation errors: %v", errors)
	}

	return nil
}

// ValidateOpsGenieConfig validates the OpsGenie configuration
func ValidateOpsGenieConfig(config *OpsGenieConfig) error {
	if config == nil {
		return nil // OpsGenie config is optional
	}

	var errors []string

	// Validate region
	validRegions := map[string]bool{"us": true, "eu": true}
	if config.Region != "" && !validRegions[config.Region] {
		errors = append(errors, fmt.Sprintf("invalid region: %s (must be 'us' or 'eu')", config.Region))
	}

	// Validate priority
	validPriorities := map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true, "P5": true}
	if config.Priority != "" && !validPriorities[config.Priority] {
		errors = append(errors, fmt.Sprintf("invalid priority: %s (must be P1-P5)", config.Priority))
	}

	// Validate cooldown
	if config.AlertCooldownSeconds < 0 {
		errors = append(errors, fmt.Sprintf("invalid alert_cooldown_seconds: %d (must be non-negative)", config.AlertCooldownSeconds))
	}

	// Validate mandatory fields if OpsGenie is enabled
	if config.Enabled {
		if config.Team == "" {
			errors = append(errors, "team is required when OpsGenie is enabled")
		}
		// Note: Other mandatory fields (Environment, BookmakerID, etc.) are validated at runtime
		// because they can use environment variables and auto-detection
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %v", errors)
	}

	return nil
}

// SaveOpsGenieConfig saves the OpsGenie configuration to the given path
// Deprecated: Consider using SaveConfig to save the full configuration including OpsGenie
func SaveOpsGenieConfig(path string, config *OpsGenieConfig) error {
	if config == nil {
		return fmt.Errorf("OpsGenie config cannot be nil")
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create OpsGenie config file: %v", err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(config)
}

// GetConfigSummary returns a summary of the current configuration for logging/debugging
func GetConfigSummary(config *Config) map[string]interface{} {
	if config == nil {
		return map[string]interface{}{"error": "config is nil"}
	}

	summary := map[string]interface{}{
		"memory_threshold":                config.MemoryThreshold,
		"latency_threshold":               config.LatencyThreshold,
		"latency_window_size":             config.LatencyWindowSize,
		"percentile":                      config.Percentile,
		"wait_time":                       config.WaitTime,
		"trend_analysis_enabled":          config.TrendAnalysisEnabled,
		"trend_analysis_min_sample_count": config.TrendAnalysisMinSampleCount,
	}

	if config.OpsGenie != nil {
		opsGenieSummary := map[string]interface{}{
			"enabled":                config.OpsGenie.Enabled,
			"region":                 config.OpsGenie.Region,
			"priority":               config.OpsGenie.Priority,
			"team":                   config.OpsGenie.Team,
			"environment":            config.OpsGenie.Environment,
			"bookmaker_id":           config.OpsGenie.BookmakerID,
			"business":               config.OpsGenie.Business,
			"additional_context":     config.OpsGenie.AdditionalContext,
			"alert_cooldown_seconds": config.OpsGenie.AlertCooldownSeconds,
			"use_environments":       config.OpsGenie.UseEnvironments,
		}
		summary["opsgenie"] = opsGenieSummary
	}

	return summary
}
