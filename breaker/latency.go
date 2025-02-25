package breaker

import (
	"sort"
	"time"
)

type LatencyWindow struct {
	Values     []int64
	Index      int
	Size       int
	NeedToSort bool
}

func NewLatencyWindow(size int) *LatencyWindow {
	return &LatencyWindow{
		Values: make([]int64, size),
		Size:   size,
	}
}

// Add This function adds a new LatencyWindow measurement to the window and must run
// in a critical section
func (lw *LatencyWindow) Add(startTime, endTime time.Time) {
	n := len(lw.Values)
	lw.Values[lw.Index] = endTime.Sub(startTime).Milliseconds()
	lw.Index = (lw.Index + 1) % n // Circular buffer
	lw.NeedToSort = true
}

// Percentile This function returns the LatencyWindow percentile in milliseconds of the window
// and must run in a critical section
func (lw *LatencyWindow) Percentile(p float64) int64 {
	if lw.NeedToSort {
		sorted := append([]int64{}, lw.Values...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		return sorted[int(float64(len(sorted))*p)]
	}
	return lw.Values[int(float64(len(lw.Values))*p)]
}

// AboveThresholdLatencies Return a slice with the latencies above the threshold
func (lw *LatencyWindow) AboveThresholdLatencies(threshold int64) []int64 {

	latencies := []int64{}
	for _, latency := range lw.Values {
		if latency > threshold {
			latencies = append(latencies, latency)
		}
	}
	return latencies
}

// AboveThreshold Return true if the LatencyWindow is above the threshold
func (lw *LatencyWindow) AboveThreshold(threshold int64) bool {
	return lw.Percentile(0.99) > threshold
}

// BelowThreshold Return true if the LatencyWindow is below the threshold
func (lw *LatencyWindow) BelowThreshold(threshold int64) bool {
	return lw.Percentile(0.99) < threshold
}

// LatencyOK Return true if the LatencyWindow is OK
func (b *BreakerDriver) LatencyOK() bool {
	return b.latencyWindow.BelowThreshold(b.config.LatencyThreshold)
}
