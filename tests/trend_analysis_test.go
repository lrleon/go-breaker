package tests

import (
	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// TestBreakerTriggersWithIncreasingTrend verifies that the breaker triggers when
// an upward trend in latencies above the threshold is detected
func TestBreakerTriggersWithIncreasingTrend(t *testing.T) {
	// Configuration with trend analysis enabled
	config := &breaker.Config{
		MemoryThreshold:             90.0, // 90% memory threshold
		LatencyThreshold:            300,  // 300ms latency threshold
		LatencyWindowSize:           20,   // Larger latency window for more samples
		Percentile:                  0.95,
		WaitTime:                    60,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 5, // Requires more samples for analysis
	}

	b := breaker.NewBreaker(config)

	// Ensure memory checks don't interfere
	breaker.SetMemoryOK(b.(*breaker.BreakerDriver), true)

	now := time.Now()

	// Case 1: Latencies below threshold - breaker should not trigger
	t.Run("LatenciesBelowThreshold", func(t *testing.T) {
		b.Reset()

		// Add constant latencies below threshold
		for i := 0; i < 10; i++ {
			latency := 200 // 200ms - below the 300ms threshold
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		assert.False(t, b.TriggeredByLatencies(), "The breaker should not trigger with latencies below threshold")
	})

	// Case 2: Latencies above threshold but without clear trend - should now trigger
	// because they're consistently above threshold (plateau behavior)
	t.Run("HighLatenciesNoTrend", func(t *testing.T) {
		b.Reset()

		// Add alternating latencies but all above threshold
		// Modify the pattern to be clearly not increasing
		alternatingLatencies := []int{400, 400, 400, 380, 390, 385, 395, 390, 385, 395}
		for i, latency := range alternatingLatencies {
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		assert.True(t, b.TriggeredByLatencies(), "The breaker should trigger with high latencies consistently above threshold, even without a clear upward trend")
	})

	// Case 3: Latencies above threshold with downward trend - should still trigger
	// because they're consistently above threshold (plateau behavior)
	t.Run("HighLatenciesWithDownwardTrend", func(t *testing.T) {
		b.Reset()

		// Descending latencies but all above threshold
		for i := 0; i < 10; i++ {
			latency := 500 - i*20 // From 500ms down to 320ms
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		assert.True(t, b.TriggeredByLatencies(), "The breaker should trigger with high latencies consistently above threshold, even with a downward trend")
	})

	// Case 4: Latencies above threshold with clear upward trend - SHOULD trigger
	t.Run("HighLatenciesWithUpwardTrend", func(t *testing.T) {
		b.Reset()

		// Ascending latencies and all above threshold
		for i := 0; i < 10; i++ {
			latency := 320 + i*20 // From 320ms up to 500ms
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		assert.True(t, b.TriggeredByLatencies(), "The breaker SHOULD trigger with latencies above threshold and upward trend")
	})

	// Case 5: Latencies initially below threshold but increasing above it - SHOULD trigger
	t.Run("ThresholdCrossingWithUpwardTrend", func(t *testing.T) {
		b.Reset()

		// First latencies below threshold
		for i := 0; i < 5; i++ {
			latency := 200 + i*10 // From 200ms to 240ms
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		// Then latencies crossing threshold and continuing to increase
		for i := 5; i < 15; i++ {
			latency := 240 + (i-4)*20 // From 260ms to 460ms
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		assert.True(t, b.TriggeredByLatencies(), "The breaker SHOULD trigger when latencies cross the threshold with upward trend")
	})

	// Case 6: Stepped increase pattern - SHOULD trigger
	t.Run("AscendingStepPattern", func(t *testing.T) {
		b.Reset()

		// Pattern: plateau, increase, plateau, increase
		latencies := []int{
			310, 310, 310, // Plateau 1
			350, 350, 350, // Plateau 2 (increase)
			390, 390, 390, // Plateau 3 (increase)
			430, 430, 430, // Plateau 4 (increase)
		}

		for i, latency := range latencies {
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			b.Done(startTime, endTime)
		}

		assert.True(t, b.TriggeredByLatencies(), "The breaker SHOULD trigger with an ascending step pattern")
	})

	// Case 7: Verify that without trend analysis, any latency above threshold triggers the breaker
	t.Run("WithoutTrendAnalysis", func(t *testing.T) {
		// Create new breaker with trend analysis disabled
		configNoTrend := *config
		configNoTrend.TrendAnalysisEnabled = false
		bNoTrend := breaker.NewBreaker(&configNoTrend)
		breaker.SetMemoryOK(bNoTrend.(*breaker.BreakerDriver), true)

		// Add latencies above threshold but with downward trend
		for i := 0; i < 5; i++ {
			latency := 500 - i*20 // Descending
			startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
			endTime := now.Add(time.Duration(i) * time.Second)
			bNoTrend.Done(startTime, endTime)
		}

		assert.True(t, bNoTrend.TriggeredByLatencies(), "Without trend analysis, the breaker SHOULD trigger with latencies above threshold only")
	})
}

// TestBreakerPreciseTriggerPoint tests to identify exactly when the breaker triggers
// after detecting an upward trend
func TestBreakerPreciseTriggerPoint(t *testing.T) {
	// Configuration with trend analysis enabled
	config := &breaker.Config{
		MemoryThreshold:             90.0,
		LatencyThreshold:            300,
		LatencyWindowSize:           15,
		Percentile:                  0.95,
		WaitTime:                    60,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 3, // Only 3 samples needed to detect trend
	}

	b := breaker.NewBreaker(config)
	breaker.SetMemoryOK(b.(*breaker.BreakerDriver), true)

	now := time.Now()

	// Add latencies above threshold one by one and verify after each addition
	latencies := []int{
		310, // #1 - Above threshold
		320, // #2 - Above threshold
		330, // #3 - Above threshold and we now have 3 with upward trend
		340, // #4 - Continuing the trend
		350, // #5 - Continuing the trend
	}

	// The breaker should not be triggered initially
	assert.False(t, b.TriggeredByLatencies(), "The breaker should not be triggered initially")

	var breakerTriggeredAt int = -1

	// Add latencies one by one and check when it triggers
	for i, latency := range latencies {
		startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
		endTime := now.Add(time.Duration(i) * time.Second)
		b.Done(startTime, endTime)

		if b.TriggeredByLatencies() && breakerTriggeredAt == -1 {
			breakerTriggeredAt = i
			t.Logf("The breaker triggered after adding latency #%d: %dms", i+1, latency)
		}
	}

	// Verify that the breaker triggered and that it was at the expected point (after the 3rd latency)
	assert.True(t, b.TriggeredByLatencies(), "The breaker should be triggered after all latencies")
	assert.Equal(t, 2, breakerTriggeredAt, "The breaker should trigger exactly after the 3rd latency (index 2)")
}

// TestBreakerWithDifferentTrendPatterns tests the behavior of the breaker with different trend patterns
func TestBreakerWithDifferentTrendPatterns(t *testing.T) {
	config := &breaker.Config{
		MemoryThreshold:             90.0,
		LatencyThreshold:            300,
		LatencyWindowSize:           20,
		Percentile:                  0.95,
		WaitTime:                    60,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 5,
	}

	// Helper function to print pattern analysis details
	printPatternDetails := func(t *testing.T, pattern string, latencies []int) {
		// Check increasing sequences
		increasing := true
		for i := 1; i < len(latencies); i++ {
			if latencies[i] < latencies[i-1] {
				increasing = false
				break
			}
		}

		// Calculate slope
		sumX, sumY, sumXY, sumX2 := float64(0), float64(0), float64(0), float64(0)
		for i, latency := range latencies {
			x, y := float64(i), float64(latency)
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}
		n := float64(len(latencies))
		slope := float64(0)
		if n*sumX2-sumX*sumX != 0 {
			slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
		}

		t.Logf("Pattern %s details:", pattern)
		t.Logf("  Latencies: %v", latencies)
		t.Logf("  Strictly increasing: %v", increasing)
		t.Logf("  Linear regression slope: %.2f", slope)
		t.Logf("  First value: %d, Last value: %d", latencies[0], latencies[len(latencies)-1])
	}

	// Create latency patterns for different scenarios
	patterns := map[string]struct {
		latencies     []int
		shouldTrigger bool
		description   string
		setup         func(b breaker.Breaker)
	}{
		"Oscillating": {
			latencies:     []int{310, 330, 320, 340, 330, 350, 340, 360, 350, 370},
			shouldTrigger: true,
			description:   "Oscillating but with general upward trend",
		},
		"Peaks": {
			latencies:     []int{310, 360, 320, 370, 330, 380, 340, 390, 350, 400},
			shouldTrigger: true,
			description:   "Alternating peaks but with general upward trend",
		},
		"Steps": {
			latencies:     []int{310, 310, 310, 350, 350, 350, 390, 390, 390, 430},
			shouldTrigger: true,
			description:   "Discrete ascending steps",
		},
		"Plateau": {
			latencies:     []int{310, 320, 330, 340, 350, 350, 350, 350, 350, 350},
			shouldTrigger: true,
			description:   "Ascending at first but then stabilizes in a plateau",
			setup: func(b breaker.Breaker) {
				// Print details about this pattern
				printPatternDetails(t, "Plateau", []int{310, 320, 330, 340, 350, 350, 350, 350, 350, 350})
			},
		},
		"Inverted-U": {
			latencies:     []int{310, 330, 350, 370, 390, 410, 390, 370, 350, 330},
			shouldTrigger: true,
			description:   "Inverted U shape, rises and then falls",
		},
		"Stable": {
			latencies:     []int{350, 350, 350, 350, 350, 350, 350, 350, 350, 350},
			shouldTrigger: true,
			description:   "Completely stable above threshold",
		},
	}

	// Test each pattern
	for name, pattern := range patterns {
		t.Run(name, func(t *testing.T) {
			b := breaker.NewBreaker(config)
			breaker.SetMemoryOK(b.(*breaker.BreakerDriver), true)

			now := time.Now()

			if pattern.setup != nil {
				pattern.setup(b)
			}

			// Special handling for Plateau pattern to check at each step
			if name == "Plateau" {
				// Add the latency pattern one by one and check after each
				var triggeredAt int = -1
				for i, latency := range pattern.latencies {
					startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
					endTime := now.Add(time.Duration(i) * time.Second)
					b.Done(startTime, endTime)

					// Check if it triggered
					if b.TriggeredByLatencies() && triggeredAt == -1 {
						triggeredAt = i
						t.Logf("Breaker triggered at step %d with latency %d", i, latency)
					}
				}

				// For the Plateau pattern, we're testing that the breaker triggers
				// So not being triggered at any point is a failure
				if triggeredAt == -1 {
					t.Logf("Plateau pattern should trigger but didn't")
					assert.False(t, true, "The breaker should trigger for pattern: %s", pattern.description)
				} else {
					assert.True(t, b.TriggeredByLatencies(), "The breaker should trigger for pattern: %s", pattern.description)
				}
			} else {
				// Normal handling for other patterns
				// Add the latency pattern
				for i, latency := range pattern.latencies {
					startTime := now.Add(time.Duration(i)*time.Second - time.Duration(latency)*time.Millisecond)
					endTime := now.Add(time.Duration(i) * time.Second)
					b.Done(startTime, endTime)
				}

				result := b.TriggeredByLatencies()

				if pattern.shouldTrigger {
					assert.True(t, result, "The breaker should trigger for pattern: %s", pattern.description)
				} else {
					assert.False(t, result, "The breaker should NOT trigger for pattern: %s", pattern.description)
				}
			}
		})
	}
}
