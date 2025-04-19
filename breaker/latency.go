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
	MaxAgeSeconds int // Maximum age in seconds to consider a latency valid
}

func NewLatencyWindow(size int) *LatencyWindow {
	return &LatencyWindow{
		Records:       make([]LatencyRecord, size),
		Size:          size,
		MaxAgeSeconds: 300, // Default: 5 minutes (300 seconds)
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
	cutoffTime := time.Now().Add(-time.Duration(lw.MaxAgeSeconds) * time.Second)
	var recentValues []int64

	for _, record := range lw.Records {
		// Only consider records with valid timestamps (not zero time) and recent ones
		if !record.Timestamp.IsZero() && record.Timestamp.After(cutoffTime) {
			recentValues = append(recentValues, record.Value)
		}
	}

	return recentValues
}

// GetRecentTimeOrderedLatencies returns latencies ordered by timestamp (oldest first)
func (lw *LatencyWindow) GetRecentTimeOrderedLatencies() []LatencyRecord {
	cutoffTime := time.Now().Add(-time.Duration(lw.MaxAgeSeconds) * time.Second)
	var recentRecords []LatencyRecord

	for _, record := range lw.Records {
		// Only consider records with valid timestamps (not zero time) and recent ones
		if !record.Timestamp.IsZero() && record.Timestamp.After(cutoffTime) {
			recentRecords = append(recentRecords, record)
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(recentRecords, func(i, j int) bool {
		return recentRecords[i].Timestamp.Before(recentRecords[j].Timestamp)
	})

	return recentRecords
}

// HasPositiveTrend checks if the latency has an upward trend
// Returns true if the trend is positive (latencies increasing), false otherwise
// minSampleCount specifies the minimum number of samples required for trend analysis
func (lw *LatencyWindow) HasPositiveTrend(minSampleCount int) bool {
	orderedRecords := lw.GetRecentTimeOrderedLatencies()

	// Need at least minSampleCount samples for meaningful trend analysis
	if len(orderedRecords) < minSampleCount {
		return false
	}

	// Use a simple linear regression slope calculation
	n := float64(len(orderedRecords))
	sumX := float64(0)
	sumY := float64(0)
	sumXY := float64(0)
	sumX2 := float64(0)

	// Use timestamps as X values (convert to float64 seconds since epoch)
	// and latency values as Y values
	for i, record := range orderedRecords {
		// X values: use sequential indices for simplicity
		x := float64(i)
		y := float64(record.Value)

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope of the trend line
	// slope = (n*sum(xy) - sum(x)*sum(y)) / (n*sum(x^2) - sum(x)^2)
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return false // Avoid division by zero
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	// Positive slope indicates increasing latencies (positive trend)
	return slope > 0
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
	var latencies []int64

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
