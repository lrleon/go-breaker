# Go Breaker

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/lrleon/go-breaker)](https://goreportcard.com/report/github.com/lrleon/go-breaker)

A circuit breaker implementation in Go that helps prevent system overload by monitoring memory usage and latency metrics, automatically tripping when thresholds are exceeded.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Interface](#interface)
- [Basic Usage](#basic-usage)
- [Server Integration Example](#server-integration-example)
- [OpsGenie Integration](#opsgenie-integration)
  - [Configuration](#configuration)
  - [Environment Variables](#environment-variables)
  - [Usage with OpsGenie](#usage-with-opsgenie)
- [General Configuration](#general-configuration)
- [HTTP API Endpoints](#http-api-endpoints)
- [Utility Scripts](#utility-scripts)
- [License](#license)

## Features

- Memory usage monitoring with configurable thresholds
- Latency tracking with percentile-based thresholds
- Sliding window for latency measurements
- Configurable wait time before circuit reset
- Thread-safe operation
- HTTP API for runtime configuration
- Kubernetes-aware memory limit detection
- OpsGenie integration for circuit breaker event alerts

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

## OpsGenie Integration

Go-Breaker integrates with OpsGenie to send alerts when circuit breaker events occur, helping teams quickly respond to service degradation.

### Configuration

Configure OpsGenie integration in your TOML configuration file:

```toml
[opsgenie]
enabled = true
region = "us"
source = "go-breaker"
tags = ["production", "api"]
team = "platform-team"
trigger_on_open = true
trigger_on_reset = true
trigger_on_memory = true
trigger_on_latency = true
include_latency_metrics = true
include_memory_metrics = true
include_system_info = true
latency_threshold = 1500
alert_cooldown_seconds = 300
priority = "P2"

# API Information to include in alerts
api_name = "Payment API"
api_version = "v1.2.3"
api_namespace = "payment"
api_description = "Handles payment processing"
api_owner = "Payments Team"
api_priority = "critical"
```

### Environment Variables

For security reasons, the OpsGenie API key is configured via environment variables:

```bash
export OPSGENIE_API_KEY="your-api-key-here"
export OPSGENIE_REGION="us"  # Optional, defaults to "us"
export OPSGENIE_API_URL=""   # Optional, for custom API endpoints
export OPSGENIE_REQUIRED="false"  # Optional, exit on OpsGenie init failure if "true"
```

### Usage with OpsGenie

When configured, Go-Breaker will automatically send alerts to OpsGenie:

1. When the circuit breaker trips (opens)
2. When the circuit breaker resets (closes)
3. When memory usage exceeds the configured threshold
4. When latency exceeds the configured threshold

Each alert contains:
- API information and details
- Current breaker state
- Performance metrics
- System information (when enabled)

For more details, see the [TOML Configuration Guide](TOML_CONFIG.md).

## General Configuration

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
# breakers.toml
memory_threshold = 80.0
latency_threshold = 1500
latency_window_size = 64
percentile = 0.95
wait_time = 10
```

```go
config, err := breaker.LoadConfig("breakers.toml")
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