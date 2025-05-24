package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	cb "github.com/lrleon/go-breaker/breaker"
)

// Enhanced test server for OpsGenie integration testing
// This server allows easy testing of all circuit breaker scenarios

var delayInMilliseconds time.Duration = 1000

const defaultConfigPath = "breakers.toml"

var breakerAPI *cb.BreakerAPI
var ApiBreaker cb.Breaker
var testScenarios map[string]TestScenario

// TestScenario represents different testing scenarios
type TestScenario struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Action      func() error
}

// Request structures for API endpoints
type DelayRequest struct {
	Delay string `json:"delay" binding:"required"`
}

type TriggerRequest struct {
	Scenario string `json:"scenario" binding:"required"`
}

type AlertTestRequest struct {
	AlertType string            `json:"alert_type" binding:"required"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details"`
}

// Initialize test scenarios
func initTestScenarios() {
	testScenarios = map[string]TestScenario{
		"memory_overload": {
			Name:        "Memory Overload",
			Description: "Simulates memory threshold breach by setting very low memory limit",
			Action: func() error {
				// Set a very low memory limit to trigger memory alerts
				cb.SetMemoryLimitFile(100 * 1024 * 1024) // 100MB
				log.Printf("🧠 Memory limit set to 100MB to trigger memory alerts")
				return nil
			},
		},
		"high_latency": {
			Name:        "High Latency",
			Description: "Sets high delay to trigger latency-based circuit breaker",
			Action: func() error {
				delayInMilliseconds = 2000 // 2 seconds
				log.Printf("⏱️  Delay set to 2000ms to trigger latency alerts")
				return nil
			},
		},
		"latency_spike": {
			Name:        "Latency Spike Pattern",
			Description: "Creates increasing latency pattern to test trend analysis",
			Action: func() error {
				// This will be handled by multiple requests with increasing delays
				log.Printf("📈 Latency spike pattern initiated - make multiple requests to /test")
				return nil
			},
		},
		"reset_normal": {
			Name:        "Reset to Normal",
			Description: "Resets all parameters to normal operation",
			Action: func() error {
				delayInMilliseconds = 100                // 100ms normal delay
				cb.SetMemoryLimitFile(512 * 1024 * 1024) // 512MB normal limit
				ApiBreaker.Reset()
				log.Printf("✅ All parameters reset to normal operation")
				return nil
			},
		},
	}
}

// Main test endpoint with circuit breaker protection
func testEndpoint(ctx *gin.Context) {
	if !ApiBreaker.Allow() {
		ctx.JSON(http.StatusTooManyRequests, gin.H{
			"error":             "Service unavailable - Circuit breaker is OPEN",
			"breaker_triggered": true,
			"suggestion":        "Wait for circuit breaker to reset or call /breaker/reset",
		})
		return
	}

	startTime := time.Now()

	// Check for latency spike scenario
	if delayInMilliseconds == 0 {
		// Simulate increasing latency pattern for trend analysis testing
		requestCount := getRequestCount()
		delayInMilliseconds = time.Duration(300 + (requestCount * 50)) // 300ms, 350ms, 400ms, etc.
	}

	// Simulate processing time
	time.Sleep(delayInMilliseconds * time.Millisecond)

	endTime := time.Now()
	actualLatency := endTime.Sub(startTime).Milliseconds()

	// Record latency with circuit breaker
	ApiBreaker.Done(startTime, endTime)

	ctx.JSON(http.StatusOK, gin.H{
		"status":              "success",
		"configured_delay_ms": delayInMilliseconds,
		"actual_latency_ms":   actualLatency,
		"breaker_status": gin.H{
			"enabled":    ApiBreaker.IsEnabled(),
			"triggered":  ApiBreaker.TriggeredByLatencies(),
			"memory_ok":  ApiBreaker.MemoryOK(),
			"latency_ok": ApiBreaker.LatencyOK(),
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

var requestCounter int

func getRequestCount() int {
	requestCounter++
	return requestCounter
}

// Set delay for testing different latency scenarios
func setDelay(ctx *gin.Context) {
	var req DelayRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Delay == "spike" {
		delayInMilliseconds = 0 // Special value for spike pattern
		requestCounter = 0
		ctx.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Latency spike pattern enabled - each request will have increasing delay",
		})
		return
	}

	d, err := time.ParseDuration(req.Delay)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delay format. Use format like '500ms', '2s', etc."})
		return
	}

	delayInMilliseconds = d / time.Millisecond
	ctx.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"delay_ms": delayInMilliseconds,
		"message":  fmt.Sprintf("Delay set to %v", d),
	})
}

