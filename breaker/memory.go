package breaker

import (
	"log"
	"os"
	"runtime"
	"strconv"
)

var MemoryLimitFile = "/sys/fs/cgroup/memory/memory.limit_in_bytes"

func GetK8sMemoryLimit() (int64, error) {
	data, err := os.ReadFile(MemoryLimitFile)
	if err != nil {
		return 0, err
	}
	limit, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return 0, err
	}
	return limit, nil
}

var MemoryLimit int64

func init() {

	// run only if we are in a k8s environment
	if _, err := os.Stat(MemoryLimitFile); os.IsNotExist(err) {
		log.Print("Not running in a k8s environment")
		return
	}
	var err error
	MemoryLimit, err = GetK8sMemoryLimit()
	if err != nil {
		panic(err)
	}
}

// Return true if the memory usage is above the threshold. The threshold is
// calculated based on the memory limit of the container
func (b *breaker) MemoryOK() bool {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return float64(m.Alloc) < float64(MemoryLimit)*b.config.MemoryThreshold
}

// SetMemoryLimitFile Set the memory limit file for testing
func SetMemoryLimitFile(sz int64) {
	MemoryLimit = sz
}
