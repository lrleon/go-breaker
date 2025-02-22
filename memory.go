package go_breaker

import (
	"log"
	"os"
	"runtime"
	"strconv"
)

var memoryLimitFile = "/sys/fs/cgroup/memory/memory.limit_in_bytes"

func getK8sMemoryLimit() (int64, error) {
	data, err := os.ReadFile(memoryLimitFile)
	if err != nil {
		return 0, err
	}
	limit, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

var memoryLimit int64

func init() {

	// run only if we are in a k8s environment
	if _, err := os.Stat(memoryLimitFile); os.IsNotExist(err) {
		log.Print("Not running in a k8s environment")
		return
	}
	var err error
	memoryLimit, err = getK8sMemoryLimit()
	if err != nil {
		panic(err)
	}
}

// Return true if the memory usage is above the threshold. The threshold is
// calculated based on the memory limit of the container
func (b *breaker) memoryOK() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return float64(m.Alloc) < float64(memoryLimit)*b.config.MemoryThreshold
}

// SetMemoryLimitFile Set the memory limit file for testing
func SetMemoryLimitFile(sz int64) {
	memoryLimit = sz
}