// Trigger specific test scenarios
func triggerScenario(ctx *gin.Context) {
	var req TriggerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scenario, exists := testScenarios[req.Scenario]
	if !exists {
		available := []string{}
		for key := range testScenarios {
			available = append(available, key)
		}
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":               "Unknown scenario",
			"available_scenarios": available,
		})
		return
	}

	err := scenario.Action()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to execute scenario",
			"details": err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":      "success",
		"scenario":    scenario.Name,
		"description": scenario.Description,
		"message":     "Scenario executed successfully",
	})
}

// Get available test scenarios
func getScenarios(ctx *gin.Context) {
	scenarios := []map[string]string{}
	for key, scenario := range testScenarios {
		scenarios = append(scenarios, map[string]string{
			"key":         key,
			"name":        scenario.Name,
			"description": scenario.Description,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"scenarios": scenarios,
		"usage":     "POST to /test/trigger with {\"scenario\": \"scenario_key\"}",
	})
}

// Enhanced OpsGenie validation endpoint
func validateOpsGenieConfiguration(ctx *gin.Context) {
	if breakerAPI == nil || breakerAPI.Config.OpsGenie == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "OpsGenie configuration not available",
		})
		return
	}

	client := cb.GetOpsGenieClient(breakerAPI.Config.OpsGenie)

	if err := client.ValidateMandatoryFields(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":         "validation_failed",
			"missing_fields": err.MissingFields,
			"invalid_fields": err.InvalidFields,
			"message":        "Mandatory fields validation failed",
			"suggestion":     "Check your breakers.toml configuration and environment variables",
		})
		return
	}

	report := client.GenerateConfigurationReport()

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "All mandatory fields validated successfully",
		"report":  report,
	})
}

// Test OpsGenie connectivity
func testOpsGenieConnection(ctx *gin.Context) {
	if breakerAPI == nil || breakerAPI.Config.OpsGenie == nil || !breakerAPI.Config.OpsGenie.Enabled {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "OpsGenie is not enabled or configured",
		})
		return
	}

	client := cb.GetOpsGenieClient(breakerAPI.Config.OpsGenie)

	if !client.IsInitialized() {
		err := client.Initialize()
		if err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "error",
				"message": fmt.Sprintf("Failed to initialize OpsGenie client: %v", err),
			})
			return
		}
	}

	err := client.TestConnection()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to OpsGenie: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Successfully connected to OpsGenie API",
	})
}

// Send manual test alert to OpsGenie
func sendTestAlert(ctx *gin.Context) {
	var req AlertTestRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Use default values if no request body
		req = AlertTestRequest{
			AlertType: "manual-test",
			Message:   "Manual test alert from circuit breaker test server",
			Details: map[string]string{
				"test_type": "manual",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			},
		}
	}

	if breakerAPI == nil || breakerAPI.Config.OpsGenie == nil || !breakerAPI.Config.OpsGenie.Enabled {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "OpsGenie is not enabled or configured",
		})
		return
	}

	client := cb.GetOpsGenieClient(breakerAPI.Config.OpsGenie)

	if !client.IsInitialized() {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "error",
			"message": "OpsGenie client is not initialized",
		})
		return
	}

	// Validate mandatory fields before sending
	if err := client.ValidateMandatoryFields(); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"status":  "validation_failed",
			"message": fmt.Sprintf("Cannot send test alert - mandatory fields validation failed: %v", err),
		})
		return
	}

	// Send test alert based on type
	var err error
	switch req.AlertType {
	case "circuit-open", "manual-test":
		err = client.SendBreakerOpenAlert(999, false, 60) // Test values
	case "circuit-reset":
		err = client.SendBreakerResetAlert()
	case "memory-threshold":
		memoryStatus := &cb.MemoryStatus{
			CurrentUsage: 95.0,
			Threshold:    80.0,
			TotalMemory:  1024 * 1024 * 1024,
			UsedMemory:   950 * 1024 * 1024,
			OK:           false,
		}
		err = client.SendMemoryThresholdAlert(memoryStatus)
	case "latency-threshold":
		err = client.SendLatencyThresholdAlert(2000, 1500)
	default:
		err = client.SendBreakerOpenAlert(999, false, 60) // Default to circuit-open
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to send test alert: %v", err),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Test alert sent to OpsGenie successfully",
		"alert_type": req.AlertType,
		"note":       "Check your OpsGenie dashboard for the alert",
	})
}

