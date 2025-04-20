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

var breakerAPI = cb.NewBreakerAPI(config)

// Create a new breaker
var ApiBreaker = cb.NewBreaker(config)

// Request body structure for delay parameter
type DelayRequest struct {
	Delay string `json:"delay" binding:"required"`
}

func testEndpoint(ctx *gin.Context) {

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
	var request DelayRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay request"})
		return
	}

	delay, err := time.ParseDuration(request.Delay)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay format"})
		return
	}

	delayInMilliseconds = delay
	ctx.JSON(http.StatusOK, gin.H{"message": "Delay set to " + request.Delay})
}

func main() {

	cb.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	// Set endpoints, including the breaker endpoints
	router := gin.Default()

	// Application endpoints
	router.GET("/test", testEndpoint)
	router.POST("/set_delay", set_delay)

	// Add all breaker endpoints
	cb.AddEndpointToRouter(router, breakerAPI)

	fmt.Println("Starting server at port 8080")

	err := router.Run(":8080")
	if err != nil {
		return
	}

}
