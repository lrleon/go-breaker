#!/bin/bash

# Automated testing script for Circuit Breaker + OpsGenie
# Usage: ./test_opsgenie.sh [test_name]

set -e

SERVER_URL="http://localhost:8080"
SLEEP_TIME=3

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Utility functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_step() {
    echo -e "\n${BLUE}🔹 $1${NC}"
}

# Check if the server is running
check_server() {
    log_step "Checking if test server is running..."
    if ! curl -s $SERVER_URL/status > /dev/null; then
        log_error "Test server is not running on $SERVER_URL"
        log_info "Start the server with: go run enhanced_test_server.go"
        exit 1
    fi
    log_success "Test server is running"
}

# Check environment variables
check_environment() {
    log_step "Checking environment variables..."

    if [ -z "$OPSGENIE_API_KEY" ]; then
        log_error "OPSGENIE_API_KEY environment variable is not set"
        log_info "Set it with: export OPSGENIE_API_KEY='your-api-key'"
        exit 1
    fi
    log_success "OPSGENIE_API_KEY is set"

    if [ -z "$Environment" ]; then
        log_warning "Environment variable is not set, will use config default"
    else
        log_success "Environment is set to: $Environment"
    fi
}

# Test 1: Initial validation
test_initial_validation() {
    log_step "Test 1: Initial validation"

    # System status
    echo "📊 System Status:"
    curl -s $SERVER_URL/status | jq '.circuit_breaker, .opsgenie.enabled, .opsgenie.validation'

    # OpsGenie validation
    echo -e "\n🔍 OpsGenie Validation:"
    validation_result=$(curl -s $SERVER_URL/opsgenie/validate)
    echo $validation_result | jq

    if echo $validation_result | jq -e '.status == "success"' > /dev/null; then
        log_success "OpsGenie validation passed"
    else
        log_error "OpsGenie validation failed"
        return 1
    fi

    # Connection test
    echo -e "\n🌐 Connection Test:"
    connection_result=$(curl -s $SERVER_URL/opsgenie/test-connection)
    echo $connection_result | jq

    if echo $connection_result | jq -e '.status == "success"' > /dev/null; then
        log_success "OpsGenie connection test passed"
    else
        log_error "OpsGenie connection test failed"
        return 1
    fi
}

# Test 2: Manual test alert
test_manual_alert() {
    log_step "Test 2: Manual test alert"

    echo "📤 Sending manual test alert..."
    result=$(curl -s -X POST $SERVER_URL/opsgenie/send-test-alert)
    echo $result | jq

    if echo $result | jq -e '.status == "success"' > /dev/null; then
        log_success "Manual test alert sent successfully"
        log_info "Check your OpsGenie dashboard for the alert"
    else
        log_error "Failed to send manual test alert"
        return 1
    fi

    sleep $SLEEP_TIME
}

# Test 3: Latency-based circuit breaker
test_latency_circuit_breaker() {
    log_step "Test 3: Latency-based circuit breaker"

    # Activate high latency
    echo "🐌 Activating high latency scenario..."
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "high_latency"}' | jq

    sleep 2

    # Make multiple requests
    echo -e "\n🔄 Making multiple requests to trigger circuit breaker..."
    for i in {1..8}; do
        echo "Request $i:"
        response=$(curl -s $SERVER_URL/test)
        echo $response | jq '.actual_latency_ms, .breaker_status.triggered'

        # Check if circuit breaker triggered
        if echo $response | jq -e '.breaker_status.triggered == true' > /dev/null; then
            log_success "Circuit breaker triggered by latency!"
            break
        fi
        sleep 1
    done

    # Check final status
    echo -e "\n📊 Final breaker status:"
    curl -s $SERVER_URL/breaker/status | jq '.triggered, .latency_ok, .current_percentile_ms'

    sleep $SLEEP_TIME
}

# Test 4: Memory-based circuit breaker
test_memory_circuit_breaker() {
    log_step "Test 4: Memory-based circuit breaker"

    # Reset first
    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null

    # Activate memory overload
    echo "🧠 Activating memory overload scenario..."
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "memory_overload"}' | jq

    sleep 2

    # Make request to activate memory check
    echo -e "\n🔄 Making request to trigger memory check..."
    response=$(curl -s $SERVER_URL/test)
    echo $response | jq '.breaker_status'

    # Check memory status
    echo -e "\n📊 Memory status:"
    curl -s $SERVER_URL/breaker/status | jq '.memory_ok, .current_memory_usage_mb, .memory_threshold_percent'

    sleep $SLEEP_TIME
}

# Test 5: Circuit breaker reset
test_circuit_breaker_reset() {
    log_step "Test 5: Circuit breaker reset"

    echo "🔄 Resetting circuit breaker..."
    reset_result=$(curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}')
    echo $reset_result | jq

    sleep 2

    # Check that it was reset
    echo -e "\n📊 Post-reset status:"
    curl -s $SERVER_URL/breaker/status | jq '.triggered, .enabled'

    # Restore normal conditions
    echo -e "\n✅ Restoring normal conditions..."
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "reset_normal"}' | jq

    sleep $SLEEP_TIME
}

