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
	}, "test_breakers.toml")

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
	}, "test_breakers.toml")

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
	}, "test_breakers.toml")

	// Reset any previous memory override that might be left over from other tests
	breaker.SetMemoryOK(nil, true)

	// Add 10 latencies under the threshold and verify the breaker is not triggered
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

	// Force memory check to fail
	breaker.SetMemoryOK(b.(*breaker.BreakerDriver), false)

	assert.False(t, b.TriggeredByLatencies(), "Breaker should not be triggered due to memory usage")

	assert.False(t, b.Allow(), "Breaker should not allow because of memory usage")

	// Force memory check to pass again
	breaker.SetMemoryOK(b.(*breaker.BreakerDriver), true)

	assert.True(t, b.Allow(), "Breaker should allow because of memory usage")
}

func Test_breaker_should_trigger_if_memory_is_above_threshold_and_latencies_are_above_threshold(t *testing.T) {

	breaker.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	// Reset any previous memory override
	breaker.SetMemoryOK(nil, true)

	b := breaker.NewBreaker(&breaker.Config{
		MemoryThreshold:   0.5,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10,
	}, "test_breakers.toml")

	// Add 10 latencies above the threshold and verify the breaker is triggered
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

	// Reset and force memory check to fail
	b.Reset()
	breaker.SetMemoryOK(b.(*breaker.BreakerDriver), false)

	assert.False(t, b.TriggeredByLatencies(), "Breaker should not be triggered due to memory usage")

	assert.False(t, b.Allow(), "Breaker should not allow because of memory usage")

	// Force memory check to pass
	breaker.SetMemoryOK(b.(*breaker.BreakerDriver), true)

	assert.True(t, b.Allow(), "Breaker should allow")

	// Add 10 latencies under the threshold and verify the breaker is not triggered
	for i := 0; i < 10; i++ {
		// random latency between 100 and 500
		val := rand.Int()%400 + 100
		latency := time.Duration(val) * time.Millisecond
		startTime := time.Now().Add(-latency)
		endTime := time.Now()
		b.Done(startTime, endTime)
	}

	assert.False(t, b.TriggeredByLatencies(), "Breaker should not be triggered")

	assert.True(t, b.Allow(), "Breaker should allow")
}

func Test_Breaker_Enable_Disable(t *testing.T) {
	b := breaker.NewBreaker(&breaker.Config{
		MemoryThreshold:   0.5,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10,
	}, "test_breakers.toml")

	breaker.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	assert.True(t, b.Allow(), "Breaker should allow")
	assert.True(t, b.IsEnabled(), "Breaker should be enabled")

	// put some latencies below the threshold
	for i := 0; i < 10; i++ {
		// random latency between 100 and 500
		val := rand.Int()%400 + 100
		latency := time.Duration(val) * time.Millisecond
		startTime := time.Now().Add(-latency)
		endTime := time.Now()
		b.Done(startTime, endTime)
	}

	assert.True(t, b.Allow(), "Breaker should allow")
	assert.True(t, b.IsEnabled(), "Breaker should be enabled")

	b.Disable()
	assert.True(t, b.Allow(), "Breaker should allow when disabled")
	assert.False(t, b.IsEnabled(), "Breaker should be disabled")

	b.Enable()
	assert.True(t, b.Allow(), "Breaker should allow when enabled")
	assert.True(t, b.IsEnabled(), "Breaker should be enabled")

	// put some latencies above the threshold
	for i := 0; i < 10; i++ {
		val := 300 + i*50
		latency := time.Duration(val) * time.Millisecond
		now := time.Now()
		startTime := now.Add(-latency)
		endTime := now
		b.Done(startTime, endTime)
	}

	assert.False(t, b.Allow(), "Breaker should not allow when triggered")
	assert.True(t, b.IsEnabled(), "Breaker should be enabled")
}
