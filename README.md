```markdown
# Breaker Package

The `breaker` package provides a circuit breaker implementation in Go. It helps to prevent system overload by monitoring memory usage and latency, and tripping the breaker when thresholds are exceeded.

## Interface

The `Breaker` interface defines the following methods:

```go
type Breaker interface {
    Allow() bool                       // Returns if the operation can continue and updates the state of the Breaker
    Done(startTime, endTime time.Time) // Reports the latency of an operation finished
    Triggered() bool                   // Indicate if the breaker is activated
    Reset()                            // Restores the state of Breaker
}
```

## Implementation

The `breaker` struct implements the `Breaker` interface and includes the following fields:

- `mu`: A mutex to ensure thread-safe operations.
- `Config`: Configuration settings for the breaker.
- `tripped`: A boolean indicating if the breaker is tripped.
- `lastTripTime`: The last time the breaker was tripped.
- `latencyWindow`: A window to track latencies.

## Methods

### NewBreaker

Creates a new breaker with the given configuration.

```go
func NewBreaker(Config Config) Breaker
```

### Allow

Checks if the operation can continue based on the current state of the breaker.

```go
func (b *breaker) Allow() bool
```

### Done

Reports the latency of a finished operation.

```go
func (b *breaker) Done(startTime, endTime time.Time)
```

### Triggered

Indicates if the breaker is currently tripped.

```go
func (b *breaker) Triggered() bool
```

### Reset

Resets the state of the breaker.

```go
func (b *breaker) Reset()
```

## Configuration

The `Config` struct is used to configure the breaker. It includes the following fields:

- `MemoryThreshold`: The memory usage threshold as a fraction of the memory limit.
- `LatencyThreshold`: The latency threshold in milliseconds.
- `LatencyWindowSize`: The size of the latency window.
- `Percentile`: The percentile of latencies to consider.
- `WaitTime`: The time to wait before allowing operations after the breaker is tripped.

## Example Usage

```go
package main

import (
    "time"
    "breaker"
)

func main() {
    Config := breaker.Config{
        MemoryThreshold:   0.85,
        LatencyThreshold:  600,
        LatencyWindowSize: 10,
        Percentile:        0.95,
        WaitTime:          10,
    }

    b := breaker.NewBreaker(Config)

    // Simulate an operation
    startTime := time.Now()
    time.Sleep(500 * time.Millisecond) // Simulate latency
    endTime := time.Now()

    b.Done(startTime, endTime)

    if b.Triggered() {
        println("Breaker is triggered")
    } else {
        println("Breaker is not triggered")
    }
}
```

## Testing

The package includes tests for the breaker functionality and memory limit handling. The tests can be run using the `go test` command.

```sh
go test ./...
```

## License

This project is licensed under the MIT License.
```