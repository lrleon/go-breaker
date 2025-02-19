package breaker

import (
	"sort"
	"sync"
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

var lw *latencyWindow
var mx sync.Mutex

func init() {

	initConfig()
	lw = newLatencyWindow(config.LatencyWindowSize)
}

// This function adds a new latency measurement to the window and must run
// in a critical section
func (lw *latencyWindow) add(startTime, endTime time.Time) {
	mx.Lock()
	defer mx.Unlock()
	lw.values[lw.index] = endTime.Sub(startTime).Milliseconds()
	lw.index = (lw.index + 1) % lw.size
}

// This function returns the latency percentile of the window and must run
// in a critical section
func (lw *latencyWindow) percentile(p float64) int64 {
	mx.Lock()
	defer mx.Unlock()
	sorted := append([]int64{}, lw.values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[int(float64(len(sorted))*p)]
}

// Return true if the latency is above the threshold
func (lw *latencyWindow) aboveThreshold(threshold int64) bool {
	return lw.percentile(0.99) > threshold
}

// Return true if the latency is below the threshold
func (lw *latencyWindow) belowThreshold(threshold int64) bool {
	return lw.percentile(0.99) < threshold
}

// Return true if the latency is OK
func (b *breaker) latencyOK() bool {
	return lw.belowThreshold(100)
}
