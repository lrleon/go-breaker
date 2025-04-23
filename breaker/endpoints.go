package breaker

import (
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// Request body structures for JSON parameters
type MemoryThresholdRequest struct {
	Threshold int `json:"threshold" binding:"required"`
}

type LatencyThresholdRequest struct {
	Threshold int `json:"threshold" binding:"required"`
}

type LatencyWindowSizeRequest struct {
	Size int `json:"size" binding:"required"`
}

type PercentileRequest struct {
	Percentile float64 `json:"percentile" binding:"required"`
}

type WaitTimeRequest struct {
	WaitTime int `json:"wait_time" binding:"required"`
}

type BreakerAPI struct {
	Config Config
	Driver Breaker
	lock   sync.Mutex
}

func NewBreakerAPI(config *Config) *BreakerAPI {
	return &BreakerAPI{
		Config: *config,
		Driver: NewBreaker(config),
	}
}

func (b *BreakerAPI) SetEnabled(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.Driver.Enable()
	ctx.JSON(http.StatusOK, gin.H{"message": "Breaker enabled"})
}

func (b *BreakerAPI) SetDisabled(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()
	b.Driver.Disable()
	ctx.JSON(http.StatusOK, gin.H{"message": "Breaker disabled"})
}

func (b *BreakerAPI) GetEnabled(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()
	ctx.JSON(http.StatusOK, gin.H{"enabled": b.Driver.IsEnabled()})
}

func (b *BreakerAPI) SetMemory(ctx *gin.Context) {
	var request MemoryThresholdRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid memory threshold request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid memory threshold request"})
		return
	}

	threshold := request.Threshold

	// error if memory threshold is less than 0 or greater or equal than 100
	if threshold < 0 || threshold >= 100 {
		log.Printf("Invalid memory threshold: %v", threshold)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid memory threshold"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.MemoryThreshold = float64(threshold)
	err := SaveConfig("BreakerDriver-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Memory threshold set to " + strconv.Itoa(threshold)})
}

func (b *BreakerAPI) GetMemory(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"memory": b.Config.MemoryThreshold})
}

func (b *BreakerAPI) SetLatency(ctx *gin.Context) {
	var request LatencyThresholdRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid latency threshold request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latency threshold request"})
		return
	}

	threshold := request.Threshold

	// error if a latency threshold is less than 5 ms or greater or equal than 5000 ms
	if threshold < 5 || threshold >= 5000 {
		log.Printf("Invalid latency threshold: %v", threshold)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latency threshold"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.LatencyThreshold = int64(threshold)
	err := SaveConfig("BreakerDriver-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Latency threshold set to " + strconv.Itoa(threshold)})
}

func (b *BreakerAPI) GetLatency(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"latency": b.Config.LatencyThreshold})
}

func (b *BreakerAPI) SetLatencyWindowSize(ctx *gin.Context) {
	var request LatencyWindowSizeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid latency window size request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid latency window size request"})
		return
	}

	size := request.Size

	// error if size window Size is less than 11 or greater or equal than 1021
	if size < 1 || size >= 1021 {
		log.Printf("Invalid size window Size: %v", size)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid size window Size"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.LatencyWindowSize = size
	err := SaveConfig("BreakerDriver-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Latency window Size set to " + strconv.Itoa(size)})
}

func (b *BreakerAPI) GetLatencyWindowSize(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"latency": b.Config.LatencyWindowSize})
}

// TODO: instead of using a slice to ve sorted for getting the percentile,
// use binary search tree with ranges

// TODO: use float for the percentiles

const MinPercentile = 1.0
const MaxPercentile = 99.99999999999999

func (b *BreakerAPI) SetPercentile(ctx *gin.Context) {
	var request PercentileRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid percentile request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid percentile request"})
		return
	}

	percentile := request.Percentile

	if percentile < MinPercentile || percentile > MaxPercentile {
		log.Printf("Invalid percentile: %v (must be in[%f, %f])", percentile, MinPercentile, MaxPercentile)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid percentile"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.Percentile = percentile / 100.0
	err := SaveConfig("BreakerDriver-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Percentile set to " + strconv.FormatFloat(percentile, 'f', -1, 64)})
}

