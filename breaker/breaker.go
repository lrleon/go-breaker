package breaker

import (
	"log"
	"sync"
	"time"
)

type Breaker interface {
	Allow() bool                       // Returns if the operation can continue and updates the state of the Breaker
	Done(startTime, endTime time.Time) // Reports the latency of an operation finished
	TriggeredByLatencies() bool        // Indicate if the BreakerDriver is activated
	Reset()                            // Restores the state of Breaker
	LatenciesAboveThreshold(threshold int64) []int64
}

type BreakerDriver struct {
	mu            sync.Mutex
	config        Config
	triggered     bool
	lastTripTime  time.Time
	latencyWindow *LatencyWindow
}

func NewBreaker(config *Config) Breaker {
	return &BreakerDriver{
		config:        *config,
		latencyWindow: NewLatencyWindow(config.LatencyWindowSize),
	}
}

// Return true if the memory usage is above the threshold and the LatencyWindow
// is below the threshold
func (b *BreakerDriver) isHealthy() bool {
	return b.MemoryOK() && b.LatencyOK()
}

func (b *BreakerDriver) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.triggered {
		if time.Since(b.lastTripTime) > time.Duration(b.config.WaitTime)*time.Second &&
			b.MemoryOK() {
			b.triggered = false
			log.Printf("BreakerDriver has been reset")
		} else {
			return false
		}
	}
	return b.MemoryOK()
}

func (b *BreakerDriver) Done(startTime, endTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.latencyWindow.Add(startTime, endTime)
	latencyPercentile := b.latencyWindow.Percentile(b.config.Percentile)
	memoryStatus := b.MemoryOK()
	if latencyPercentile > b.config.LatencyThreshold || !memoryStatus {
		b.triggered = true
		b.lastTripTime = time.Now()
		log.Printf("BreakerDriver triggered. Latency: %v, Memory: %v",
			latencyPercentile, memoryStatus)
		log.Printf("Retry after %v seconds", b.config.WaitTime)
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
	b.triggered = false
}
