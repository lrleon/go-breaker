package tests

import (
	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
	"time"
)

func Test_breaker_should_not_trigger_if_latencies_are_below_threshold(t *testing.T) {

	breaker.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	b := breaker.NewBreaker(&breaker.Config{
		MemoryThreshold:   0.85,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10,
	})

	// Add 100 latencies below the threshold and verify the breaker is not triggered
	for i := 0; i < 100; i++ {
		// random latency between 100 and 500
		val := rand.Int()%400 + 100

		if val > 500 {
			t.Error("Latency should be below 500")
		}

		latency := time.Duration(val) * time.Millisecond
		now := time.Now()
		startTime := now.Add(-latency)
		endTime := now
		b.Done(startTime, endTime)
	}

	assert.False(t, b.TriggeredByLatencies(), "Breaker should not be triggered")

	assert.True(t, b.Allow(), "Breaker should allow")
}

func Test_breaker_should_trigger_if_latencies_are_above_threshold(t *testing.T) {

	breaker.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	b := breaker.NewBreaker(&breaker.Config{
		MemoryThreshold:   0.85,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10, // 10 seconds
	})

	// Add 10 latencies so the 95th percentile is above the threshold
	for i := 0; i < 10; i++ {
		val := 300 + i*50
		latency := time.Duration(val) * time.Millisecond
		now := time.Now()
		startTime := now.Add(-latency)
		endTime := now
		b.Done(startTime, endTime)
	}

	assert.True(t, b.TriggeredByLatencies(), "Breaker should be triggered")

	assert.False(t, b.Allow(), "Breaker should not allow")
}

func Test_breaker_should_trigger_if_memory_is_above_threshold(t *testing.T) {

	breaker.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	b := breaker.NewBreaker(&breaker.Config{
		MemoryThreshold:   0.5,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10,
	})

	// Add 10 latencies under the threshold and verify the breaker is triggered
	for i := 0; i < 10; i++ {
		// random latency between 100 and 500
		val := rand.Int()%400 + 100
		latency := time.Duration(val) * time.Millisecond
		startTime := time.Now().Add(-latency)
		endTime := time.Now()
		b.Done(startTime, endTime)
	}

	assert.False(t, b.TriggeredByLatencies(), "Breaker should not be triggered due to latencies")

	assert.True(t, b.Allow(), "Breaker should allow because of latencies are below threshold")

	// now we reduce MemoryLimit to trigger the breaker
	breaker.MemoryLimit = 1 * 1024 * 1024 // 256 MB

	assert.False(t, b.TriggeredByLatencies(), "Breaker should not be triggered due to memory usage")

	assert.False(t, b.Allow(), "Breaker should not allow because of memory usage")

	// now we increase MemoryLimit to allow the breaker
	breaker.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	assert.True(t, b.Allow(), "Breaker should allow because of memory usage")
}
