package breaker

import "runtime"

// Return true if the memory usage is above the threshold
func (b *breaker) memoryOK() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	usedMemory := float64(m.Alloc) / float64(m.Sys) * 100
	status := usedMemory > b.config.MemoryThreshold

	return status
}