// Get comprehensive system status
func getSystemStatus(ctx *gin.Context) {
	status := gin.H{
		"server": gin.H{
			"status":           "running",
			"current_delay_ms": delayInMilliseconds,
			"request_count":    requestCounter,
			"uptime":           time.Since(startTime).String(),
		},
		"circuit_breaker": gin.H{
			"enabled":    ApiBreaker.IsEnabled(),
			"triggered":  ApiBreaker.TriggeredByLatencies(),
			"memory_ok":  ApiBreaker.MemoryOK(),
			"latency_ok": ApiBreaker.LatencyOK(),
		},
		"configuration": gin.H{
			"memory_threshold":    breakerAPI.Config.MemoryThreshold,
			"latency_threshold":   breakerAPI.Config.LatencyThreshold,
			"latency_window_size": breakerAPI.Config.LatencyWindowSize,
			"percentile":          breakerAPI.Config.Percentile,
			"wait_time":           breakerAPI.Config.WaitTime,
		},
	}

	// Add OpsGenie status if configured
	if breakerAPI.Config.OpsGenie != nil && breakerAPI.Config.OpsGenie.Enabled {
		client := cb.GetOpsGenieClient(breakerAPI.Config.OpsGenie)
		opsgenieStatus := gin.H{
			"enabled":     true,
			"initialized": client.IsInitialized(),
		}

		// Add mandatory fields status
		if err := client.ValidateMandatoryFields(); err != nil {
			opsgenieStatus["validation"] = "failed"
			opsgenieStatus["validation_error"] = err.Error()
		} else {
			opsgenieStatus["validation"] = "passed"
		}

		// Add mandatory field values
		mandatoryFields := map[string]interface{}{}
		if client != nil {
			// We'd need to expose these values - for now, get from config
			mandatoryFields["team"] = breakerAPI.Config.OpsGenie.Team
			mandatoryFields["environment"] = breakerAPI.Config.OpsGenie.Environment
			mandatoryFields["bookmaker_id"] = breakerAPI.Config.OpsGenie.BookmakerID
			mandatoryFields["business"] = breakerAPI.Config.OpsGenie.Business
			if breakerAPI.Config.OpsGenie.AdditionalContext != "" {
				mandatoryFields["additional_context"] = breakerAPI.Config.OpsGenie.AdditionalContext
			}
		}
		opsgenieStatus["mandatory_fields"] = mandatoryFields

		status["opsgenie"] = opsgenieStatus
	} else {
		status["opsgenie"] = gin.H{"enabled": false}
	}

	ctx.JSON(http.StatusOK, status)
}

var startTime time.Time

func main() {
	startTime = time.Now()
	log.Printf("🚀 Starting Enhanced Circuit Breaker Test Server")
	log.Printf("📝 This server is designed for easy OpsGenie integration testing")

	// Set memory limit for testing
	cb.MemoryLimit = 512 * 1024 * 1024 // 512 MB

	// Load configuration
	config, err := cb.LoadConfig(defaultConfigPath)
	if err != nil {
		log.Printf("❌ Failed to load configuration: %v", err)
		log.Println("Creating test configuration...")

		// Create test configuration with OpsGenie enabled
		config = &cb.Config{
			MemoryThreshold:   80,
			LatencyThreshold:  1500,
			LatencyWindowSize: 64,
			Percentile:        0.95,
			WaitTime:          10,
			OpsGenie: &cb.OpsGenieConfig{
				Enabled:               true,
				Region:                "us",
				Priority:              "P3",
				Source:                "go-breaker-test",
				Tags:                  []string{"test", "circuit-breaker"},
				TriggerOnOpen:         true,
				TriggerOnReset:        true,
				TriggerOnMemory:       true,
				TriggerOnLatency:      true,
				IncludeLatencyMetrics: true,
				IncludeMemoryMetrics:  true,
				IncludeSystemInfo:     true,
				AlertCooldownSeconds:  60, // Shorter cooldown for testing
				Team:                  "test-team",
				Environment:           "TEST",
				BookmakerID:           "test-bookmaker",
				Business:              "internal",
				AdditionalContext:     "test-server-circuit-breaker",
				APIName:               "Test Circuit Breaker API",
				APIVersion:            "v1.0.0",
				APIDescription:        "Test server for circuit breaker functionality",
			},
		}

		// Save test configuration
		if err := cb.SaveConfig(defaultConfigPath, config); err != nil {
			log.Printf("⚠️  Failed to save test configuration: %v", err)
		} else {
			log.Printf("✅ Test configuration saved to %s", defaultConfigPath)
		}
	}

	// Validate OpsGenie configuration
	if config.OpsGenie != nil && config.OpsGenie.Enabled {
		log.Printf("🔍 Validating OpsGenie configuration...")

		// Show environment variables needed
		log.Printf("📋 Required Environment Variables:")
		log.Printf("   OPSGENIE_API_KEY = %s", getEnvStatus("OPSGENIE_API_KEY"))
		log.Printf("   Environment = %s", getEnvStatus("Environment"))

		client := cb.GetOpsGenieClient(config.OpsGenie)
		if err := client.ValidateConfigurationAtStartup(); err != nil {
			log.Printf("⚠️  OpsGenie validation warnings: %v", err)
			log.Printf("📘 You can still test, but alerts may use fallback values")
		} else {
			log.Printf("✅ OpsGenie configuration validated successfully")
		}
	} else {
		log.Printf("ℹ️  OpsGenie integration is disabled")
	}

	// Initialize test scenarios
	initTestScenarios()

	// Initialize breaker
	breakerAPI = cb.NewBreakerAPI(config)
	ApiBreaker = cb.NewBreaker(config, defaultConfigPath)

	// Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Add CORS middleware for testing from different origins
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Main application endpoints
	router.GET("/test", testEndpoint)
	router.POST("/test/delay", setDelay)
	router.POST("/test/trigger", triggerScenario)
	router.GET("/test/scenarios", getScenarios)

	// System status and health
	router.GET("/status", getSystemStatus)
	router.GET("/health", getSystemStatus) // Alias

	// OpsGenie testing endpoints
	opsgenieGroup := router.Group("/opsgenie")
	{
		opsgenieGroup.GET("/validate", validateOpsGenieConfiguration)
		opsgenieGroup.GET("/test-connection", testOpsGenieConnection)
		opsgenieGroup.POST("/send-test-alert", sendTestAlert)
	}

	// Add all standard breaker endpoints
	cb.AddEndpointToRouter(router, breakerAPI)

	// Print startup information
	printStartupInfo(config)

	// Start server
	fmt.Println("\n🎯 Test Server started on port 8080")
	fmt.Println("📋 Open http://localhost:8080/status to see current status")

	err = router.Run(":8080")
	if err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
}

