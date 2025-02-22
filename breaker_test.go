package go_breaker

import (
	"math/rand"
	"testing"
	"time"
)

func Test_breaker_should_not_trigger_if_latencies_are_below_threshold(t *testing.T) {

	memoryLimit = 512 * 1024 * 1024 // 512 MB

	b := NewBreaker(Config{
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
		latency := time.Duration(val) * time.Millisecond
		startTime := time.Now().Add(-latency)
		endTime := time.Now()
		b.Done(startTime, endTime)
	}

	if b.Triggered() {
		t.Error("Breaker should not be triggered")
	}

	if !b.Allow() {
		t.Error("Breaker should not allow")
	}
}

func Test_breaker_should_trigger_if_latencies_are_above_threshold(t *testing.T) {

	memoryLimit = 512 * 1024 * 1024 // 512 MB

	b := NewBreaker(Config{
		MemoryThreshold:   0.85,
		LatencyThreshold:  600,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          10,
	})

	// Add 100 latencies above the threshold and verify the breaker is triggered
	for i := 0; i < 100; i++ {
		// random latency between 700 and 1000
		val := rand.Int()%300 + 700
		latency := time.Duration(val) * time.Millisecond
		startTime := time.Now().Add(-latency)
		endTime := time.Now()
		b.Done(startTime, endTime)
	}

	if !b.Triggered() {
		t.Error("Breaker should be triggered")
	}

	if !b.Allow() {
		t.Error("Breaker should allow")
	}

}
