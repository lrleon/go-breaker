package breaker

import (
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

func NewBreaker(config *Config) Breaker {
	lw := NewLatencyWindow(config.LatencyWindowSize)

	// Use the WaitTime value (in seconds) as the maximum age for latencies
	// This means latencies older than the circuit breaker wait time will not be considered
	if config.WaitTime > 0 {
		lw.MaxAgeSeconds = config.WaitTime
	}

	// Create a new OpsGenie client if configuration is provided
	var opsGenieClient *OpsGenieClient
	if config.OpsGenie != nil {
		opsGenieClient = NewOpsGenieClient(config.OpsGenie)
		// Initialize the client in a goroutine to avoid blocking
		if config.OpsGenie.Enabled {
			go func() {
				if err := opsGenieClient.Initialize(); err != nil {
					// Just log the error, don't fail the breaker creation
					// The circuit breaker will still work without OpsGenie
					logger := NewLogger("OpsGenie")
					logger.Logf("Failed to initialize OpsGenie: %v", err)
				}
			}()
		}
	}

	return &BreakerDriver{
		config:         *config,
		latencyWindow:  lw,
		enabled:        true,
		logger:         NewLogger("BreakerDriver"),
		opsGenieClient: opsGenieClient,
	}
}

// NewBreakerFromConfigFile creates a new Breaker instance from a TOML configuration file.
// It reads the configuration from the specified file path, and if successful,
// it initializes and returns a new Breaker with that configuration.
// If the file cannot be read or parsed, it returns an error.
func NewBreakerFromConfigFile(configPath string) (Breaker, error) {
	// Load configuration from the specified file
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	// If OpsGenie section is present but incomplete, try loading from separate OpsGenie config file
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

	// Create the breaker with the loaded configuration
	return NewBreaker(config), nil
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
		if time.Since(b.lastTripTime) > time.Duration(b.config.WaitTime)*time.Second &&
			b.MemoryOK() {
			b.triggered = false
			b.logger.BreakerReset()

			// Send OpsGenie alert for breaker reset
			if b.opsGenieClient != nil && b.config.OpsGenie != nil && b.config.OpsGenie.Enabled {
				go func() {
					if err := b.opsGenieClient.SendBreakerResetAlert(); err != nil {
						b.logger.Logf("Failed to send OpsGenie alert for breaker reset: %v", err)
					}
				}()
			}
		} else {
			return false
		}
	}

	return b.MemoryOK()
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

	// Determine whether to trigger the breaker
	shouldTrigger := false

	// If there's a memory issue, always trigger
	if !memoryStatus {
		shouldTrigger = true
	}

	// For latency issues, check if we need to consider trend analysis
	if latencyAboveThreshold {
		if b.config.TrendAnalysisEnabled {
			// Only trigger if there's a positive trend in latencies, or if latencies
			// have been consistently high for a while (plateau)
			hasTrend := b.latencyWindow.HasPositiveTrend(b.config.TrendAnalysisMinSampleCount)
			b.logger.TrendAnalysisInfo(hasTrend)

			if hasTrend {
				b.logger.Logf("Breaker triggered: Latency above threshold AND positive trend detected")
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
						b.logger.Logf("Breaker triggered: Latency plateau detected above threshold")
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
		}
	}

	if shouldTrigger {
		b.triggered = true
		b.lastTripTime = time.Now()
		b.logger.BreakerTriggered(latencyPercentile, memoryStatus, b.config.TrendAnalysisEnabled, b.config.WaitTime)

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

// LatencyOK Return true if the LatencyWindow is OK
func (b *BreakerDriver) LatencyOK() bool {
	return b.latencyWindow.BelowThreshold(b.config.LatencyThreshold)
}
