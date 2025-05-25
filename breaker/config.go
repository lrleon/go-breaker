package breaker

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
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
	APINamespace    string   `toml:"api_namespace"`    // Namespace/environment of the API
	APIName         string   `toml:"api_name"`         // Name of the API being protected
	APIVersion      string   `toml:"api_version"`      // Version of the API
	APIOwner        string   `toml:"api_owner"`        // Team or individual responsible for the API
	APIDependencies []string `toml:"api_dependencies"` // List of dependencies this API relies on
	APIEndpoints    []string `toml:"api_endpoints"`    // List of important endpoints being protected
	APIDescription  string   `toml:"api_description"`  // Brief description of the API's purpose
	APIPriority     string   `toml:"api_priority"`     // Business priority of the API (critical, high, medium, low)

	// Service Configuration
	ServiceTier    string      `toml:"service_tier"`    // critical, high, medium, low
	ContactDetails ContactInfo `toml:"contact_details"` // Contact information
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

// TOMLValidationError representa un error específico con información de línea
type TOMLValidationError struct {
	Field      string
	Value      interface{}
	Expected   string
	Line       int
	ConfigPath string
	Message    string
}

func (e *TOMLValidationError) Error() string {
	return fmt.Sprintf("TOML validation error in %s:%d - Field '%s': %s",
		e.ConfigPath, e.Line, e.Field, e.Message)
}

// TOMLConfigLoader maneja la carga y validación de archivos TOML con logging detallado
type TOMLConfigLoader struct {
	configPath   string
	absolutePath string
	rawContent   string
	lines        []string
}

// NewTOMLConfigLoader crea un nuevo loader con la ruta especificada
func NewTOMLConfigLoader(configPath string) (*TOMLConfigLoader, error) {
	// Obtener ruta absoluta
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for %s: %v", configPath, err)
	}

	// Leer contenido del archivo
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", absPath, err)
	}

	// Log de archivo encontrado
	log.Printf("📁 Loading TOML configuration:")
	log.Printf("   File: %s", configPath)
	log.Printf("   Absolute path: %s", absPath)

	// Obtener información del archivo
	fileInfo, err := os.Stat(absPath)
	if err == nil {
		log.Printf("   Size: %d bytes", fileInfo.Size())
		log.Printf("   Modified: %s", fileInfo.ModTime().Format("2006-01-02 15:04:05"))
	}

	return &TOMLConfigLoader{
		configPath:   configPath,
		absolutePath: absPath,
		rawContent:   string(content),
		lines:        strings.Split(string(content), "\n"),
	}, nil
}

// findFieldLine busca en qué línea está definido un campo específico
func (loader *TOMLConfigLoader) findFieldLine(fieldPath string) int {
	// Convertir dot notation a formato TOML
	// Ej: "OpsGenie.Team" -> buscar "team" en sección [opsgenie]
	parts := strings.Split(fieldPath, ".")

	if len(parts) == 1 {
		// Campo de nivel raíz
		fieldName := strings.ToLower(parts[0])
		for i, line := range loader.lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, fieldName+" =") {
				return i + 1 // Las líneas empiezan en 1
			}
		}
	} else if len(parts) == 2 {
		// Campo en sección
		sectionName := strings.ToLower(parts[0])
		fieldName := strings.ToLower(parts[1])

		inSection := false
		for i, line := range loader.lines {
			trimmed := strings.TrimSpace(line)

			// Detectar inicio de sección
			if trimmed == fmt.Sprintf("[%s]", sectionName) {
				inSection = true
				continue
			}

			// Detectar nueva sección
			if inSection && strings.HasPrefix(trimmed, "[") && trimmed != fmt.Sprintf("[%s]", sectionName) {
				inSection = false
				continue
			}

			// Buscar campo en la sección actual
			if inSection && strings.HasPrefix(trimmed, fieldName+" =") {
				return i + 1
			}
		}
	}

	return 0 // No encontrado
}

