package breaker

import (
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

type BreakerAPI struct {
	Config Config
	lock   sync.Mutex
}

func (b *BreakerAPI) SetMemory(ctx *gin.Context) {

	// error if memory threshold is less than 0 or greater or equal
	// than 100
	thresholdStr := ctx.Query("threshold")
	threshold, err := strconv.Atoi(thresholdStr)
	if err != nil {
		log.Printf("Invalid memory threshold: %v", thresholdStr)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid memory threshold"})
		return
	}

	if threshold < 0 || threshold >= 100 {
		log.Printf("Invalid memory threshold: %v", threshold)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid memory threshold"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.MemoryThreshold = float64(threshold)
	err = SaveConfig("breaker-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Memory threshold set to " + thresholdStr})
}

func (b *BreakerAPI) GetMemory(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"memory": b.Config.MemoryThreshold})
}

func (b *BreakerAPI) SetLatency(ctx *gin.Context) {

	// error if a latency threshold is less than 5 ms or greater or equal
	// than 5000 ms
	thresholdStr := ctx.Query("threshold")
	threshold, err := strconv.Atoi(thresholdStr)
	if err != nil {
		log.Printf("Invalid latency threshold: %v", thresholdStr)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latency threshold"})
		return
	}

	if threshold < 5 || threshold >= 5000 {
		log.Printf("Invalid latency threshold: %v", threshold)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latency threshold"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.LatencyThreshold = int64(threshold)
	err = SaveConfig("breaker-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Latency threshold set to " + thresholdStr})
}

func (b *BreakerAPI) GetLatency(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"latency": b.Config.LatencyThreshold})
}

func (b *BreakerAPI) SetLatencyWindowSize(ctx *gin.Context) {

	// error if size window Size is less than 11 or greater or equal
	// than 1021
	sizeStr := ctx.Query("size")
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		log.Printf("Invalid size window Size: %v", sizeStr)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid size window Size"})
		return
	}

	if size < 1 || size >= 1021 {
		log.Printf("Invalid size window Size: %v", size)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid size window Size"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.LatencyWindowSize = size
	err = SaveConfig("breaker-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Latency window Size set to " + sizeStr})
}

func (b *BreakerAPI) GetLatencyWindowSize(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"latency": b.Config.LatencyWindowSize})
}

func (b *BreakerAPI) SetPercentile(ctx *gin.Context) {

	// error if percentile is less than 40 or greater than 99
	percentileStr := ctx.Query("percentile")
	percentile, err := strconv.ParseFloat(percentileStr, 64)
	if err != nil {
		log.Printf("Invalid percentile: %v", percentileStr)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid percentile"})
		return
	}

	if percentile < 40 || percentile > 99 {
		log.Printf("Invalid percentile: %v", percentile)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid percentile"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.Percentile = percentile
	err = SaveConfig("breaker-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Percentile set to " + percentileStr})
}

func (b *BreakerAPI) GetPercentile(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"percentile": b.Config.Percentile})
}

func (b *BreakerAPI) SetWait(ctx *gin.Context) {

	// error if wait is less than 1 seconds or greater or equal than 10 seconds
	waitStr := ctx.Query("wait")
	wait, err := strconv.Atoi(waitStr)
	if err != nil {
		log.Printf("Invalid wait: %v", waitStr)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wait"})
		return
	}

	if wait < 1 || wait >= 10 {
		log.Printf("Invalid wait: %v", wait)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wait"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.WaitTime = wait
	err = SaveConfig("breaker-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Wait set to " + waitStr})
}

func (b *BreakerAPI) GetWait(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"wait": b.Config.WaitTime})
}
