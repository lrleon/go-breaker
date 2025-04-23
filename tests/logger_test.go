package tests

import (
	"bytes"
	"log"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/lrleon/go-breaker/breaker"
)

// setupTestLogger configures log package to write to a buffer instead of stdout
// Returns the buffer and a cleanup function
func setupTestLogger() (*bytes.Buffer, func()) {
	var buf bytes.Buffer
	originalOutput := log.Writer()
	originalFlags := log.Flags()

	log.SetOutput(&buf)
	log.SetFlags(0) // Remove date/time prefix for easier testing

	cleanup := func() {
		log.SetOutput(originalOutput)
		log.SetFlags(originalFlags)
	}

	return &buf, cleanup
}

// TestNewLogger verifies that a new logger is created with the expected prefix
func TestNewLogger(t *testing.T) {
	prefix := "TestPrefix"
	logger := breaker.NewLogger(prefix)

	if logger.GetPrefix() != prefix {
		t.Errorf("Expected prefix %q, got %q", prefix, logger.GetPrefix())
	}
}

// TestDefaultLogger verifies that the default logger is initialized correctly
func TestDefaultLogger(t *testing.T) {
	if breaker.DefaultLogger == nil {
		t.Error("DefaultLogger should not be nil")
	}

	if breaker.DefaultLogger.GetPrefix() != "" {
		t.Errorf("DefaultLogger should have empty prefix, got %q", breaker.DefaultLogger.GetPrefix())
	}
}

// TestGetCallerInfo verifies that the caller information is correctly extracted
func TestGetCallerInfo(t *testing.T) {
	// Get caller info from this test function
	fileName, line := breaker.GetCallerInfo(1)

	// Get "real" values for comparison
	_, file, expectedLine, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get caller info")
	}
	parts := strings.Split(file, "/")
	expectedFileName := parts[len(parts)-1]

	// Line number should be slightly different
	// getCallerInfo was called a few lines above, so line should be smaller than expectedLine
	if line <= 0 || line > expectedLine {
		t.Errorf("Line number not as expected: got %d, should be > 0 and < %d", line, expectedLine)
	}

	if fileName != expectedFileName {
		t.Errorf("Expected file name %q, got %q", expectedFileName, fileName)
	}
}

// TestLogf verifies that logging includes file and line information
func TestLogf(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("TestPrefix")

	// Capture the current file name for comparison
	_, file, _, _ := runtime.Caller(0)
	parts := strings.Split(file, "/")
	expectedFileName := parts[len(parts)-1]

	logger.Logf("Test message with %s", "formatting")

	output := buf.String()

	// Verify output format - should contain file name, line number, prefix and message
	expectedPattern := regexp.MustCompile(`\[TestPrefix\] \[` + expectedFileName + `:\d+\] Test message with formatting`)
	if !expectedPattern.MatchString(output) {
		t.Errorf("Log output doesn't match expected pattern. Got: %q", output)
	}
}

// TestPrintf verifies that Printf is an alias for Logf
func TestPrintf(t *testing.T) {
	buf1, cleanup1 := setupTestLogger()
	defer cleanup1()

	logger := breaker.NewLogger("TestPrefix")
	logger.Logf("Test message 1")
	output1 := buf1.String()

	buf2, cleanup2 := setupTestLogger()
	defer cleanup2()

	logger.Printf("Test message 1")
	output2 := buf2.String()

	// Both outputs should contain the same file and line info pattern
	fileLinePattern := regexp.MustCompile(`\[TestPrefix\] \[[^:]+:\d+\]`)
	matches1 := fileLinePattern.FindString(output1)
	matches2 := fileLinePattern.FindString(output2)

	if len(matches1) == 0 || len(matches2) == 0 {
		t.Fatalf("Expected both outputs to contain file:line pattern, got: %q and %q", output1, output2)
	}
}

// TestBreakerTriggered verifies the BreakerTriggered method formats correctly
func TestBreakerTriggered(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("TestPrefix")
	logger.BreakerTriggered(100, true, false, 30)

	output := buf.String()

	// Verify that the output contains the expected information
	expectedParts := []string{
		"[TestPrefix]",
		"BreakerDriver triggered",
		"Latency: 100",
		"Memory OK: true",
		"TrendAnalysis: false",
		"Retry after 30 seconds",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected output to contain %q, but got: %q", part, output)
		}
	}
}

// TestBreakerReset verifies the BreakerReset method formats correctly
func TestBreakerReset(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("TestPrefix")
	logger.BreakerReset()

	output := buf.String()

	if !strings.Contains(output, "BreakerDriver has been reset") {
		t.Errorf("Expected output to contain reset message, but got: %q", output)
	}
}

