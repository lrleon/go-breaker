package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	cb "github.com/lrleon/go-breaker"
	"net/http"
	"time"
)

// example of a server using breaker

var config = cb.Config{
	MemoryThreshold:   80,
	LatencyThreshold:  1500,
	LatencyWindowSize: 64,
	Percentile:        0.95,
}

var delayInMilliseconds time.Duration = 1000

var ApiBreaker = cb.NewBreaker(config)

var BreakerAPI cb.BreakerAPI = cb.BreakerAPI{
	Config: config,
}

func test_endpoint(ctx *gin.Context) {
	if !ApiBreaker.Allow() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Service unavailable"})
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

	// Set endpoints, including the breaker endpoints
	router := gin.Default()
	router.GET("/", test_endpoint)
	router.GET("/set_delay", set_delay)
	router.GET("/breaker/memory", BreakerAPI.GetMemory)
	router.GET("/breaker/latency", BreakerAPI.GetLatency)
	router.GET("/breaker/latency_window_size", BreakerAPI.GetLatencyWindowSize)
	router.GET("/breaker/percentile", BreakerAPI.GetPercentile)
	router.GET("/breaker/wait", BreakerAPI.GetWait)
	router.GET("/breaker/set_memory", BreakerAPI.SetMemory)
	router.GET("/breaker/set_latency", BreakerAPI.SetLatency)
	router.GET("/breaker/set_latency_window_size", BreakerAPI.SetLatencyWindowSize)
	router.GET("/breaker/set_percentile", BreakerAPI.SetPercentile)
	router.GET("/breaker/set_wait", BreakerAPI.SetWait)

	fmt.Println("Starting server at port 8080")

	err := router.Run(":8080")
	if err != nil {
		return
	}

}
