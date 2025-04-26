package breaker

import (
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

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

type ResetRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
}

func (b *BreakerAPI) Reset(ctx *gin.Context) {
	var req ResetRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
		return
	}

	if !req.Confirm {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Reset not confirmed", "message": "Set confirm:true to reset the breaker"})
		return
	}

	b.Driver.Reset()
	ctx.JSON(http.StatusOK, gin.H{"message": "Breaker reset"})
}

// BreakerStatus represents the complete status of the circuit breaker
type BreakerStatus struct {
	// Overall breaker state
	Enabled      bool      `json:"enabled"`
	Triggered    bool      `json:"triggered"`
	LastTripTime time.Time `json:"last_trip_time,omitempty"`

	// Memory metrics
	MemoryOK           bool    `json:"memory_ok"`
	CurrentMemoryUsage int64   `json:"current_memory_usage_mb"`
	MemoryThreshold    float64 `json:"memory_threshold_percent"`

	// Latency metrics
	LatencyOK         bool    `json:"latency_ok"`
	CurrentPercentile int64   `json:"current_percentile_ms"`
	LatencyThreshold  int64   `json:"latency_threshold_ms"`
	PercentileValue   float64 `json:"percentile_value"`

	// Configuration
	LatencyWindowSize int `json:"latency_window_size"`
	WaitTime          int `json:"wait_time_seconds"`

	// Recent latencies
	RecentLatencies []int64 `json:"recent_latencies_ms"`

	// Trend analysis
	TrendAnalysisEnabled        bool `json:"trend_analysis_enabled"`
	TrendAnalysisMinSampleCount int  `json:"trend_analysis_min_sample_count"`
	HasPositiveTrend            bool `json:"has_positive_trend"`
}

// GetBreakerStatus returns detailed information about the current state of the circuit breaker
func (b *BreakerAPI) GetBreakerStatus(ctx *gin.Context) {
	b.lock.Lock()
	defer b.lock.Unlock()

	// Get the BreakerDriver from the interface
	// We need to cast it to access internal details
	driver, ok := b.Driver.(*BreakerDriver)
	if !ok {
		// If the driver is not a BreakerDriver, return a minimal status
		ctx.JSON(http.StatusOK, gin.H{
			"enabled":    b.Driver.IsEnabled(),
			"triggered":  b.Driver.TriggeredByLatencies(),
			"memory_ok":  b.Driver.MemoryOK(),
			"latency_ok": b.Driver.LatencyOK(),
		})
		return
	}

	// Need to acquire the driver's mutex to access internal state safely
	driver.mu.Lock()
	defer driver.mu.Unlock()

	// Get current memory usage
	currentMemoryUsageMB := MemoryUsage()

	// Get current latency percentile
	latencyPercentile := driver.latencyWindow.Percentile(driver.config.Percentile)

	// Get recent latencies
	recentLatencies := driver.latencyWindow.GetRecentLatencies()

	// Check if there's a positive trend in latencies
	hasPositiveTrend := false
	if len(recentLatencies) >= driver.config.TrendAnalysisMinSampleCount {
		hasPositiveTrend = driver.latencyWindow.HasPositiveTrend(driver.config.TrendAnalysisMinSampleCount)
	}

	// Prepare the status object
	status := BreakerStatus{
		Enabled:                     driver.enabled,
		Triggered:                   driver.triggered,
		MemoryOK:                    driver.MemoryOK(),
		CurrentMemoryUsage:          currentMemoryUsageMB,
		MemoryThreshold:             driver.config.MemoryThreshold,
		LatencyOK:                   driver.LatencyOK(),
		CurrentPercentile:           latencyPercentile,
		LatencyThreshold:            driver.config.LatencyThreshold,
		PercentileValue:             driver.config.Percentile,
		LatencyWindowSize:           driver.config.LatencyWindowSize,
		WaitTime:                    driver.config.WaitTime,
		RecentLatencies:             recentLatencies,
		TrendAnalysisEnabled:        driver.config.TrendAnalysisEnabled,
		TrendAnalysisMinSampleCount: driver.config.TrendAnalysisMinSampleCount,
		HasPositiveTrend:            hasPositiveTrend,
	}

	// Only include last trip time if the breaker is triggered
	if driver.triggered {
		status.LastTripTime = driver.lastTripTime
	}

	ctx.JSON(http.StatusOK, status)
}

// AddEndpointToRouter adds all the breaker endpoints to the provided router
func AddEndpointToRouter(router *gin.Engine, breakerAPI *BreakerAPI) {
	// Create a router group for the breaker
	breakerGroup := router.Group("/breaker")
	{
		breakerGroup.GET("/status", breakerAPI.GetBreakerStatus)
		breakerGroup.GET("/enabled", breakerAPI.GetEnabled)
		breakerGroup.POST("/enabled", breakerAPI.SetEnabled)
		breakerGroup.POST("/disabled", breakerAPI.SetDisabled)
		breakerGroup.GET("/memory", breakerAPI.GetMemory)
		breakerGroup.POST("/memory", breakerAPI.SetMemory)
		breakerGroup.GET("/latency", breakerAPI.GetLatency)
		breakerGroup.POST("/latency", breakerAPI.SetLatency)
		breakerGroup.GET("/latency-window-size", breakerAPI.GetLatencyWindowSize)
		breakerGroup.POST("/latency-window-size", breakerAPI.SetLatencyWindowSize)
		breakerGroup.GET("/percentile", breakerAPI.GetPercentile)
		breakerGroup.POST("/percentile", breakerAPI.SetPercentile)
		breakerGroup.GET("/wait", breakerAPI.GetWait)
		breakerGroup.POST("/wait", breakerAPI.SetWait)
		breakerGroup.GET("/memory-usage", breakerAPI.GetMemoryUsage)
		breakerGroup.GET("/trend-analysis", breakerAPI.GetTrendAnalysis)
		breakerGroup.POST("/trend-analysis", breakerAPI.SetTrendAnalysis)
		breakerGroup.GET("/latencies-above-threshold", breakerAPI.LatenciesAboveThreshold)
		breakerGroup.GET("/memory-limit", breakerAPI.GetMemoryLimit)
		breakerGroup.POST("/reset", breakerAPI.Reset)
	}
}
