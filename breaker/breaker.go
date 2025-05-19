package breaker

import (
	"runtime"
	"sync"
	"time"
)

type Breaker interface {
	Allow() bool                       // Returns if the operation can continue and updates the state of the Breaker
	Done(startTime, endTime time.Time) // Reports the latency of an operation finished
	TriggeredByLatencies() bool        // Indicate if the BreakerDriver is activated
	Reset()                            // Restores the state of Breaker
	LatenciesAboveThreshold(threshold int64) []int64
	MemoryOK() bool
	LatencyOK() bool
	IsEnabled() bool
	Disable()
	Enable()
	GetConfigFile() string
}

type BreakerDriver struct {
	mu             sync.Mutex
	config         Config
	triggered      bool
	lastTripTime   time.Time
	latencyWindow  *LatencyWindow
	enabled        bool
	logger         *Logger
	opsGenieClient *OpsGenieClient // OpsGenie client for sending alerts
	configFile     string          // Path to the config file that was used to create this breaker
}

func (b *BreakerDriver) IsEnabled() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.enabled
}

func (b *BreakerDriver) Disable() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.enabled = false
}

func (b *BreakerDriver) Enable() {
	b.Reset()
}

func NewBreaker(config *Config, configFile string) Breaker {

	lw := NewLatencyWindow(config.LatencyWindowSize)

	// Use the WaitTime value (in seconds) as the maximum age for latencies
	// This means latencies older than the circuit breaker wait time will not be considered
	if config.WaitTime > 0 {
		lw.MaxAgeSeconds = config.WaitTime
	}

	// Create a new OpsGenie client if configuration is provided
	var opsGenieClient *OpsGenieClient
	if config.OpsGenie != nil {
		// Use the singleton pattern to get the OpsGenie client
		// This ensures that only one instance of the OpsGenie client is created
		opsGenieClient = GetOpsGenieClient(config.OpsGenie)
	}

	logger := NewLogger("BreakerDriver")

	// Check memory status at initialization
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// Default values in case of not having a valid memory limit
	currentMemMB := int64(memStats.Alloc) / 1024 / 1024
	var memThresholdMB int64 = 0
	var memoryOK = true

	if MemoryLimit <= 0 {
		logger.Logf("Breaker initialized - Warning: Cannot determine memory limit. Memory checks will be skipped.")
	} else {
		// To avoid loss of precision, we make the division before multiplication
		thresholdFraction := config.MemoryThreshold / 100.0
		memThresholdBytes := float64(MemoryLimit) * thresholdFraction
		memThresholdMB = int64(memThresholdBytes / 1024 / 1024)

		memoryOK = float64(memStats.Alloc) < memThresholdBytes

		logger.Logf("Breaker initialized - Memory status: Current: %dMB, Threshold: %dMB (%.2f%% of %dMB), Memory OK: %v",
			currentMemMB,
			memThresholdMB,
			config.MemoryThreshold,
			MemoryLimit/1024/1024,
			memoryOK)

		// Additional letter if the memory is close to the limit
		if float64(memStats.Alloc) > (memThresholdBytes*0.9) && float64(memStats.Alloc) < memThresholdBytes {
			logger.Logf("WARNING: Memory usage is approaching threshold (>90%% of limit)")
		}
	}

	return &BreakerDriver{
		config:         *config,
		latencyWindow:  lw,
		enabled:        true,
		logger:         logger,
		opsGenieClient: opsGenieClient,
		configFile:     configFile,
	}
}

// NewBreakerFromConfigFile creates a new Breaker instance from a TOML configuration file.
// This function takes a path to a TOML file, attempts to load the configuration from it, and if successful,
// it initializes and returns a new Breaker with that configuration.
// If the file cannot be read or parsed, it returns an error.
func NewBreakerFromConfigFile(configPath string) (Breaker, error) {
	// Load configuration from the specified file
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// If the OpsGenie section is present but incomplete, try loading from a separate OpsGenie config file
	if config.OpsGenie != nil && config.OpsGenie.Enabled && config.OpsGenie.APIKey == "" {
		// Try to load from the default OpsGenie config path
		opsGenieConfig, opsGenieErr := LoadOpsGenieConfig(GetOpsGenieConfigPath())
		if opsGenieErr == nil {
			config.OpsGenie = opsGenieConfig
		} else {
			// Log warning but continue - OpsGenie features just won't work
			logger := NewLogger("BreakerDriver")
			logger.Logf("Warning: OpsGenie config enabled but failed to load: %v", opsGenieErr)
		}
	}

	// Create the breaker with the loaded configuration and remember the config file path
	return NewBreaker(config, configPath), nil
}

