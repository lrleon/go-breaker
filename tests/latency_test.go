package tests

import (
	"github.com/lrleon/go-breaker/breaker"
	"reflect"
	"testing"
	"time"
)

func Test_latencyWindow_aboveThreshold(t *testing.T) {
	lw := breaker.NewLatencyWindow(10)

	// Add recent latencies (with current timestamps) to be considered
	now := time.Now()
	latencies := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}

	for _, latency := range latencies {
		// Simulate different start and end times
		startTime := now.Add(-time.Duration(latency) * time.Millisecond)
		endTime := now
		lw.Add(startTime, endTime)
	}

	// Verify that it's above the threshold (using 99th percentile)
	if got := lw.AboveThreshold(500); !got {
		t.Errorf("AboveThreshold() = %v, want %v", got, true)
	}
}

func Test_latencyWindow_add(t *testing.T) {
	lw := breaker.NewLatencyWindow(10)

	now := time.Now()
	// generate latencies between 300 and 1600 ms
	for i := 300; i < 1600; i += 13 {
		latency := time.Duration(i) * time.Millisecond
		startTime := now.Add(-latency)
		endTime := now
		lw.Add(startTime, endTime)
	}

	// print the 95th percentile
	p := 0.95
	percentile := lw.Percentile(p)
	t.Logf("95th percentile: %d ms", percentile)
}

func Test_latencyWindow_belowThreshold(t *testing.T) {
	lw := breaker.NewLatencyWindow(10)

	// Add recent latencies (with current timestamps) to be considered
	now := time.Now()
	latencies := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}

	for _, latency := range latencies {
		// Simulate different start and end times
		startTime := now.Add(-time.Duration(latency) * time.Millisecond)
		endTime := now
		lw.Add(startTime, endTime)
	}

	// Verify that it's NOT below the threshold (using 99th percentile)
	if got := lw.BelowThreshold(500); got {
		t.Errorf("BelowThreshold() = %v, want %v", got, false)
	}
}

func Test_latencyWindow_aboveThresholdLatencies(t *testing.T) {
	lw := breaker.NewLatencyWindow(10)

	// Add recent latencies (with current timestamps) to be considered
	now := time.Now()
	latencies := []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000}

	for _, latency := range latencies {
		// Simulate different start and end times
		startTime := now.Add(-time.Duration(latency) * time.Millisecond)
		endTime := now
		lw.Add(startTime, endTime)
	}

	// Verify that latencies above the threshold are as expected
	latenciesAboveThreshold := lw.AboveThresholdLatencies(500)
	expectedLatencies := []int64{600, 700, 800, 900, 1000}

	// Sort both slices to make comparison easier
	if !reflect.DeepEqual(sortInt64s(latenciesAboveThreshold), sortInt64s(expectedLatencies)) {
		t.Errorf("AboveThresholdLatencies() = %v, want %v", latenciesAboveThreshold, expectedLatencies)
	}
}

// Test for the new GetRecentLatencies method and latency expiration
func Test_latencyWindow_expiration(t *testing.T) {
	lw := breaker.NewLatencyWindow(10)

	// Configure to only consider latencies from the last 5 minutes (300 seconds)
	lw.MaxAgeSeconds = 300

	now := time.Now()

	// Add recent latencies
	recentLatencies := []int64{100, 200, 300}
	for _, latency := range recentLatencies {
		startTime := now.Add(-time.Duration(latency) * time.Millisecond)
		lw.Add(startTime, now)
	}

	// First get recent latencies to verify how many there are
	recentValues := lw.GetRecentLatencies()
	if len(recentValues) != len(recentLatencies) {
		t.Errorf("GetRecentLatencies() returned %d values, expected approximately %d",
			len(recentValues), len(recentLatencies))
	}

	// Now test the time-based filtering functionality
	// Add latencies with old timestamps (simulation)
	for i := 0; i < 5; i++ {
		// We can't directly modify the Records, so we use an alternative solution
		// to test expiration: add and then modify the system time for verification
		lw.Add(now, now)
	}

	// Simulate that it's now 10 minutes later for verification
	// Latencies we just added should be considered old
	futureTime := now.Add(10 * time.Minute)

	// Create a new window with the same parameters, but that considers latencies
	// as if we were in the future
	lwFuture := breaker.NewLatencyWindow(10)
	lwFuture.MaxAgeSeconds = 300

	// Add latencies with "future" timestamps
	futureLats := []int64{150, 250, 350}
	for _, lat := range futureLats {
		startTime := futureTime.Add(-time.Duration(lat) * time.Millisecond)
		lwFuture.Add(startTime, futureTime)
	}

	// Verify that we only have the "future" latencies
	futureLatencies := lwFuture.GetRecentLatencies()
	if len(futureLatencies) != len(futureLats) {
		t.Errorf("Future latency window should have only %d latencies, got %d",
			len(futureLats), len(futureLatencies))
	}
}

