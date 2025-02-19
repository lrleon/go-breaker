package breaker

import "runtime"

func (b *breaker) memoryOK() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	usedMemory := float64(m.Alloc) / float64(m.Sys) * 100
	return usedMemory > b.config.MemoryThreshold
}