func (b *BreakerAPI) GetPercentile(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"percentile": b.Config.Percentile})
}

func (b *BreakerAPI) SetWait(ctx *gin.Context) {
	var request WaitTimeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid wait time request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wait time request"})
		return
	}

	wait := request.WaitTime

	// error if wait is less than 1 second or greater or equal than 10 seconds
	if wait < 1 || wait >= 10 {
		log.Printf("Invalid wait: %v", wait)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wait"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.WaitTime = wait
	err := SaveConfig("BreakerDriver-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Wait set to " + strconv.Itoa(wait)})
}

func (b *BreakerAPI) GetWait(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"wait": b.Config.WaitTime})
}

// GetMemoryUsage Return the most recent memory usage
func (b *BreakerAPI) GetMemoryUsage(ctx *gin.Context) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	ctx.JSON(http.StatusOK, gin.H{"memory_usage": m.Alloc})
}

// GetTrendAnalysis Return the trend analysis of the latencies
func (b *BreakerAPI) GetTrendAnalysis(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	ctx.JSON(http.StatusOK, gin.H{"trend_analysis": b.Config.TrendAnalysisEnabled})
}

type TrendAnalysisRequest struct {
	Enabled bool `json:"enabled"`
}

func (b *BreakerAPI) SetTrendAnalysis(ctx *gin.Context) {
	var request TrendAnalysisRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		log.Printf("Invalid trend analysis request: %v", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid trend analysis request"})
		return
	}

	enabled := request.Enabled

	b.lock.Lock()
	defer b.lock.Unlock()

	b.Config.TrendAnalysisEnabled = enabled
	err := SaveConfig("BreakerDriver-Config.toml", &b.Config)
	if err != nil {
		log.Printf("Failed to save Config: %v", err)
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save Config"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Trend analysis set to " + strconv.FormatBool(enabled)})
}

func (b *BreakerAPI) LatenciesAboveThreshold(ctx *gin.Context) {
	thresholdStr := ctx.Param("threshold")
	threshold, err := strconv.Atoi(thresholdStr)
	if err != nil {
		log.Printf("Invalid threshold: %v", thresholdStr)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid threshold"})
		return
	}

	b.lock.Lock()
	defer b.lock.Unlock()

	latencies := b.Driver.LatenciesAboveThreshold(int64(threshold))
	ctx.JSON(http.StatusOK, gin.H{"latencies": latencies})
}

func (b *BreakerAPI) GetMemoryLimit(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"memory_limit": MemoryLimit})
}

func (b *BreakerAPI) Reset(ctx *gin.Context) {
	b.Driver.Reset()
	ctx.JSON(http.StatusOK, gin.H{"message": "Breaker reset"})
}

func AddEndpointToRouter(router *gin.Engine, breakerAPI *BreakerAPI) {
	group := router.Group("/breaker")
	group.GET("/enabled", breakerAPI.GetEnabled)
	group.POST("/disable", breakerAPI.SetDisabled)
	group.POST("/enable", breakerAPI.SetEnabled)
	group.GET("/memory", breakerAPI.GetMemory)
	group.GET("/latency", breakerAPI.GetLatency)
	group.GET("/latency_window_size", breakerAPI.GetLatencyWindowSize)
	group.GET("/percentile", breakerAPI.GetPercentile)
	group.GET("/wait", breakerAPI.GetWait)
	group.POST("/set_memory", breakerAPI.SetMemory)
	group.POST("/set_latency", breakerAPI.SetLatency)
	group.POST("/set_latency_window_size", breakerAPI.SetLatencyWindowSize)
	group.POST("/set_percentile", breakerAPI.SetPercentile)
	group.POST("/set_wait", breakerAPI.SetWait)
	group.GET("/memory_usage", breakerAPI.GetMemoryUsage)
	group.GET("/latencies_above_threshold/:threshold", breakerAPI.LatenciesAboveThreshold)
	group.GET("/memory_limit", breakerAPI.GetMemoryLimit)
	group.POST("/reset", breakerAPI.Reset)
	group.POST("/set_trend_analysis", breakerAPI.SetTrendAnalysis)
	group.GET("/trend_analysis", breakerAPI.GetTrendAnalysis)
}
