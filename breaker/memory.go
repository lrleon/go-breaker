package breaker

import (
	"bytes"
	"os"
	"runtime"
	"strconv"
)

var MemoryLimitFile = "/sys/fs/cgroup/memory/memory.limit_in_bytes"

// Variables for testing only
var (
	memoryOverride      bool
	memoryOverrideValue bool
	memoryLogger        = NewLogger("MemoryMonitor")
)

// SetMemoryOK is used only for testing to override the memory check
// This allows tests to control whether memory is considered OK
func SetMemoryOK(b *BreakerDriver, value bool) {
	memoryOverride = true
	memoryOverrideValue = value
}

func GetK8sMemoryLimit() (int64, error) {
	data, err := os.ReadFile(MemoryLimitFile)
	if err != nil {
		memoryLogger.Logf("Error reading memory limit file: %v", err)
		return 0, err
	}
	data = bytes.TrimSpace(data)
	limit, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

var MemoryLimit int64 // MemoryLimit is the memory limit of the container

func init() {

	// run only if we are in a k8s environment
	if _, err := os.Stat(MemoryLimitFile); os.IsNotExist(err) {
		memoryLogger.Logf("Not running in a k8s environment")
		return
	}
	var err error
	MemoryLimit, err = GetK8sMemoryLimit()
	if err != nil {
		memoryLogger.Logf("Error getting memory limit: %v", err)
		panic(err)
	}
}

// MemoryUsage Return the current memory usage in MB
func MemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return int64(m.Alloc) / 1024 / 1024
}

// MemoryOK Return true if the memory usage is above the threshold. The threshold is
// calculated based on the memory limit of the container
func (b *BreakerDriver) MemoryOK() bool {
	// For testing purposes
	if memoryOverride {
		return memoryOverrideValue
	}

	// If we do not have a valid memory limit, we cannot verify
	if MemoryLimit <= 0 {
		memoryLogger.Logf("Warning: Invalid memory limit (%d). Cannot perform memory threshold check.", MemoryLimit)
		return true // We assume that memory is fine if we don't have a valid limit
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currMem := float64(m.Alloc)

	// To avoid loss of precision, we make the division before multiplication
	// we convert the percentage to fraction by dividing by 100
	thresholdFraction := b.config.MemoryThreshold / 100.0
	memLimit := float64(MemoryLimit) * thresholdFraction

	memoryOK := currMem < memLimit

	// Detailed logging for debug if the memory is close to the limit
	if currMem > (memLimit * 0.9) {
		memoryLogger.Logf("Memory usage is high: %.2f MB of %.2f MB (%.2f%% of limit)",
			currMem/1024/1024, memLimit/1024/1024, 100*currMem/float64(MemoryLimit))
	}

	return memoryOK
}

// SetMemoryLimitFile Set the memory limit file for testing
func SetMemoryLimitFile(sz int64) {
	MemoryLimit = sz
}