// Return true if the memory usage is above the threshold and the LatencyWindow
// is below the threshold
func (b *BreakerDriver) isHealthy() bool {
	return b.MemoryOK() && b.LatencyOK()
}

func (b *BreakerDriver) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.enabled {
		return true
	}

	if b.triggered {
		timeWaiting := time.Since(b.lastTripTime)
		waitDuration := time.Duration(b.config.WaitTime) * time.Second
		memoryStatus := b.MemoryOK()

		b.logger.Logf("Breaker Allow check: triggered=%v, time since trip=%v, wait time=%v, memory ok=%v",
			b.triggered, timeWaiting, waitDuration, memoryStatus)

		if timeWaiting > waitDuration && memoryStatus {
			b.triggered = false
			b.logger.BreakerReset()
			b.logger.Logf("INFO: Breaker automatically reset after waiting %v (required %v) and memory status OK",
				timeWaiting, waitDuration)

			// Send OpsGenie alert for breaker reset
			if b.opsGenieClient != nil && b.config.OpsGenie != nil && b.config.OpsGenie.Enabled {
				go func() {
					if err := b.opsGenieClient.SendBreakerResetAlert(); err != nil {
						b.logger.Logf("Failed to send OpsGenie alert for breaker reset: %v", err)
					}
				}()
			}
		} else {
			if !memoryStatus {
				b.logger.Logf("DENY: Request denied because memory is still above threshold")
			} else {
				b.logger.Logf("DENY: Request denied because wait time (%v) has not elapsed yet (%v passed)",
					waitDuration, timeWaiting)
			}
			return false
		}
	}

	memoryOk := b.MemoryOK()
	if !memoryOk {
		b.logger.Logf("DENY: Request denied due to memory threshold exceeded")
	}
	return memoryOk
}

