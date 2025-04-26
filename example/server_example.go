package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"time"

	// Import the breaker package
	cb "github.com/lrleon/go-breaker/breaker"
)

// Example of a server using breaker
//
// OpsGenie Integration:
// This example can be configured to send alerts to OpsGenie when the circuit breaker is triggered.
// For security, the OpsGenie API key should be set via environment variables:
//
//   export OPSGENIE_API_KEY="your-api-key-here"
//   export OPSGENIE_REGION="us"  # Optional, defaults to "us"
//   export OPSGENIE_API_URL=""   # Optional, for custom OpsGenie endpoints
//
// Additionally, you can set OPSGENIE_REQUIRED=true if the application should fail
// when OpsGenie integration fails to initialize.

var delayInMilliseconds time.Duration = 1000

// Default path for configuration file
const defaultConfigPath = "BreakerDriver-Config.toml"

var breakerAPI *cb.BreakerAPI
var ApiBreaker cb.Breaker
var opsGenieClient *cb.OpsGenieClient

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

	time.Sleep(delayInMilliseconds * time.Millisecond)
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "delay": delayInMilliseconds})

	ApiBreaker.Done(startTime, time.Now())
}

// Endpoint to set delay value
func set_delay(ctx *gin.Context) {
	var req DelayRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse the delay value
	d, err := time.ParseDuration(req.Delay)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay format"})
		return
	}

	delayInMilliseconds = d / time.Millisecond
	ctx.JSON(http.StatusOK, gin.H{"status": "success", "delay_ms": delayInMilliseconds})
}

// testOpsGenieConnection is a new endpoint to test the OpsGenie connection
func testOpsGenieConnection(ctx *gin.Context) {
	if opsGenieClient == nil || !opsGenieClient.IsInitialized() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "OpsGenie client is not initialized",
		})
		return
	}

	// Test the connection
	err := opsGenieClient.TestConnection()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to OpsGenie: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Successfully connected to OpsGenie",
	})
}

func main() {
	// Set memory limit for the breaker
	cb.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	// Load configuration
	config, err := cb.LoadConfig(defaultConfigPath)
	if err != nil {
		log.Printf("Warning: Failed to load configuration: %v", err)
		log.Println("Creating default configuration")

		// Create default configuration if loading fails
		config = &cb.Config{
			MemoryThreshold:   80,
			LatencyThreshold:  1500,
			LatencyWindowSize: 64,
			Percentile:        0.95,
			OpsGenie:          &cb.OpsGenieConfig{Enabled: false},
		}
	}

	// Initialize the breaker API and the breaker itself
	breakerAPI = cb.NewBreakerAPI(config)
	ApiBreaker = cb.NewBreaker(config)

	// Initialize OpsGenie client if configuration is available
	if config.OpsGenie != nil && config.OpsGenie.Enabled {
		opsGenieClient = cb.NewOpsGenieClient(config.OpsGenie)

		// Try to initialize the OpsGenie client
		err := opsGenieClient.Initialize()
		if err != nil {
			log.Printf("Error initializing OpsGenie client: %v", err)

			// If API key validation fails, we can either continue without OpsGenie or exit
			if os.Getenv("OPSGENIE_REQUIRED") == "true" {
				log.Fatalf("OpsGenie integration is required but failed to initialize: %v", err)
			} else {
				log.Println("Continuing without OpsGenie integration")
				// Disable OpsGenie to prevent further attempts
				config.OpsGenie.Enabled = false
			}
		} else {
			log.Println("OpsGenie integration initialized successfully")
		}
	} else {
		log.Println("OpsGenie integration is disabled")
	}

	// Set up the router and endpoints
	router := gin.Default()

	// Application endpoints
	router.GET("/test", testEndpoint)
	router.POST("/set_delay", set_delay)

	// Add OpsGenie test endpoint
	router.GET("/test_opsgenie", testOpsGenieConnection)

	// Add all breaker endpoints
	cb.AddEndpointToRouter(router, breakerAPI)

	fmt.Println("Starting server at port 8080")

	err = router.Run(":8080")
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