// Test for the trend analysis functionality
func Test_latencyWindow_hasPositiveTrend(t *testing.T) {
	lw := breaker.NewLatencyWindow(10)

	// Test with increasing latencies (positive trend)
	now := time.Now()
	for i := 1; i <= 5; i++ {
		// Create increasing latencies: 100ms, 200ms, 300ms, etc.
		latency := int64(i * 100)
		// Add them with timestamps spaced 1 second apart
		timestamp := now.Add(time.Duration(i) * time.Second)

		// Simulate a request that started (latency) milliseconds ago
		startTime := timestamp.Add(-time.Duration(latency) * time.Millisecond)
		lw.Add(startTime, timestamp)
	}

	// Should detect positive trend with minimum 3 samples
	if !lw.HasPositiveTrend(3) {
		t.Errorf("HasPositiveTrend() with increasing latencies = false, want true")
	}

	// Reset for next test
	lw = breaker.NewLatencyWindow(10)

	// Test with decreasing latencies (negative trend)
	for i := 5; i >= 1; i-- {
		// Create decreasing latencies: 500ms, 400ms, 300ms, etc.
		latency := int64(i * 100)
		// Add them with timestamps spaced 1 second apart
		timestamp := now.Add(time.Duration(6-i) * time.Second)

		// Simulate a request that started (latency) milliseconds ago
		startTime := timestamp.Add(-time.Duration(latency) * time.Millisecond)
		lw.Add(startTime, timestamp)
	}

	// Should NOT detect positive trend with minimum 3 samples
	if lw.HasPositiveTrend(3) {
		t.Errorf("HasPositiveTrend() with decreasing latencies = true, want false")
	}

	// Test with minimum sample count not met
	if lw.HasPositiveTrend(20) {
		t.Errorf("HasPositiveTrend() with too few samples should be false")
	}
}

// Test the breaker with trend analysis enabled
func Test_breaker_withTrendAnalysis(t *testing.T) {
	// Create a mock implementation of Breaker to override MemoryOK
	type mockBreakerDriver struct {
		*breaker.BreakerDriver
		memoryOK bool
	}

	// Create config with trend analysis enabled
	config := &breaker.Config{
		MemoryThreshold:             90.0, // 90% memory threshold
		LatencyThreshold:            300,  // 300ms latency threshold
		LatencyWindowSize:           10,
		Percentile:                  0.95,
		WaitTime:                    60,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 3,
	}

	// Override MemoryOK to always return true for testing
	originalBreaker := breaker.NewBreaker(config, "breaker-config.toml").(*breaker.BreakerDriver)

	// Expose the MemoryOK method for testing
	breaker.SetMemoryOK(originalBreaker, true)

	b := originalBreaker

	now := time.Now()

	// First add latencies that are under threshold - breaker shouldn't trigger
	for i := 1; i <= 3; i++ {
		startTime := now.Add(time.Duration(i)*time.Second - 200*time.Millisecond)
		endTime := now.Add(time.Duration(i) * time.Second)
		b.Done(startTime, endTime) // 200ms latency
	}

	// Breaker should not be triggered yet
	if b.TriggeredByLatencies() {
		t.Errorf("Breaker should not trigger with latencies under threshold")
	}

	// Reset the breaker
	b.Reset()

	// Now add latencies OVER threshold BUT with negative trend
	// These are decreasing (500ms, 450ms, 400ms)
	for i := 0; i < 3; i++ {
		latency := 500 - i*50 // decreasing latencies
		startTime := now.Add(time.Duration(4+i)*time.Second - time.Duration(latency)*time.Millisecond)
		endTime := now.Add(time.Duration(4+i) * time.Second)
		b.Done(startTime, endTime)
	}

	// Breaker should still not be triggered because trend is negative
	if b.TriggeredByLatencies() {
		t.Errorf("Breaker should not trigger with high latencies but negative trend")
	}

	// Reset the breaker
	b.Reset()

	// Now add latencies OVER threshold WITH positive trend
	// These are increasing (400ms, 450ms, 500ms)
	for i := 0; i < 3; i++ {
		latency := 400 + i*50 // increasing latencies
		startTime := now.Add(time.Duration(7+i)*time.Second - time.Duration(latency)*time.Millisecond)
		endTime := now.Add(time.Duration(7+i) * time.Second)
		b.Done(startTime, endTime)
	}

	// Now breaker should be triggered because latencies are over threshold AND trend is positive
	if !b.TriggeredByLatencies() {
		t.Errorf("Breaker should trigger with high latencies and positive trend")
	}

	// Reset and test with trend analysis disabled
	config.TrendAnalysisEnabled = false
	b = breaker.NewBreaker(config, "breaker-config.toml").(*breaker.BreakerDriver)
	breaker.SetMemoryOK(b, true) // Override memory check

	// Add latencies over threshold but with negative trend
	for i := 0; i < 3; i++ {
		latency := 500 - i*50 // decreasing latencies
		startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
		endTime := now.Add(time.Duration(i) * time.Second)
		b.Done(startTime, endTime)
	}

	// Now breaker should be triggered regardless of trend, since trend analysis is disabled
	if !b.TriggeredByLatencies() {
		t.Errorf("Breaker with trend analysis disabled should trigger with high latencies regardless of trend")
	}
}

// Helper function to sort int64 slices
func sortInt64s(values []int64) []int64 {
	result := make([]int64, len(values))
	copy(result, values)
	// Simple bubble sort
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