// validateAndLog valida un campo y registra warnings/errores con números de línea
func (loader *TOMLConfigLoader) validateAndLog(fieldPath string, currentValue interface{}, expectedType string, isValid bool, message string) {
	line := loader.findFieldLine(fieldPath)

	if !isValid {
		log.Printf("⚠️  WARNING in %s:%d - %s = %v: %s",
			loader.configPath, line, fieldPath, currentValue, message)
	} else {
		log.Printf("✅ %s:%d - %s = %v (valid)",
			loader.configPath, line, fieldPath, currentValue)
	}
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
	// Crear loader con logging detallado
	loader, err := NewTOMLConfigLoader(path)
	if err != nil {
		return nil, err
	}

	log.Printf("🔍 Parsing TOML configuration...")

	// Start with default config
	defaultConfig := createDefaultConfig()

	// Try to parse with the root-level structure
	var config Config
	_, err = toml.DecodeFile(loader.absolutePath, &config)

	// If we failed to load or all values are zero, try the [circuit_breaker] format
	if err != nil || (config.MemoryThreshold == 0 && config.LatencyThreshold == 0 &&
		config.LatencyWindowSize == 0 && config.Percentile == 0) {

		log.Printf("⚠️  Root-level parsing failed or incomplete, trying section-based format...")

		// Try to parse with the section-based structure
		type ConfigWithSections struct {
			CircuitBreaker Config          `toml:"circuit_breaker"`
			OpsGenie       *OpsGenieConfig `toml:"opsgenie"`
		}

		var sectionConfig ConfigWithSections
		_, sectionErr := toml.DecodeFile(loader.absolutePath, &sectionConfig)

		if sectionErr == nil {
			// Use values from the circuit_breaker section
			config = sectionConfig.CircuitBreaker

			// Preserve OpsGenie config if it was loaded in the section format
			if sectionConfig.OpsGenie != nil {
				config.OpsGenie = sectionConfig.OpsGenie
			}
			log.Printf("✅ Configuration loaded using [circuit_breaker] section format")
		} else if err != nil {
			log.Printf("❌ ERROR loading config from %s: %v. Using default values.", loader.absolutePath, err)
			return defaultConfig, err
		}
	} else {
		log.Printf("✅ Configuration loaded using root-level format")
	}

	log.Printf("🔍 Validating configuration values...")

	// Validate and set defaults for any zero or invalid values with line numbers
	if config.MemoryThreshold <= 0 || config.MemoryThreshold > 100 {
		loader.validateAndLog("memory_threshold", config.MemoryThreshold, "float64 (0-100)", false,
			fmt.Sprintf("Invalid value. Using default: %.2f", defaultConfig.MemoryThreshold))
		config.MemoryThreshold = defaultConfig.MemoryThreshold
	} else {
		loader.validateAndLog("memory_threshold", config.MemoryThreshold, "float64", true, "")
	}

	if config.LatencyThreshold <= 0 {
		loader.validateAndLog("latency_threshold", config.LatencyThreshold, "int64 (>0)", false,
			fmt.Sprintf("Invalid value. Using default: %d", defaultConfig.LatencyThreshold))
		config.LatencyThreshold = defaultConfig.LatencyThreshold
	} else {
		loader.validateAndLog("latency_threshold", config.LatencyThreshold, "int64", true, "")
	}

	if config.LatencyWindowSize <= 0 {
		loader.validateAndLog("latency_window_size", config.LatencyWindowSize, "int (>0)", false,
			fmt.Sprintf("Invalid value. Using default: %d", defaultConfig.LatencyWindowSize))
		config.LatencyWindowSize = defaultConfig.LatencyWindowSize
	} else {
		loader.validateAndLog("latency_window_size", config.LatencyWindowSize, "int", true, "")
	}

	if config.Percentile <= 0 || config.Percentile > 1 {
		loader.validateAndLog("percentile", config.Percentile, "float64 (0-1)", false,
			fmt.Sprintf("Invalid value. Using default: %.2f", defaultConfig.Percentile))
		config.Percentile = defaultConfig.Percentile
	} else {
		loader.validateAndLog("percentile", config.Percentile, "float64", true, "")
	}

	if config.WaitTime <= 0 {
		loader.validateAndLog("wait_time", config.WaitTime, "int (>0)", false,
			fmt.Sprintf("Invalid value. Using default: %d", defaultConfig.WaitTime))
		config.WaitTime = defaultConfig.WaitTime
	} else {
		loader.validateAndLog("wait_time", config.WaitTime, "int", true, "")
	}

	if config.TrendAnalysisMinSampleCount <= 0 {
		config.TrendAnalysisMinSampleCount = defaultConfig.TrendAnalysisMinSampleCount
	}

	// Initialize OpsGenie config if nil
	if config.OpsGenie == nil {
		log.Printf("⚠️  No OpsGenie configuration found in %s, using defaults", loader.configPath)
		config.OpsGenie = defaultConfig.OpsGenie
	} else {
		log.Printf("🔍 Validating OpsGenie configuration...")
		validateOpsGenieConfigWithLineNumbers(config.OpsGenie, defaultConfig.OpsGenie, loader)
	}

	log.Printf("✅ Configuration validation completed for %s", loader.absolutePath)
	logConfigSummary(&config)

	return &config, nil
}

