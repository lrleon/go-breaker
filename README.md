# Go Breaker

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/lrleon/go-breaker)](https://goreportcard.com/report/github.com/lrleon/go-breaker)

A circuit breaker implementation in Go that helps prevent system overload by monitoring memory usage and latency metrics, automatically tripping when thresholds are exceeded.

## Features

- Memory usage monitoring with configurable thresholds
- Latency tracking with percentile-based thresholds
- Sliding window for latency measurements
- Configurable wait time before circuit reset
- Thread-safe operation
- HTTP API for runtime configuration
- Kubernetes-aware memory limit detection

## Installation

```bash
go get github.com/lrleon/go-breaker
```

## Interface

The `Breaker` interface defines the following methods:

```go
type Breaker interface {
    Allow() bool                       // Returns if the operation can continue
    Done(startTime, endTime time.Time) // Reports the latency of a finished operation
    TriggeredByLatencies() bool        // Indicates if the breaker is activated
    Reset()                            // Restores the state of the breaker
    LatenciesAboveThreshold(threshold int64) []int64  // Returns latencies above the threshold
    MemoryOK() bool                    // Checks if memory usage is below threshold
    LatencyOK() bool                   // Checks if latencies are below threshold
    IsEnabled() bool                   // Returns if the breaker is enabled
    Disable()                          // Disables the breaker
    Enable()                           // Enables the breaker
}
```

## Basic Usage

```go
package main

import (
    "time"
    "github.com/lrleon/go-breaker/breaker"
)

func main() {
    // Create breaker configuration
    config := &breaker.Config{
        MemoryThreshold:   80.0,       // 80% of memory limit
        LatencyThreshold:  600,        // 600ms latency threshold
        LatencyWindowSize: 64,         // Track last 64 operations
        Percentile:        0.95,       // 95th percentile
        WaitTime:          10,         // Wait 10 seconds before reset
    }

    // Create a new breaker
    b := breaker.NewBreaker(config)

    // In your request handler
    if !b.Allow() {
        // Circuit is open, return error or fallback response
        return
    }

    // Record start time
    startTime := time.Now()
    
    // Perform operation
    // ...
    
    // Record end time and update breaker
    endTime := time.Now()
    b.Done(startTime, endTime)
}
```

## Server Integration Example

```go
package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "time"
    "github.com/lrleon/go-breaker/breaker"
)

func main() {
    // Create configuration
    config := &breaker.Config{
        MemoryThreshold:   80.0,
        LatencyThreshold:  1500,
        LatencyWindowSize: 64,
        Percentile:        0.95,
        WaitTime:          10,
    }

    // Create breaker
    b := breaker.NewBreaker(config)
    
    // Create API handler
    breakerAPI := breaker.NewBreakerAPI(config)
    
    // Set up router
    router := gin.Default()
    
    // Add endpoint with breaker protection
    router.GET("/api/resource", func(ctx *gin.Context) {
        if !b.Allow() {
            ctx.JSON(http.StatusServiceUnavailable, gin.H{
                "error": "Service temporarily unavailable",
            })
            return
        }
        
        startTime := time.Now()
        
        // Process request
        // ...
        
        // Respond
        ctx.JSON(http.StatusOK, gin.H{"data": "response"})
        
        // Update breaker
        b.Done(startTime, time.Now())
    })
    
    // Add breaker management endpoints
    breaker.AddEndpointToRouter(router, breakerAPI)
    
    router.Run(":8080")
}
```

## Configuration

The `Config` struct configures the breaker with the following fields:

| Parameter | Description | Default |
|-----------|-------------|---------|
| MemoryThreshold | Memory threshold as percentage (0-100) | 80.0 |
| LatencyThreshold | Latency threshold in milliseconds | 1500 |
| LatencyWindowSize | Number of operations to track | 64 |
| Percentile | Percentile for latency measurement (0-1) | 0.95 |
| WaitTime | Time to wait after tripping in seconds | 10 |

Configuration can be loaded from a TOML file:

```toml
# config.toml
memory_threshold = 80.0
latency_threshold = 1500
latency_window_size = 64
percentile = 0.95
wait_time = 10
```

```go
config, err := breaker.LoadConfig("config.toml")
if err != nil {
    log.Fatalf("Failed to load config: %v", err)
}
```

## HTTP API Endpoints

When using `AddEndpointToRouter`, the following endpoints are available under `/breaker/`:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/enabled` | GET | Check if the breaker is enabled |
| `/disable` | GET | Disable the breaker |
| `/enable` | GET | Enable the breaker |
| `/memory` | GET | Get memory threshold |
| `/latency` | GET | Get latency threshold |
| `/latency_window_size` | GET | Get latency window size |
| `/percentile` | GET | Get percentile |
| `/wait` | GET | Get wait time |
| `/set_memory/:threshold` | GET | Set memory threshold |
| `/set_latency/:threshold` | GET | Set latency threshold |
| `/set_latency_window_size/:size` | GET | Set latency window size |
| `/set_percentile/:percentile` | GET | Set percentile |
| `/set_wait/:wait_time` | GET | Set wait time |
| `/memory_usage` | GET | Get current memory usage |
| `/latencies_above_threshold/:threshold` | GET | Get latencies above threshold |
| `/memory_limit` | GET | Get memory limit |
| `/reset` | GET | Reset the breaker |

## Utility Scripts

The repository includes Ruby client scripts in the `example` directory for testing and interacting with the breaker API:

```ruby
# Example using the Ruby client
require_relative 'client'

# Set the server URL
$url = "http://localhost:8080/breaker/"

# Get memory threshold
memory()

# Set memory threshold to 85%
set_memory(85)

# Get latencies above 1000ms
get_latencies(1000)
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.