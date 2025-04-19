package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"

	// Import the breaker package
	cb "github.com/lrleon/go-breaker/breaker"
)

// example of a server using breaker

var delayInMilliseconds time.Duration = 1000

// Create a default configuration
var config = &cb.Config{
	MemoryThreshold:   80,
	LatencyThreshold:  1500,
	LatencyWindowSize: 64,
	Percentile:        0.95,
}

var breakerAPI = cb.BreakerAPI{
	Config: *config,
}

// Create a new breaker
var ApiBreaker = cb.NewBreaker(config)

func test_endpoint(ctx *gin.Context) {

	if !ApiBreaker.Allow() {
		ctx.JSON(http.StatusTooManyRequests, gin.H{"error": "Service unavailable"})
		return
	}

	startTime := time.Now()

	// Simulate a delay
	time.Sleep(delayInMilliseconds * time.Millisecond)

	ctx.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})

	ApiBreaker.Done(startTime, time.Now())
}

func set_delay(ctx *gin.Context) {
	delayStr := ctx.Query("delay")
	delay, err := time.ParseDuration(delayStr)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay"})
		return
	}

	delayInMilliseconds = delay
	ctx.JSON(http.StatusOK, gin.H{"message": "Delay set to " + delayStr})
}

func main() {

	cb.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	// Set endpoints, including the breaker endpoints
	router := gin.Default()
	router.GET("/test", test_endpoint)
	router.POST("/set_delay", set_delay)
	router.GET("/memory", breakerAPI.GetMemory)
	router.GET("/latency", breakerAPI.GetLatency)
	router.GET("/latency_window_size", breakerAPI.GetLatencyWindowSize)
	router.GET("/percentile", breakerAPI.GetPercentile)
	router.GET("/wait", breakerAPI.GetWait)
	router.POST("/set_memory/:threshold", breakerAPI.SetMemory)
	router.POST("/set_latency/:threshold", breakerAPI.SetLatency)
	router.POST("/set_latency_window_size/:size", breakerAPI.SetLatencyWindowSize)
	router.POST("/set_percentile/:percentile", breakerAPI.SetPercentile)
	router.POST("/set_wait/:wait", breakerAPI.SetWait)

	fmt.Println("Starting server at port 8080")

	err := router.Run(":8080")
	if err != nil {
		return
	}

}
