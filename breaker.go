package breaker

import (
	"sync"
	"time"
)

type Breaker interface {
	Allow() bool                       // Returns if the operation can continue
	Done(startTime, endTime time.Time) // Reports the latency of an operation finished
	Triggered() bool                   // Indicate if the breaker is activated
	Reset()                            // Restores the state of Breaker
}

type breaker struct {
	mu           sync.Mutex
	config       Config
	tripped      bool
	lastTripTime time.Time
	latency      *latencyWindow
}

func NewBreaker(config Config) Breaker {
	return &breaker{
		config:  config,
		latency: newLatencyWindow(config.LatencyWindowSize),
	}
}

// Return true if the memory usage is above the threshold and the latency
// is below the threshold
func (b *breaker) isHealthy() bool {
	return b.memoryOK() && b.latencyOK()
}

func (b *breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tripped {
		return false
	}
	return !b.memoryOK()
}

func (b *breaker) Done(startTime, endTime time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.latency.add(startTime, endTime)
	if b.latency.percentile(b.config.Percentile) > b.config.LatencyThreshold || b.memoryOK() {
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
