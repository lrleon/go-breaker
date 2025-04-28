package breaker

import (
	"fmt"
	"log"
	"runtime"
	"strings"
)

// Logger is a wrapper around the standard log package that adds file and line information
// to each log message in a portable way.
type Logger struct {
	prefix string
}

// NewLogger creates a new Logger with the given prefix
func NewLogger(prefix string) *Logger {
	return &Logger{
		prefix: prefix,
	}
}

// DefaultLogger is the default logger instance
var DefaultLogger = NewLogger("")

// GetCallerInfo returns the file name and line number of the caller
// skipFrames is the number of stack frames to skip (1 would be the caller of getCallerInfo)
// This is exported for testing purposes
func GetCallerInfo(skipFrames int) (fileName string, line int) {
	// Get the caller's information
	_, file, line, ok := runtime.Caller(skipFrames)
	if !ok {
		return "unknown", 0
	}

	// Extract just the filename from the full path
	parts := strings.Split(file, "/")
	fileName = parts[len(parts)-1]

	return fileName, line
}

// findCallerOutsideLogger skips frames until finding one that's not within logger.go
// This is useful when the logger is used from external packages
func findCallerOutsideLogger(initialSkip int) (fileName string, line int) {
	// Start from the initial skip and increment until we find a frame outside logger.go
	for skip := initialSkip; skip < initialSkip+10; skip++ { // Limit to 10 frames to avoid infinite loop
		_, file, lineNo, ok := runtime.Caller(skip)
		if !ok {
			return "unknown", 0
		}

		// Extract just the filename from the full path
		parts := strings.Split(file, "/")
		name := parts[len(parts)-1]

		// If this frame is not from logger.go, return it
		if name != "logger.go" {
			return name, lineNo
		}
	}

	// If we couldn't find a non-logger.go frame, return the initial one
	return GetCallerInfo(initialSkip)
}

// getCallerInfo is an internal version for use within the package
func getCallerInfo(skipFrames int) (fileName string, line int) {
	return findCallerOutsideLogger(skipFrames + 1) // +1 to skip this function call
}

// GetPrefix returns the prefix of the logger
// This is exported for testing purposes
func (l *Logger) GetPrefix() string {
	if l == nil {
		return ""
	}
	return l.prefix
}

// Logf formats and logs a message with file and line information
// It's safe to call this method on a nil Logger (becomes a no-op)
func (l *Logger) Logf(format string, args ...interface{}) {
	if l == nil {
		return
	}

	fileName, line := getCallerInfo(2) // Skip 2 frames: getCallerInfo and Logf
	prefix := fmt.Sprintf("[%s:%d] ", fileName, line)
	if l.prefix != "" {
		prefix = fmt.Sprintf("[%s] %s", l.prefix, prefix)
	}
	log.Printf(prefix+format, args...)
}

// Printf is an alias for Logf for compatibility with log.Printf
// It's safe to call this method on a nil Logger (becomes a no-op)
func (l *Logger) Printf(format string, args ...interface{}) {
	if l == nil {
		return
	}
	l.Logf(format, args...)
}

// BreakerTriggered logs a message when the breaker is triggered
// It's safe to call this method on a nil Logger (becomes a no-op)
func (l *Logger) BreakerTriggered(latency int64, memoryOK bool, trendAnalysisEnabled bool, waitTime int) {
	if l == nil {
		return
	}

	fileName, line := getCallerInfo(2)
	l.Logf("BreakerDriver triggered at [%s:%d]. Latency: %v, Memory OK: %v, TrendAnalysis: %v",
		fileName, line, latency, memoryOK, trendAnalysisEnabled)
	l.Logf("Retry after %v seconds", waitTime)
}

// BreakerReset logs a message when the breaker is reset
// It's safe to call this method on a nil Logger (becomes a no-op)
func (l *Logger) BreakerReset() {
	if l == nil {
		return
	}

	fileName, line := getCallerInfo(2)
	l.Logf("BreakerDriver has been reset at [%s:%d]", fileName, line)
}

// LatencyInfo logs information about current latency
// It's safe to call this method on a nil Logger (becomes a no-op)
func (l *Logger) LatencyInfo(currentLatency, threshold int64, aboveThreshold bool) {
	if l == nil {
		return
	}

	fileName, line := getCallerInfo(2)
	if aboveThreshold {
		l.Logf("At [%s:%d] - Current latency percentile: %v, Threshold: %v, Above threshold: %v",
			fileName, line, currentLatency, threshold, aboveThreshold)
		return
	}
}

// TrendAnalysisInfo logs information about trend analysis
// It's safe to call this method on a nil Logger (becomes a no-op)
func (l *Logger) TrendAnalysisInfo(hasTrend bool) {
	if l == nil {
		return
	}

	fileName, line := getCallerInfo(2)
	l.Logf("At [%s:%d] - Trend analysis enabled. Positive trend detected: %v",
		fileName, line, hasTrend)
}
