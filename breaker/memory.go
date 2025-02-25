package breaker

import (
	"bytes"
	"log"
	"os"
	"runtime"
	"strconv"
)

var MemoryLimitFile = "/sys/fs/cgroup/memory/memory.limit_in_bytes"

func GetK8sMemoryLimit() (int64, error) {
	data, err := os.ReadFile(MemoryLimitFile)
	if err != nil {
		log.Printf("Error reading memory limit file: %v", err)
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
		log.Print("Not running in a k8s environment")
		return
	}
	var err error
	MemoryLimit, err = GetK8sMemoryLimit()
	if err != nil {
		log.Printf("Error getting memory limit: %v", err)
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
func (b *breaker) MemoryOK() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	currMem := float64(m.Alloc)
	memLimit := float64(MemoryLimit) * b.config.MemoryThreshold

	return currMem < memLimit
}

// SetMemoryLimitFile Set the memory limit file for testing
func SetMemoryLimitFile(sz int64) {
	MemoryLimit = sz
}
