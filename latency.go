package breaker

import (
	"sort"
	"time"
)

type latencyWindow struct {
	values []int64
	index  int
	size   int
}

func newLatencyWindow(size int) *latencyWindow {
	return &latencyWindow{
		values: make([]int64, size),
		size:   size,
	}
}

// This function adds a new latencyWindow measurement to the window and must run
// in a critical section
func (lw *latencyWindow) add(startTime, endTime time.Time) {
	lw.values[lw.index] = endTime.Sub(startTime).Milliseconds()
	lw.index = (lw.index + 1) % lw.size // Circular buffer
}

// This function returns the latencyWindow percentile in milliseconds of the window
// and must run in a critical section
func (lw *latencyWindow) percentile(p float64) int64 {
	sorted := append([]int64{}, lw.values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted))*p)]
}

// Return a slice with the latencies above the threshold
func (lw *latencyWindow) aboveThresholdLatencies(threshold int64) []int64 {

	latencies := []int64{}
	for _, latency := range lw.values {
		if latency > threshold {
			latencies = append(latencies, latency)
		}
	}
	return latencies
}

// Return true if the latencyWindow is above the threshold
func (lw *latencyWindow) aboveThreshold(threshold int64) bool {
	return lw.percentile(0.99) > threshold
}

// Return true if the latencyWindow is below the threshold
func (lw *latencyWindow) belowThreshold(threshold int64) bool {
	return lw.percentile(0.99) < threshold
}

// Return true if the latencyWindow is OK
func (b *breaker) latencyOK() bool {
	return b.latencyWindow.belowThreshold(b.config.LatencyThreshold)
}
