#!/bin/bash

# Enhanced testing script for Staged Alerting System
# Usage: ./test_staged_alerts.sh

set -e

SERVER_URL="http://localhost:8080"
SLEEP_TIME=3

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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
    echo -e "\n${PURPLE}🔹 $1${NC}"
}

log_escalation() {
    echo -e "${YELLOW}🚨 $1${NC}"
}

# Check if the server is running
check_server() {
    log_step "Checking if test server is running..."
    if ! curl -s $SERVER_URL/status > /dev/null; then
        log_error "Test server is not running on $SERVER_URL"
        log_info "Start the server with: go run example/server_example.go"
        exit 1
    fi
    log_success "Test server is running"
}

# Get staged alerts configuration
get_staged_config() {
    log_step "Getting staged alerts configuration..."

    config=$(curl -s $SERVER_URL/breaker/staged-alerts)
    enabled=$(echo $config | jq -r '.enabled')
    time_before_alert=$(echo $config | jq -r '.time_before_alert')
    initial_priority=$(echo $config | jq -r '.initial_priority')
    escalated_priority=$(echo $config | jq -r '.escalated_priority')

    echo "📋 Staged Alerts Configuration:"
    echo "   Enabled: $enabled"
    echo "   Time before escalation: $time_before_alert seconds"
    echo "   Initial priority: $initial_priority"
    echo "   Escalated priority: $escalated_priority"

    if [ "$enabled" = "false" ]; then
        log_error "Staged alerting is not enabled! Check your configuration."
        log_info "Set 'time_before_send_alert > 0' in your opsgenie configuration"
        exit 1
    fi

    if [ "$time_before_alert" = "0" ] || [ "$time_before_alert" = "null" ]; then
        log_error "Staged alerting time is not configured!"
        exit 1
    fi

    log_success "Staged alerting is properly configured"
    echo "ESCALATION_TIME=$time_before_alert"

    # Export for use in other functions
    export ESCALATION_TIME=$time_before_alert
}

# Monitor pending alerts
monitor_pending_alerts() {
    local max_checks=$1
    local check_interval=$2

    for i in $(seq 1 $max_checks); do
        status=$(curl -s $SERVER_URL/breaker/staged-alerts)
        pending_count=$(echo $status | jq -r '.pending_alerts_count')

        echo "🔍 Check $i/$max_checks: Pending alerts = $pending_count"

        if [ "$pending_count" != "0" ] && [ "$pending_count" != "null" ]; then
            echo "📊 Pending alerts details:"
            echo $status | jq '.pending_alerts'
        fi

        sleep $check_interval
    done
}

# Test 1: Staged Alert Flow - Complete Escalation
test_staged_alert_complete_escalation() {
    log_step "Test 1: Complete Staged Alert Flow (with Escalation)"

    # Reset breaker first
    log_info "Resetting circuit breaker..."
    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null

    # Trigger high latency to activate circuit breaker
    log_info "Activating high latency scenario..."
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "high_latency"}' | jq '.'

    sleep 2

    # Make requests to trigger the breaker
    log_info "Making requests to trigger circuit breaker..."
    for i in {1..5}; do
        echo "Request $i:"
        response=$(curl -s $SERVER_URL/test)
        triggered=$(echo $response | jq -r '.breaker_status.triggered')
        latency=$(echo $response | jq -r '.actual_latency_ms')
        echo "  Latency: ${latency}ms, Triggered: $triggered"

        if [ "$triggered" = "true" ]; then
            log_success "Circuit breaker triggered! Staged alerting should have started."
            break
        fi
        sleep 1
    done

    # Check initial staged alert status
    log_info "Checking staged alerts status..."
    status=$(curl -s $SERVER_URL/breaker/staged-alerts)
    pending_count=$(echo $status | jq -r '.pending_alerts_count')

    if [ "$pending_count" = "0" ] || [ "$pending_count" = "null" ]; then
        log_warning "No pending alerts found. Staged alerting might not be working."
    else
        log_success "Found $pending_count pending alert(s)"
        echo "📊 Alert details:"
        echo $status | jq '.pending_alerts'
    fi

    # Wait for escalation time + buffer
    escalation_wait=$((ESCALATION_TIME + 10))
    log_escalation "Waiting $escalation_wait seconds for escalation (configured: ${ESCALATION_TIME}s + 10s buffer)..."
    log_info "During this time, the system should escalate if the breaker remains triggered."

    # Monitor pending alerts during wait
    echo "🔍 Monitoring alerts during escalation period..."
    monitor_pending_alerts $((escalation_wait / 5)) 5

    # Check final status
    log_info "Checking final staged alerts status..."
    final_status=$(curl -s $SERVER_URL/breaker/staged-alerts)
    final_pending=$(echo $final_status | jq -r '.pending_alerts_count')

    echo "📊 Final status:"
    echo "   Pending alerts: $final_pending"
    echo $final_status | jq '.'

    # Check if breaker is still triggered
    breaker_status=$(curl -s $SERVER_URL/breaker/status)
    still_triggered=$(echo $breaker_status | jq -r '.triggered')

    if [ "$still_triggered" = "true" ]; then
        log_escalation "Circuit breaker is still triggered - escalation should have occurred!"
    else
        log_success "Circuit breaker recovered automatically"
    fi
}

