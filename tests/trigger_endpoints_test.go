package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTriggerBreakerByMemoryEndpoint(t *testing.T) {
	// Set up memory limit for testing
	breaker.SetMemoryLimitFile(512 * 1024 * 1024) // 512MB

	// Set up a test configuration
	config := &breaker.Config{
		MemoryThreshold:   80.0,
		LatencyThreshold:  300,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          60,
	}

	// Create a BreakerAPI
	breakerAPI := breaker.NewBreakerAPI(config)

	// Set up Gin router for testing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// Ensure the breaker starts in a non-triggered state
	assert.False(t, breakerAPI.Driver.TriggeredByLatencies(), "Breaker should not be triggered initially")

	// Create an HTTP test recorder for the memory trigger endpoint
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/breaker/trigger-by-memory", nil)
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Debug: Print the actual response
	t.Logf("Memory trigger response: %+v", response)

	// Verify the response structure
	assert.Contains(t, response, "message")
	assert.Contains(t, response, "triggered")
	assert.Contains(t, response, "reason")
	assert.Contains(t, response, "note")
	assert.Contains(t, response, "timestamp")
	assert.Equal(t, "manual_memory_trigger", response["reason"])

	// Verify that the breaker is now in a triggered state (Allow should return false)
	assert.False(t, breakerAPI.Driver.Allow(), "Breaker should not allow requests after memory trigger")
}

func TestTriggerBreakerByLatencyEndpoint(t *testing.T) {
	// Set up memory limit for testing
	breaker.SetMemoryLimitFile(512 * 1024 * 1024) // 512MB

	// Set up a test configuration with lower thresholds for easier triggering
	config := &breaker.Config{
		MemoryThreshold:      80.0,
		LatencyThreshold:     100, // Low threshold for easier triggering
		LatencyWindowSize:    5,   // Small window for faster triggering
		Percentile:           0.95,
		WaitTime:             60,
		TrendAnalysisEnabled: false, // Disable trend analysis for predictable behavior
	}

	// Create a BreakerAPI
	breakerAPI := breaker.NewBreakerAPI(config)

	// Set up Gin router for testing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// Ensure the breaker starts in a non-triggered state
	assert.False(t, breakerAPI.Driver.TriggeredByLatencies(), "Breaker should not be triggered initially")

	// Create an HTTP test recorder for the latency trigger endpoint
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/breaker/trigger-by-latency", nil)
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Debug: Print the actual response
	t.Logf("Latency trigger response: %+v", response)

	// Verify the response structure
	assert.Contains(t, response, "message")
	assert.Contains(t, response, "triggered")
	assert.Contains(t, response, "reason")
	assert.Contains(t, response, "latency_used")
	assert.Contains(t, response, "measurements_added")
	assert.Contains(t, response, "note")
	assert.Contains(t, response, "timestamp")
	assert.Equal(t, "manual_latency_trigger", response["reason"])

	// Verify that the breaker was triggered
	triggered, ok := response["triggered"].(bool)
	require.True(t, ok, "triggered field should be a boolean")
	assert.True(t, triggered, "Breaker should be triggered after latency trigger")

	// Double-check by verifying the breaker state directly
	assert.True(t, breakerAPI.Driver.TriggeredByLatencies(), "Breaker should be triggered after latency trigger")
}

func TestTriggerEndpointsWithResetFlow(t *testing.T) {
	// Set up memory limit for testing
	breaker.SetMemoryLimitFile(512 * 1024 * 1024) // 512MB

	// Test the complete flow: trigger -> verify -> reset -> verify
	config := &breaker.Config{
		MemoryThreshold:      80.0,
		LatencyThreshold:     100,
		LatencyWindowSize:    5,
		Percentile:           0.95,
		WaitTime:             60,
		TrendAnalysisEnabled: false,
	}

	breakerAPI := breaker.NewBreakerAPI(config)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// 1. Initial state - not triggered
	assert.False(t, breakerAPI.Driver.TriggeredByLatencies(), "Initial state should not be triggered")

	// 2. Trigger by latency
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/breaker/trigger-by-latency", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.True(t, breakerAPI.Driver.TriggeredByLatencies(), "Should be triggered after latency trigger")

	// 3. Reset the breaker
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/breaker/reset",
		strings.NewReader(`{"confirm": true}`))
	req2.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.False(t, breakerAPI.Driver.TriggeredByLatencies(), "Should not be triggered after reset")

	// 4. Trigger by memory
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/breaker/trigger-by-memory", nil)
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	// Note: Memory trigger affects Allow() behavior, not TriggeredByLatencies()
	assert.False(t, breakerAPI.Driver.Allow(), "Should not allow requests after memory trigger")

	// 5. Restore memory check
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest("GET", "/breaker/restore-memory-check", nil)
	router.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
	assert.True(t, breakerAPI.Driver.Allow(), "Should allow requests after memory restore")
}

func TestRestoreMemoryCheckEndpoint(t *testing.T) {
	// Test the restore memory check endpoint specifically

	// Set up memory limit for testing to avoid "Cannot determine memory limit" warning
	breaker.SetMemoryLimitFile(512 * 1024 * 1024) // 512MB

	config := &breaker.Config{
		MemoryThreshold:   80.0,
		LatencyThreshold:  300,
		LatencyWindowSize: 10,
		Percentile:        0.95,
		WaitTime:          60,
	}

	breakerAPI := breaker.NewBreakerAPI(config)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// 1. Trigger memory issue
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/breaker/trigger-by-memory", nil)
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.False(t, breakerAPI.Driver.Allow(), "Should not allow after memory trigger")

	// 2. Restore memory check
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/breaker/restore-memory-check", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Parse the response
	var response map[string]interface{}
	err := json.Unmarshal(w2.Body.Bytes(), &response)
	require.NoError(t, err)

	// Debug: Print the actual response to see what we're getting
	t.Logf("Restore memory check response: %+v", response)

	// Verify response structure
	assert.Contains(t, response, "message")
	assert.Contains(t, response, "action")
	assert.Contains(t, response, "note")
	assert.Contains(t, response, "timestamp")
	assert.Equal(t, "memory_check_restored", response["action"])

	// 3. Verify that requests are now allowed
	assert.True(t, breakerAPI.Driver.Allow(), "Should allow requests after memory restore")
}