func (b *BreakerDriver) Done(startTime, endTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.enabled {
		return
	}

	b.latencyWindow.Add(startTime, endTime)
	latencyPercentile := b.latencyWindow.Percentile(b.config.Percentile)
	memoryStatus := b.MemoryOK()

	// Check if latency is above the threshold
	latencyAboveThreshold := latencyPercentile > b.config.LatencyThreshold

	// Logging for debugging
	b.logger.LatencyInfo(latencyPercentile, b.config.LatencyThreshold, latencyAboveThreshold)
	b.logger.Logf("Status check: memory_ok=%v, latency_percentile=%dms, threshold=%dms, above_threshold=%v",
		memoryStatus, latencyPercentile, b.config.LatencyThreshold, latencyAboveThreshold)

	// Add explicit log when latency exceeds the threshold
	if latencyAboveThreshold {
		b.logger.Logf("ALERT: Latency %dms exceeds threshold of %dms", latencyPercentile, b.config.LatencyThreshold)
	}

	// Add explicit log when memory has issues
	if !memoryStatus {
		memStats := new(runtime.MemStats)
		runtime.ReadMemStats(memStats)
		memLimit := float64(MemoryLimit) * (b.config.MemoryThreshold / 100.0)
		b.logger.Logf("ALERT: Memory threshold exceeded - Current: %dMB, Limit: %.2fMB (%.2f%% of %dMB)",
			memStats.Alloc/1024/1024, memLimit/1024/1024, b.config.MemoryThreshold, MemoryLimit/1024/1024)
	}

	// Determine whether to trigger the breaker
	shouldTrigger := false

	// If there's a memory issue, always trigger
	if !memoryStatus {
		shouldTrigger = true
		b.logger.Logf("TRIGGER REASON: Memory threshold exceeded")
	}

	// For latency issues, check if we need to consider trend analysis
	if latencyAboveThreshold {
		if b.config.TrendAnalysisEnabled {
			// Only trigger if there's a positive trend in latencies, or if latencies
			// have been consistently high for a while (plateau)
			hasTrend := b.latencyWindow.HasPositiveTrend(b.config.TrendAnalysisMinSampleCount)
			b.logger.TrendAnalysisInfo(hasTrend)

			if hasTrend {
				b.logger.Logf("TRIGGER REASON: Latency above threshold AND positive trend detected")
				shouldTrigger = true
			} else {
				// Check for a plateau - latencies consistently above threshold
				// We consider it a plateau if we have at least 5 samples, and they're all above threshold
				latencies := b.latencyWindow.GetRecentLatencies()

				// If we have enough samples, and they're all high, consider it a plateau
				if len(latencies) >= 5 {
					allAboveThreshold := true
					for _, lat := range latencies {
						if lat <= b.config.LatencyThreshold {
							allAboveThreshold = false
							break
						}
					}

					if allAboveThreshold {
						b.logger.Logf("TRIGGER REASON: Latency plateau detected above threshold")
						shouldTrigger = true
					} else {
						b.logger.Logf("Latency above threshold but NO positive trend or plateau. Not triggering breaker.")
					}
				} else {
					b.logger.Logf("Latency above threshold but NO positive trend. Not triggering breaker.")
				}
			}
		} else {
			// No trend analysis, trigger based on a threshold only
			shouldTrigger = true
			b.logger.Logf("TRIGGER REASON: Latency above threshold (trend analysis disabled)")
		}
	}

	if shouldTrigger {
		b.triggered = true
		b.lastTripTime = time.Now()
		b.logger.BreakerTriggered(latencyPercentile, memoryStatus, b.config.TrendAnalysisEnabled, b.config.WaitTime)

		// Log the breaker triggered event with more details
		triggerReason := "latency and/or memory issues"
		if !memoryStatus && latencyAboveThreshold {
			triggerReason = "both latency and memory issues"
		} else if !memoryStatus {
			triggerReason = "memory issues"
		} else if latencyAboveThreshold {
			triggerReason = "latency issues"
		}
		b.logger.Logf("ACTION: Circuit breaker TRIGGERED due to %s. Waiting %d seconds before reset attempt",
			triggerReason, b.config.WaitTime)

		// Send OpsGenie alert for breaker triggered
		if b.opsGenieClient != nil && b.config.OpsGenie != nil && b.config.OpsGenie.Enabled {
			go func() {
				if err := b.opsGenieClient.SendBreakerOpenAlert(latencyPercentile, memoryStatus, b.config.WaitTime); err != nil {
					b.logger.Logf("Failed to send OpsGenie alert for breaker open: %v", err)
				}
			}()
		}
	}
}

// TriggeredByLatencies returns a boolean indicating if the BreakerDriver is currently triggered.
// The BreakerDriver is triggered when both the memory usage is above the threshold
// and the latency percentile is above the latency threshold.
func (b *BreakerDriver) TriggeredByLatencies() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.triggered
}

// LatenciesAboveThreshold Return latencies above the threshold
func (b *BreakerDriver) LatenciesAboveThreshold(threshold int64) []int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.latencyWindow.AboveThresholdLatencies(threshold)
}

func (b *BreakerDriver) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Only send reset alert if previously triggered
	wasTriggered := b.triggered

	b.triggered = false
	b.lastTripTime = time.Time{}
	b.enabled = true
	b.latencyWindow.Reset()

	// If the breaker was previously triggered, send a reset alert
	if wasTriggered && b.opsGenieClient != nil && b.config.OpsGenie != nil && b.config.OpsGenie.Enabled {
		go func() {
			if err := b.opsGenieClient.SendBreakerResetAlert(); err != nil {
				b.logger.Logf("Failed to send OpsGenie alert for manual breaker reset: %v", err)
			}
		}()
	}
}

// LatencyOK reports whether the current latency percentile is below the configured threshold
func (b *BreakerDriver) LatencyOK() bool {
	return b.latencyWindow.BelowThreshold(b.config.LatencyThreshold)
}

// GetConfigFile returns the configuration file path used to create this breaker
func (b *BreakerDriver) GetConfigFile() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Default to the standard config path if not set
	if b.configFile == "" {
		return "breakers.toml"
	}
	return b.configFile
}