# Test 2: Staged Alert Flow - Auto Recovery
test_staged_alert_auto_recovery() {
    log_step "Test 2: Staged Alert Flow (with Auto Recovery)"

    # Reset breaker first
    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null

    # Trigger latency spike
    log_info "Triggering latency spike pattern..."
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "latency_spike"}' > /dev/null

    # Make some requests to trigger
    log_info "Making requests to trigger circuit breaker..."
    for i in {1..3}; do
        curl -s $SERVER_URL/test > /dev/null
        sleep 1
    done

    # Check if triggered
    status=$(curl -s $SERVER_URL/breaker/status)
    triggered=$(echo $status | jq -r '.triggered')

    if [ "$triggered" = "true" ]; then
        log_success "Circuit breaker triggered for auto-recovery test"

        # Immediately restore normal conditions
        log_info "Restoring normal conditions for auto-recovery..."
        curl -s -X POST $SERVER_URL/test/trigger \
            -H "Content-Type: application/json" \
            -d '{"scenario": "reset_normal"}' > /dev/null

        # Wait a bit less than escalation time
        recovery_wait=$((ESCALATION_TIME - 5))
        log_info "Waiting $recovery_wait seconds (less than escalation time) for auto-recovery..."

        # Monitor during recovery
        monitor_pending_alerts $((recovery_wait / 3)) 3

        # Check if recovered
        final_status=$(curl -s $SERVER_URL/breaker/status)
        recovered=$(echo $final_status | jq -r '.triggered')

        if [ "$recovered" = "false" ]; then
            log_success "Circuit breaker auto-recovered before escalation!"
        else
            log_warning "Circuit breaker did not auto-recover as expected"
        fi
    else
        log_warning "Circuit breaker was not triggered for auto-recovery test"
    fi
}

# Test 3: Manual Reset During Staging
test_manual_reset_during_staging() {
    log_step "Test 3: Manual Reset During Staging Period"

    # Reset and trigger again
    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null

    # Trigger high latency
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "high_latency"}' > /dev/null

    # Make requests to trigger
    for i in {1..3}; do
        curl -s $SERVER_URL/test > /dev/null
        sleep 1
    done

    # Check staged alerts
    status=$(curl -s $SERVER_URL/breaker/staged-alerts)
    pending_before=$(echo $status | jq -r '.pending_alerts_count')

    log_info "Pending alerts before manual reset: $pending_before"

    # Wait a bit, then manually reset
    sleep 5
    log_info "Performing manual reset during staging period..."
    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null

    # Check staged alerts after reset
    sleep 2
    final_status=$(curl -s $SERVER_URL/breaker/staged-alerts)
    pending_after=$(echo $final_status | jq -r '.pending_alerts_count')

    log_info "Pending alerts after manual reset: $pending_after"

    if [ "$pending_after" = "0" ] || [ "$pending_after" = "null" ]; then
        log_success "Manual reset correctly cleared pending alerts"
    else
        log_warning "Manual reset did not clear pending alerts as expected"
    fi
}

# Main execution
main() {
    echo -e "${BLUE}🧪 STAGED ALERTING SYSTEM TESTING SUITE${NC}"
    echo -e "${BLUE}=======================================${NC}\n"

    check_server
    get_staged_config

    echo -e "\n${PURPLE}🚀 Starting Staged Alerting Tests...${NC}\n"

    test_staged_alert_complete_escalation
    echo -e "\n${BLUE}─────────────────────────────────────────${NC}\n"

    test_staged_alert_auto_recovery
    echo -e "\n${BLUE}─────────────────────────────────────────${NC}\n"

    test_manual_reset_during_staging

    echo -e "\n${GREEN}🎉 STAGED ALERTING TESTS COMPLETED!${NC}"
    echo -e "${BLUE}📋 Next steps:${NC}"
    echo "1. Check your OpsGenie dashboard for staged alerts"
    echo "2. Verify initial alerts have low priority (P3/P4)"
    echo "3. Verify escalated alerts have high priority (P1/P2)"
    echo "4. Check timing matches your configuration"
    echo "5. Review server logs for staging details"

    # Final cleanup
    log_info "Performing final cleanup..."
    curl -s -X POST $SERVER_URL/test/trigger \
        -H "Content-Type: application/json" \
        -d '{"scenario": "reset_normal"}' > /dev/null

    curl -s -X POST $SERVER_URL/breaker/reset \
        -H "Content-Type: application/json" \
        -d '{"confirm": true}' > /dev/null
}

# Run main function
main "$@"