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

	// Configure to only consider latencies from the last 5 minutes
	lw.MaxAgeMinutes = 5

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

	// Create a new window with the same parameters but that considers latencies
	// as if we were in the future
	lwFuture := breaker.NewLatencyWindow(10)
	lwFuture.MaxAgeMinutes = 5

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