# Test 6: Trend analysis
test_trend_analysis() {
    log_step "Test 6: Trend analysis"

    # Reset first
    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null

    # Activate increasing latency pattern
    echo "📈 Activating latency spike pattern..."
    curl -s -X POST $SERVER_URL/test/delay \
        -H "Content-Type: application/json" \
        -d '{"delay": "spike"}' | jq

    sleep 2

    # Make requests with increasing latency
    echo -e "\n🔄 Making requests with increasing latency..."
    for i in {1..10}; do
        echo "Request $i:"
        response=$(curl -s $SERVER_URL/test)
        latency=$(echo $response | jq '.actual_latency_ms')
        triggered=$(echo $response | jq '.breaker_status.triggered')
        echo "  Latency: ${latency}ms, Triggered: $triggered"

        if [ "$triggered" = "true" ]; then
            log_success "Circuit breaker triggered by trend analysis!"
            break
        fi
        sleep 1
    done

    # Check trend analysis
    echo -e "\n📊 Trend analysis status:"
    curl -s $SERVER_URL/breaker/status | jq '.has_positive_trend, .trend_analysis_enabled'

    sleep $SLEEP_TIME
}

# Test 7: Different alert types
test_alert_types() {
    log_step "Test 7: Different alert types"

    alert_types=("circuit-open" "circuit-reset" "memory-threshold" "latency-threshold")

    for alert_type in "${alert_types[@]}"; do
        echo "📤 Sending $alert_type alert..."
        result=$(curl -s -X POST $SERVER_URL/opsgenie/send-test-alert \
            -H "Content-Type: application/json" \
            -d "{\"alert_type\": \"$alert_type\"}")

        if echo $result | jq -e '.status == "success"' > /dev/null; then
            log_success "$alert_type alert sent"
        else
            log_warning "$alert_type alert failed or in cooldown"
        fi

        sleep 2
    done
}

# Test 8: Cooldown verification
test_cooldown() {
    log_step "Test 8: Cooldown verification"

    echo "📤 Sending multiple alerts to test cooldown..."
    for i in {1..3}; do
        echo "Alert attempt $i:"
        result=$(curl -s -X POST $SERVER_URL/opsgenie/send-test-alert)
        status=$(echo $result | jq -r '.status')
        message=$(echo $result | jq -r '.message')
        echo "  Status: $status"
        echo "  Message: $message"
        sleep 3
    done

    log_info "Check server logs for cooldown messages"
}

# Main function
run_all_tests() {
    echo -e "${BLUE}🧪 OPSGENIE CIRCUIT BREAKER TESTING SUITE${NC}"
    echo -e "${BLUE}==========================================${NC}\n"

    check_server
    check_environment

    # Run all tests
    test_initial_validation || log_error "Initial validation failed"
    test_manual_alert || log_error "Manual alert test failed"
    test_latency_circuit_breaker || log_error "Latency circuit breaker test failed"
    test_memory_circuit_breaker || log_error "Memory circuit breaker test failed"
    test_circuit_breaker_reset || log_error "Circuit breaker reset test failed"
    test_trend_analysis || log_error "Trend analysis test failed"
    test_alert_types || log_error "Alert types test failed"
    test_cooldown || log_error "Cooldown test failed"

    echo -e "\n${GREEN}🎉 ALL TESTS COMPLETED!${NC}"
    echo -e "${BLUE}📋 Next steps:${NC}"
    echo "1. Check your OpsGenie dashboard for all alerts"
    echo "2. Verify each alert has all mandatory fields:"
    echo "   - Team, Environment, BookmakerId, Host, Business"
    echo "   - Tags: env:test, environment:test, bookmaker:*, host:*, business:*, team:*"
    echo "3. Close alerts one by one in OpsGenie"
    echo "4. Review server logs for any errors or warnings"
}

# Function for individual test
run_single_test() {
    local test_name=$1

    check_server
    check_environment

    case $test_name in
        "validation"|"1")
            test_initial_validation
            ;;
        "manual"|"2")
            test_manual_alert
            ;;
        "latency"|"3")
            test_latency_circuit_breaker
            ;;
        "memory"|"4")
            test_memory_circuit_breaker
            ;;
        "reset"|"5")
            test_circuit_breaker_reset
            ;;
        "trend"|"6")
            test_trend_analysis
            ;;
        "alerts"|"7")
            test_alert_types
            ;;
        "cooldown"|"8")
            test_cooldown
            ;;
        *)
            echo "Available tests:"
            echo "  validation (1) - Initial validation"
            echo "  manual (2)     - Manual test alert"
            echo "  latency (3)    - Latency circuit breaker"
            echo "  memory (4)     - Memory circuit breaker"
            echo "  reset (5)      - Circuit breaker reset"
            echo "  trend (6)      - Trend analysis"
            echo "  alerts (7)     - Different alert types"
            echo "  cooldown (8)   - Cooldown verification"
            echo ""
            echo "Example: ./test_opsgenie.sh validation"
            echo "Example: ./test_opsgenie.sh 3"
            exit 1
            ;;
    esac
}

# Script entry point
if [ $# -eq 0 ]; then
    run_all_tests
else
    run_single_test $1
fi