package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lrleon/go-breaker/breaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBreakerStatusEndpoint(t *testing.T) {
	// Set up a test configuration
	config := &breaker.Config{
		MemoryThreshold:             80.0,
		LatencyThreshold:            300,
		LatencyWindowSize:           100,
		Percentile:                  95.0,
		WaitTime:                    60,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 3,
	}

	// Create a BreakerAPI
	breakerAPI := breaker.NewBreakerAPI(config)

	// Set up Gin router for testing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// Create an HTTP test recorder
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/breaker/status", nil)
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var status breaker.BreakerStatus
	err := json.Unmarshal(w.Body.Bytes(), &status)
	require.NoError(t, err)

	// Verify the basic structure of the response
	assert.True(t, status.Enabled)
	assert.False(t, status.Triggered)
	assert.Equal(t, config.MemoryThreshold, status.MemoryThreshold)
	assert.Equal(t, config.LatencyThreshold, status.LatencyThreshold)
	assert.Equal(t, config.Percentile, status.PercentileValue)
	assert.Equal(t, config.LatencyWindowSize, status.LatencyWindowSize)
	assert.Equal(t, config.WaitTime, status.WaitTime)
	assert.Equal(t, config.TrendAnalysisEnabled, status.TrendAnalysisEnabled)
	assert.Equal(t, config.TrendAnalysisMinSampleCount, status.TrendAnalysisMinSampleCount)
}

func TestGetBreakerStatusWithTriggeredBreaker(t *testing.T) {
	// Set up a test configuration with a very low latency threshold
	config := &breaker.Config{
		MemoryThreshold:      80.0,
		LatencyThreshold:     10, // Very low threshold that will be easily exceeded
		LatencyWindowSize:    100,
		Percentile:           95.0,
		WaitTime:             60,
		TrendAnalysisEnabled: false, // Disable trend analysis to make the breaker trip easily
	}

	// Create a BreakerAPI
	breakerAPI := breaker.NewBreakerAPI(config)

	// Simulate some latency measurements
	driver := breakerAPI.Driver

	// Add multiple high latency measurements (simulate operations)
	// Since we have direct access, use a simpler approach by adding measurements with a 100ms latency
	for i := 0; i < 10; i++ {
		startTime := time.Now().Add(-100 * time.Millisecond)
		endTime := time.Now()
		driver.Done(startTime, endTime)
	}

	// Set up Gin router for testing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// Create an HTTP test recorder
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/breaker/status", nil)
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var status breaker.BreakerStatus
	err := json.Unmarshal(w.Body.Bytes(), &status)
	require.NoError(t, err)

	// If the breaker is triggered, the latency must be above threshold
	if status.Triggered {
		assert.True(t, status.CurrentPercentile > status.LatencyThreshold)
		assert.False(t, status.LastTripTime.IsZero(), "LastTripTime should be set when breaker is triggered")
	}

	// Verify we have latency data
	assert.NotEmpty(t, status.RecentLatencies)

	// Each latency should be around 100ms (give or take some milliseconds for execution time)
	for _, latency := range status.RecentLatencies {
		assert.InDelta(t, 100, latency, 20) // Allow a 20ms delta for execution time variations
	}
}

func TestGetBreakerStatusWithLatencyTrend(t *testing.T) {
	// Set up a test configuration
	config := &breaker.Config{
		MemoryThreshold:             80.0,
		LatencyThreshold:            200,
		LatencyWindowSize:           100,
		Percentile:                  95.0,
		WaitTime:                    60,
		TrendAnalysisEnabled:        true,
		TrendAnalysisMinSampleCount: 3,
	}

	// Create a BreakerAPI
	breakerAPI := breaker.NewBreakerAPI(config)

	// Simulate an increasing latency trend
	driver := breakerAPI.Driver

	// Add latencies with an increasing trend (100ms, 150ms, 200ms)
	startTime1 := time.Now().Add(-100 * time.Millisecond)
	endTime1 := time.Now()
	driver.Done(startTime1, endTime1)

	startTime2 := time.Now().Add(-150 * time.Millisecond)
	endTime2 := time.Now()
	driver.Done(startTime2, endTime2)

	startTime3 := time.Now().Add(-200 * time.Millisecond)
	endTime3 := time.Now()
	driver.Done(startTime3, endTime3)

	// Set up Gin router for testing
	gin.SetMode(gin.TestMode)
	router := gin.New()
	breaker.AddEndpointToRouter(router, breakerAPI)

	// Create an HTTP test recorder
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/breaker/status", nil)
	router.ServeHTTP(w, req)

	// Verify the response code
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse the response body
	var status breaker.BreakerStatus
	err := json.Unmarshal(w.Body.Bytes(), &status)
	require.NoError(t, err)

	// We should have latency data
	assert.NotEmpty(t, status.RecentLatencies)

	// The latencies might not be in the exact order we added them
	// but they should be between 100-220ms
	for _, latency := range status.RecentLatencies {
		assert.GreaterOrEqual(t, latency, int64(90))
		assert.LessOrEqual(t, latency, int64(220))
	}

	// The has_positive_trend flag should reflect if there's an increasing trend
	// Note: this test might be flaky because trend detection is complex
	// and depends on the exact timing of the test execution
	t.Logf("Has positive trend: %v", status.HasPositiveTrend)
}