// TestLatencyInfo verifies the LatencyInfo method formats correctly
func TestLatencyInfo(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("TestPrefix")
	logger.LatencyInfo(150, 100, true)

	output := buf.String()

	// Verify that the output contains the expected information
	expectedParts := []string{
		"Current latency percentile: 150",
		"Threshold: 100",
		"Above threshold: true",
	}

	for _, part := range expectedParts {
		if !strings.Contains(output, part) {
			t.Errorf("Expected output to contain %q, but got: %q", part, output)
		}
	}
}

// TestTrendAnalysisInfo verifies the TrendAnalysisInfo method formats correctly
func TestTrendAnalysisInfo(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("TestPrefix")
	logger.TrendAnalysisInfo(true)

	output := buf.String()

	if !strings.Contains(output, "Trend analysis enabled. Positive trend detected: true") {
		t.Errorf("Expected output to contain trend analysis message, but got: %q", output)
	}
}

// TestNestedCallers tests how the logger behaves when used from nested function calls
func TestNestedCallers(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("Nested")

	// Define a sequence of nested functions
	level3 := func() {
		logger.Logf("This is level 3")
	}

	level2 := func() {
		logger.Logf("This is level 2")
		level3()
	}

	level1 := func() {
		logger.Logf("This is level 1")
		level2()
	}

	// Execute the nested calls
	level1()

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Should have 3 lines (with a trailing empty line)
	if len(lines) != 4 {
		t.Fatalf("Expected 4 lines (including trailing empty line), got %d: %q", len(lines), output)
	}

	// Each line should contain "This is level X"
	for i, level := range []string{"1", "2", "3"} {
		expected := "This is level " + level
		if !strings.Contains(lines[i], expected) {
			t.Errorf("Line %d should contain %q, but got: %q", i, expected, lines[i])
		}
	}

	// Ensure each line contains a file:line pattern
	pattern := regexp.MustCompile(`\[[^\/]+\.go:\d+\]`)
	for i, line := range lines[:3] {
		if !pattern.MatchString(line) {
			t.Errorf("Line %d should contain a file:line pattern, but got: %q", i, line)
		}
	}
}

// TestFormatInjection ensures the logger is resistant to format string injection
func TestFormatInjection(t *testing.T) {
	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("TestPrefix")

	// A string that could potentially cause problems with formatting
	evilString := "%s %d %v %T"
	logger.Logf("Testing with potential format injection: %s", evilString)

	output := buf.String()

	// The evil string should be output as-is, not interpreted as format
	if !strings.Contains(output, evilString) {
		t.Errorf("Expected output to contain the exact format string, got: %q", output)
	}
}

// TestLogfWithMultipleParallelGoroutines ensures the logger works correctly in parallel
func TestLogfWithMultipleParallelGoroutines(t *testing.T) {
	// This test will verify that the logger can be used from multiple goroutines
	// Note: We can't easily test the exact output since goroutines execute in
	// non-deterministic order, but we can check that the program doesn't crash
	// and that the expected number of log lines is generated

	buf, cleanup := setupTestLogger()
	defer cleanup()

	logger := breaker.NewLogger("Parallel")
	done := make(chan bool)

	// Launch 10 goroutines that each log a message
	const numGoroutines = 10
	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			logger.Logf("Message from goroutine %d", index)
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Should have numGoroutines lines plus a trailing empty line
	if len(lines) != numGoroutines+1 {
		t.Errorf("Expected %d lines (including trailing empty line), got %d", numGoroutines+1, len(lines))
	}

	// Each line should contain "Message from goroutine" and a number
	pattern := regexp.MustCompile(`Message from goroutine \d+`)
	count := 0
	for _, line := range lines[:numGoroutines] {
		if pattern.MatchString(line) {
			count++
		}
	}

	if count != numGoroutines {
		t.Errorf("Expected %d messages from goroutines, found %d", numGoroutines, count)
	}
}

// TestWithNilLogger ensures that method calls on a nil logger don't panic
func TestWithNilLogger(t *testing.T) {
	// This test ensures that if someone tries to use methods on a nil Logger,
	// the program doesn't panic

	var logger *breaker.Logger

	// These should not panic, though they won't output anything useful
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Logf panicked with nil logger: %v", r)
			}
		}()

		logger.Logf("This should not panic")
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Printf panicked with nil logger: %v", r)
			}
		}()

		logger.Printf("This should not panic")
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("BreakerTriggered panicked with nil logger: %v", r)
			}
		}()

		logger.BreakerTriggered(100, true, false, 30)
	}()
}
