package breaker

import (
	"sort"
	"time"
)

// LatencyRecord stores a latency along with its timestamp
type LatencyRecord struct {
	Value     int64
	Timestamp time.Time
}

type LatencyWindow struct {
	Records       []LatencyRecord
	Index         int
	Size          int
	NeedToSort    bool
	MaxAgeMinutes int // New parameter: maximum age in minutes to consider a latency valid
}

func NewLatencyWindow(size int) *LatencyWindow {
	return &LatencyWindow{
		Records:       make([]LatencyRecord, size),
		Size:          size,
		MaxAgeMinutes: 5, // Default: 5 minutes
	}
}

// Add This function adds a new LatencyWindow measurement to the window and must run
// in a critical section
func (lw *LatencyWindow) Add(startTime, endTime time.Time) {
	n := len(lw.Records)
	lw.Records[lw.Index] = LatencyRecord{
		Value:     endTime.Sub(startTime).Milliseconds(),
		Timestamp: endTime,
	}
	lw.Index = (lw.Index + 1) % n // Circular buffer
	lw.NeedToSort = true
}

// Reset This function resets the LatencyWindow and must run in a critical section
func (lw *LatencyWindow) Reset() {
	lw.Records = make([]LatencyRecord, lw.Size)
	lw.Index = 0
	lw.NeedToSort = false
}

// GetRecentLatencies returns only latencies within the configured time period
func (lw *LatencyWindow) GetRecentLatencies() []int64 {
	cutoffTime := time.Now().Add(-time.Duration(lw.MaxAgeMinutes) * time.Minute)
	var recentValues []int64

	for _, record := range lw.Records {
		// Only consider records with valid timestamps (not zero time) and recent ones
		if !record.Timestamp.IsZero() && record.Timestamp.After(cutoffTime) {
			recentValues = append(recentValues, record.Value)
		}
	}

	return recentValues
}

// Percentile This function returns the LatencyWindow percentile in milliseconds of the window
// and must run in a critical section
func (lw *LatencyWindow) Percentile(p float64) int64 {
	recentValues := lw.GetRecentLatencies()

	// If there are no recent values, return 0
	if len(recentValues) == 0 {
		return 0
	}

	sorted := append([]int64{}, recentValues...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(float64(len(sorted)) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}

// AboveThresholdLatencies Return a slice with the latencies above the threshold
func (lw *LatencyWindow) AboveThresholdLatencies(threshold int64) []int64 {
	recentValues := lw.GetRecentLatencies()
	latencies := []int64{}

	for _, latency := range recentValues {
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