func getEnvStatus(envVar string) string {
	value := os.Getenv(envVar)
	if value == "" {
		return "❌ NOT SET"
	}
	if len(value) > 10 {
		return fmt.Sprintf("✅ SET (%s...)", value[:10])
	}
	return fmt.Sprintf("✅ SET (%s)", value)
}

func printStartupInfo(config *cb.Config) {
	log.Printf("\n🎮 TEST SERVER READY FOR OPSGENIE TESTING")
	log.Printf("==========================================")
	log.Printf("📊 Circuit Breaker Configuration:")
	log.Printf("   - Memory Threshold: %.1f%%", config.MemoryThreshold)
	log.Printf("   - Latency Threshold: %dms", config.LatencyThreshold)
	log.Printf("   - Wait Time: %ds", config.WaitTime)

	if config.OpsGenie != nil && config.OpsGenie.Enabled {
		log.Printf("📤 OpsGenie Configuration:")
		log.Printf("   - Team: %s", config.OpsGenie.Team)
		log.Printf("   - Environment: %s", config.OpsGenie.Environment)
		log.Printf("   - BookmakerID: %s", config.OpsGenie.BookmakerID)
		log.Printf("   - Business: %s", config.OpsGenie.Business)
		if config.OpsGenie.AdditionalContext != "" {
			log.Printf("   - Additional Context: %s", config.OpsGenie.AdditionalContext)
		}
		log.Printf("   - Cooldown: %ds", config.OpsGenie.AlertCooldownSeconds)
	}

	log.Printf("\n🌐 Available Test Endpoints:")
	log.Printf("   GET  /status                     - System status and configuration")
	log.Printf("   GET  /test                       - Protected endpoint with circuit breaker")
	log.Printf("   POST /test/delay                 - Set response delay: {\"delay\": \"2s\"}")
	log.Printf("   POST /test/trigger               - Trigger scenarios: {\"scenario\": \"high_latency\"}")
	log.Printf("   GET  /test/scenarios             - List available test scenarios")
	log.Printf("")
	log.Printf("🔧 OpsGenie Test Endpoints:")
	log.Printf("   GET  /opsgenie/validate          - Validate mandatory fields")
	log.Printf("   GET  /opsgenie/test-connection   - Test OpsGenie API connection")
	log.Printf("   POST /opsgenie/send-test-alert   - Send manual test alert")
	log.Printf("")
	log.Printf("⚙️  Circuit Breaker Management:")
	log.Printf("   GET  /breaker/status             - Detailed breaker status")
	log.Printf("   POST /breaker/reset              - Reset circuit breaker")
	log.Printf("   GET  /breaker/memory             - Current memory threshold")
	log.Printf("   GET  /breaker/latency            - Current latency threshold")

	log.Printf("\n🧪 TESTING WORKFLOW:")
	log.Printf("1. Check status: curl http://localhost:8080/status")
	log.Printf("2. Validate OpsGenie: curl http://localhost:8080/opsgenie/validate")
	log.Printf("3. Test connection: curl http://localhost:8080/opsgenie/test-connection")
	log.Printf("4. Send test alert: curl -X POST http://localhost:8080/opsgenie/send-test-alert")
	log.Printf("5. Trigger scenarios: curl -X POST http://localhost:8080/test/trigger -d '{\"scenario\":\"high_latency\"}'")
	log.Printf("6. Make requests: curl http://localhost:8080/test")
	log.Printf("7. Check OpsGenie dashboard for alerts!")
}
