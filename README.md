# Go Breaker

[![MIT License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/lrleon/go-breaker)](https://goreportcard.com/report/github.com/lrleon/go-breaker)

A comprehensive circuit breaker implementation in Go that monitors memory usage and latency metrics, automatically tripping when thresholds are exceeded. Features advanced trend analysis, OpsGenie integration for alerting, and staged alert escalation for production environments.

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Interface](#interface)
- [Configuration](#configuration)
- [OpsGenie Integration](#opsgenie-integration)
- [Staged Alerting](#staged-alerting)
- [HTTP API Endpoints](#http-api-endpoints)
- [Advanced Features](#advanced-features)
- [Examples](#examples)
- [Testing](#testing)
- [Utility Scripts](#utility-scripts)
- [Architecture](#architecture)
- [Contributing](#contributing)
- [License](#license)

## Features

### Core Circuit Breaker Capabilities
- **Memory usage monitoring** with configurable thresholds and Kubernetes-aware memory limit detection
- **Latency tracking** with percentile-based thresholds and sliding window measurements
- **Trend analysis** for intelligent triggering based on latency patterns
- **Configurable wait time** before circuit reset attempts
- **Thread-safe operation** with comprehensive logging
- **HTTP API** for runtime configuration and monitoring

### Advanced Alerting & Monitoring
- **OpsGenie integration** with comprehensive alert management
- **Staged alerting system** with escalation workflows
- **Alert cooldowns** to prevent alert storms
- **Rich alert context** including system metrics and trend analysis
- **Mandatory field validation** for production-ready alerts

### Production Features
- **TOML configuration** with validation and hot-reloading
- **Comprehensive logging** with file and line information
- **Kubernetes deployment ready** with memory limit detection
- **Ruby client scripts** for testing and management
- **Extensive test suite** with benchmarks and integration tests

## Installation

```bash
go get github.com/lrleon/go-breaker
```

## Quick Start

### Basic Usage

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
        TrendAnalysisEnabled: true,    // Enable intelligent trend detection
    }

    // Create a new breaker
    b := breaker.NewBreaker(config, "breakers.toml")

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

### Configuration File Usage

```go
// Load breaker from TOML configuration
b, err := breaker.NewBreakerFromConfigFile("breakers.toml")
if err != nil {
    log.Fatalf("Failed to load breaker: %v", err)
}

// Use the breaker
if b.Allow() {
    // Process request
}
```

## Interface

The `Breaker` interface provides comprehensive circuit breaker functionality:

```go
type Breaker interface {
    Allow() bool                       // Check if operation can proceed
    Done(startTime, endTime time.Time) // Record operation latency
    TriggeredByLatencies() bool        // Check if breaker is triggered
    Reset()                            // Manually reset the breaker
    LatenciesAboveThreshold(threshold int64) []int64  // Get high latencies
    MemoryOK() bool                    // Check memory status
    LatencyOK() bool                   // Check latency status
    IsEnabled() bool                   // Check if breaker is enabled
    Disable()                          // Disable the breaker
    Enable()                           // Enable the breaker
    GetConfigFile() string             // Get configuration file path
}
```

## Configuration

### TOML Configuration File

Create a `breakers.toml` file with your configuration:

```toml
# Core Circuit Breaker Settings
memory_threshold = 80.0              # Memory threshold percentage (0-100)
latency_threshold = 1500             # Latency threshold in milliseconds
latency_window_size = 64             # Number of operations to track
percentile = 0.95                    # Percentile for latency measurement (0-1)
wait_time = 10                       # Time to wait before reset (seconds)

# Advanced Features
trend_analysis_enabled = true        # Enable intelligent trend detection
trend_analysis_min_sample_count = 10 # Minimum samples for trend analysis

# OpsGenie Integration
[opsgenie]
enabled = true
region = "us"
priority = "P2"
source = "go-breaker"
team = "platform-team"
environment = "production"
bookmaker_id = "your-service-id"
business = "internal"

# Alert Configuration
trigger_on_breaker_open = true
trigger_on_breaker_reset = true
trigger_on_memory_threshold = true
trigger_on_latency_threshold = true
include_latency_metrics = true
include_memory_metrics = true
include_system_info = true
alert_cooldown_seconds = 300

# Staged Alerting (Optional)
time_before_send_alert = 60          # Seconds before escalation
initial_alert_priority = "P3"        # Initial alert priority
escalated_alert_priority = "P1"      # Escalated alert priority

# Tags for alert categorization
tags = [
    "Environment:production",
    "Component:circuit-breaker",
    "Team:platform",
    "Priority:critical"
]

# API Information
api_name = "Payment API"
api_version = "v1.2.0"
api_namespace = "payment"
api_description = "Handles payment processing"
api_owner = "Payments Team"
api_priority = "critical"
api_dependencies = ["database", "auth-service"]
api_endpoints = ["/payments", "/refunds", "/transactions"]
```

### Configuration Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `memory_threshold` | Memory threshold as percentage (0-100) | 80.0 |
| `latency_threshold` | Latency threshold in milliseconds | 1500 |
| `latency_window_size` | Number of operations to track | 64 |
| `percentile` | Percentile for latency measurement (0-1) | 0.95 |
| `wait_time` | Time to wait after tripping (seconds) | 10 |
| `trend_analysis_enabled` | Enable intelligent trend detection | false |
| `trend_analysis_min_sample_count` | Minimum samples for trend analysis | 10 |

## OpsGenie Integration

### Environment Variables

For security, configure OpsGenie using environment variables:

```bash
export OPSGENIE_API_KEY="your-api-key-here"
export OPSGENIE_REGION="us"  # Optional: "us" or "eu"
export Environment="production"  # Environment identifier
```

### Alert Types

Go Breaker automatically sends alerts for:

1. **Circuit Breaker Open** - When the circuit trips due to high latency or memory usage
2. **Circuit Breaker Reset** - When the circuit recovers
3. **Memory Threshold Breach** - When memory usage exceeds configured limits
4. **Latency Threshold Breach** - When latency exceeds configured limits

### Alert Content

Each alert includes:
- Complete system information and metrics
- Trend analysis data
- API and service details
- Mandatory fields for proper routing
- Custom tags and metadata

### Mandatory Fields Validation

The system validates that all required fields are present:
- `team` - OpsGenie team for alert routing
- `environment` - Deployment environment
- `bookmaker_id` - Service/project identifier
- `hostname` - Server hostname
- `business` - Business unit

## Staged Alerting

The staged alerting system provides intelligent alert escalation:

### How It Works

1. **Initial Alert** - Low priority alert sent immediately when circuit trips
2. **Monitoring Period** - System monitors if the issue persists
3. **Escalation** - High priority alert sent if issue continues beyond threshold
4. **Auto-Resolution** - Resolution alert sent if circuit recovers automatically

### Configuration

```toml
[opsgenie]
# Enable staged alerting
time_before_send_alert = 60          # Wait 60 seconds before escalation
initial_alert_priority = "P3"        # Low priority for initial alert
escalated_alert_priority = "P1"      # High priority for escalated alert
```

### Benefits

- **Reduces alert fatigue** by sending low-priority alerts for transient issues
- **Escalates appropriately** for persistent problems
- **Provides context** about issue duration and recovery
- **Prevents alert storms** with intelligent cooldowns

## HTTP API Endpoints

### Breaker Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/breaker/status` | GET | Get detailed breaker status |
| `/breaker/enabled` | GET | Check if breaker is enabled |
| `/breaker/enabled` | POST | Enable the breaker |
| `/breaker/disabled` | POST | Disable the breaker |
| `/breaker/reset` | POST | Reset the breaker |

### Configuration

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/breaker/memory` | GET/POST | Get/set memory threshold |
| `/breaker/latency` | GET/POST | Get/set latency threshold |
| `/breaker/latency-window-size` | GET/POST | Get/set window size |
| `/breaker/percentile` | GET/POST | Get/set percentile |
| `/breaker/wait` | GET/POST | Get/set wait time |
| `/breaker/trend-analysis` | GET/POST | Get/set trend analysis |

### Monitoring

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/breaker/memory-usage` | GET | Current memory usage |
| `/breaker/latencies-above-threshold` | GET | High latencies |
| `/breaker/memory-limit` | GET | Memory limit |
| `/breaker/staged-alerts` | GET | Staged alert status |

### OpsGenie Management

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/breaker/opsgenie/status` | GET | OpsGenie configuration status |
| `/breaker/opsgenie/toggle` | POST | Enable/disable OpsGenie |
| `/breaker/opsgenie/priority` | POST | Update alert priority |
| `/breaker/opsgenie/triggers` | POST | Update alert triggers |
| `/breaker/opsgenie/tags` | POST | Update alert tags |
| `/breaker/opsgenie/cooldown` | POST | Update cooldown period |

## Advanced Features

### Trend Analysis

The circuit breaker includes intelligent trend analysis that considers:

- **Latency patterns** - Increasing, decreasing, or oscillating trends
- **Threshold crossings** - When latencies cross configured limits
- **Plateau detection** - Sustained high latencies
- **Sample requirements** - Minimum data points for reliable analysis

### Memory Monitoring

- **Kubernetes-aware** - Automatically detects container memory limits
- **Precise calculations** - Uses runtime memory statistics
- **Threshold validation** - Prevents invalid configurations
- **Fallback behavior** - Graceful handling when limits can't be determined

### Logging System

Comprehensive logging with:
- **File and line information** - Precise error location tracking
- **Structured logging** - Consistent format and metadata
- **Null-safe operations** - Safe to use with nil loggers
- **Performance optimized** - Minimal overhead in production

## Examples

### Server Integration

```go
package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "time"
    "github.com/lrleon/go-breaker/breaker"
)

func main() {
    // Load configuration
    config, err := breaker.LoadConfig("breakers.toml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    // Create breaker and API handler
    b := breaker.NewBreaker(config, "breakers.toml")
    breakerAPI := breaker.NewBreakerAPI(config)
    
    // Set up router
    router := gin.Default()
    
    // Add protected endpoint
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
        
        ctx.JSON(http.StatusOK, gin.H{"data": "response"})
        
        // Update breaker
        b.Done(startTime, time.Now())
    })
    
    // Add breaker management endpoints
    breaker.AddEndpointToRouter(router, breakerAPI)
    
    router.Run(":8080")
}
```

### Custom Alert Handling

```go
// Initialize with custom OpsGenie configuration
config := &breaker.Config{
    MemoryThreshold:   85.0,
    LatencyThreshold:  1000,
    LatencyWindowSize: 100,
    Percentile:        0.95,
    WaitTime:          30,
    OpsGenie: &breaker.OpsGenieConfig{
        Enabled:     true,
        Team:        "backend-team",
        Environment: "production",
        BookmakerID: "payment-service",
        Priority:    "P1",
        Tags:        []string{"critical", "payment"},
        
        // Staged alerting
        TimeBeforeSendAlert:    120, // 2 minutes
        InitialAlertPriority:   "P3",
        EscalatedAlertPriority: "P1",
        
        // Alert triggers
        TriggerOnOpen:    true,
        TriggerOnReset:   true,
        TriggerOnMemory:  true,
        TriggerOnLatency: true,
    },
}

b := breaker.NewBreaker(config, "breakers.toml")
```

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test suite
go test ./tests/

# Run benchmarks
go test -bench=. ./tests/
```

### Test Server

A comprehensive test server is included for integration testing:

```bash
# Start test server
go run example/server_example.go

# Test with scenarios
curl -X POST http://localhost:8080/test/trigger -d '{"scenario":"high_latency"}'

# Check breaker status
curl http://localhost:8080/breaker/status
```

### OpsGenie Testing

```bash
# Set required environment variables
export OPSGENIE_API_KEY="your-api-key"
export Environment="TEST"

# Run diagnostic script
./example/opsgenie-diag.sh

# Test staged alerts
./example/test_staged_alerts.sh
```

## Utility Scripts

### Ruby Client Scripts

Located in the `example/` directory:

```ruby
# Set server URL
$url = "http://localhost:8080/breaker/"

# Get memory threshold
memory()

# Set latency threshold
set_latency(500)

# Get latencies above threshold
get_latencies(1000)

# Reset breaker
reset()
```

### Bash Management Scripts

```bash
# Get all parameters
./utils/set.sh -e production -a get

# Set memory threshold
./utils/set.sh -e production -a set -m 85

# Set latency threshold  
./utils/set.sh -e production -a set -l 800
```

## Architecture

### Core Components

1. **BreakerDriver** - Main circuit breaker implementation
2. **LatencyWindow** - Sliding window for latency tracking with trend analysis
3. **OpsGenieClient** - Alert management and delivery
4. **StagedAlertManager** - Intelligent alert escalation
5. **Logger** - Structured logging with caller information
6. **BreakerAPI** - HTTP API for management and monitoring

### Design Principles

- **Thread-safe operations** with proper mutex usage
- **Configurable behavior** through TOML files
- **Graceful degradation** when components fail
- **Production-ready** with comprehensive error handling
- **Extensible architecture** for additional integrations

### Memory Management

- **Kubernetes-aware** memory limit detection
- **Efficient sliding windows** with configurable sizes
- **Garbage collection friendly** data structures
- **Memory leak prevention** with proper cleanup

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass (`go test ./...`)
6. Commit your changes (`git commit -m 'Add amazing feature'`)
7. Push to the branch (`git push origin feature/amazing-feature`)
8. Open a Pull Request

### Development Guidelines

- Follow Go conventions and best practices
- Add comprehensive tests for new features
- Update documentation for API changes
- Use meaningful commit messages
- Ensure backward compatibility when possible

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with production-grade reliability in mind
- Inspired by Netflix Hystrix and similar circuit breaker patterns
- Designed for high-performance, low-latency applications
- Integrated with modern alerting and monitoring systems