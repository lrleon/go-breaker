package breaker

import (
	"sync"
	"time"
)

type Breaker interface {
	Allow() bool                       // Returns if the operation can continue and updates the state of the Breaker
	Done(startTime, endTime time.Time) // Reports the latency of an operation finished
	Triggered() bool                   // Indicate if the breaker is activated
	Reset()                            // Restores the state of Breaker
}

type breaker struct {
	mu            sync.Mutex
	config        Config
	tripped       bool
	lastTripTime  time.Time
	latencyWindow *latencyWindow
}

func NewBreaker(config Config) Breaker {
	return &breaker{
		config:        config,
		latencyWindow: newLatencyWindow(config.LatencyWindowSize),
	}
}

// Return true if the memory usage is above the threshold and the latencyWindow
// is below the threshold
func (b *breaker) isHealthy() bool {
	return b.memoryOK() && b.latencyOK()
}

func (b *breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tripped {
		if time.Since(b.lastTripTime) > time.Duration(b.config.WaitTime)*time.Second {
			b.tripped = true
		}
	}
	return b.memoryOK()
}

func (b *breaker) Done(startTime, endTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.latencyWindow.add(startTime, endTime)
	latencyPercentile := b.latencyWindow.percentile(b.config.Percentile)
	memoryStatus := b.memoryOK()
	if latencyPercentile > b.config.LatencyThreshold ||
		!memoryStatus {
		b.tripped = true
		b.lastTripTime = time.Now()
	}
}

func (b *breaker) Triggered() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tripped
}

func (b *breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tripped = false
}