// validateOpsGenieConfigWithLineNumbers valida la configuración de OpsGenie con números de línea
func validateOpsGenieConfigWithLineNumbers(config *OpsGenieConfig, defaults *OpsGenieConfig, loader *TOMLConfigLoader) {
	if config.Region == "" {
		loader.validateAndLog("opsgenie.region", config.Region, "string", false,
			fmt.Sprintf("Empty region. Using default: %s", defaults.Region))
		config.Region = defaults.Region
	} else {
		loader.validateAndLog("opsgenie.region", config.Region, "string", true, "")
	}

	if config.Priority == "" {
		config.Priority = defaults.Priority
	}

	if config.Source == "" {
		config.Source = defaults.Source
	}

	if len(config.Tags) == 0 {
		loader.validateAndLog("opsgenie.tags", config.Tags, "[]string", false,
			"No tags specified. Using defaults.")
		config.Tags = defaults.Tags
	} else {
		loader.validateAndLog("opsgenie.tags", len(config.Tags), "[]string", true,
			fmt.Sprintf("%d tags configured", len(config.Tags)))
	}

	// Validate team
	if config.Team == "" {
		loader.validateAndLog("opsgenie.team", config.Team, "string", false,
			"Team name is required for proper alert routing")
	} else {
		loader.validateAndLog("opsgenie.team", config.Team, "string", true, "")
	}

	// Validate mandatory field defaults
	if config.Business == "" {
		config.Business = defaults.Business
	}

	// Validate priority format
	validPriorities := map[string]bool{"P1": true, "P2": true, "P3": true, "P4": true, "P5": true}
	if !validPriorities[config.Priority] {
		loader.validateAndLog("opsgenie.priority", config.Priority, "string (P1-P5)", false,
			fmt.Sprintf("Invalid priority. Using default: %s", defaults.Priority))
		config.Priority = defaults.Priority
	} else {
		loader.validateAndLog("opsgenie.priority", config.Priority, "string", true, "")
	}

	// Validate region
	validRegions := map[string]bool{"us": true, "eu": true}
	if !validRegions[config.Region] {
		loader.validateAndLog("opsgenie.region", config.Region, "string (us|eu)", false,
			fmt.Sprintf("Invalid region. Using default: %s", defaults.Region))
		config.Region = defaults.Region
	}

	// Validate cooldown
	if config.AlertCooldownSeconds <= 0 {
		config.AlertCooldownSeconds = defaults.AlertCooldownSeconds
	}
}

// logConfigSummary muestra un resumen final de la configuración cargada
func logConfigSummary(config *Config) {
	log.Printf("📋 Configuration Summary:")
	log.Printf("   Circuit Breaker:")
	log.Printf("     - Memory threshold: %.2f%%", config.MemoryThreshold)
	log.Printf("     - Latency threshold: %dms", config.LatencyThreshold)
	log.Printf("     - Latency window size: %d", config.LatencyWindowSize)
	log.Printf("     - Percentile: %.2f", config.Percentile)
	log.Printf("     - Wait time: %ds", config.WaitTime)
	log.Printf("     - Trend analysis: %t", config.TrendAnalysisEnabled)

	if config.OpsGenie != nil {
		log.Printf("   OpsGenie:")
		log.Printf("     - Enabled: %t", config.OpsGenie.Enabled)
		if config.OpsGenie.Enabled {
			log.Printf("     - Team: %s", config.OpsGenie.Team)
			log.Printf("     - Environment: %s", config.OpsGenie.Environment)
			log.Printf("     - BookmakerID: %s", config.OpsGenie.BookmakerID)
			log.Printf("     - Business: %s", config.OpsGenie.Business)
			log.Printf("     - Tags: %v", config.OpsGenie.Tags)
		}
	}
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
		}
		summary["opsgenie"] = opsGenieSummary
	}

	return summary
}
