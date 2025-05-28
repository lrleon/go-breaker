#!/bin/bash

# Diagnostic script for staged alerts
# Usage: ./debug_staged_alerts.sh

SERVER_URL="http://localhost:8080"
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
log_success() { echo -e "${GREEN}✅ $1${NC}"; }
log_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
log_error() { echo -e "${RED}❌ $1${NC}"; }
log_step() { echo -e "\n${BLUE}🔹 $1${NC}"; }

# Step-by-step diagnosis
main() {
    echo -e "${BLUE}🔍 STAGED ALERTS DIAGNOSIS${NC}"
    echo -e "${BLUE}====================================${NC}\n"

    # Step 1: Verify server
    log_step "1. Verifying server..."
    if curl -s $SERVER_URL/status > /dev/null; then
        log_success "Server is working"
    else
        log_error "Server not responding"
        exit 1
    fi

    # Step 2: Verify basic OpsGenie
    log_step "2. Verifying OpsGenie..."
    opsgenie_status=$(curl -s $SERVER_URL/opsgenie/test-connection)
    if echo $opsgenie_status | jq -e '.status == "success"' > /dev/null 2>&1; then
        log_success "OpsGenie connected"
    else
        log_error "OpsGenie NOT connected"
        echo $opsgenie_status | jq '.'
        exit 1
    fi

    # Step 3: Verify staged alerts configuration
    log_step "3. Verifying staged alerts configuration..."
    staged_config=$(curl -s $SERVER_URL/breaker/staged-alerts)
    echo $staged_config | jq '.'

    enabled=$(echo $staged_config | jq -r '.enabled')
    if [ "$enabled" = "true" ]; then
        log_success "Staged alerts enabled"
    else
        log_error "Staged alerts NOT enabled"
        exit 1
    fi

    # Step 4: Test manual alert
    log_step "4. Sending manual test alert..."
    manual_alert=$(curl -s -X POST $SERVER_URL/opsgenie/send-test-alert)
    echo $manual_alert | jq '.'

    if echo $manual_alert | jq -e '.status == "success"' > /dev/null 2>&1; then
        log_success "Manual alert sent - Can you see it in OpsGenie?"
        read -p "Did you receive the test alert in OpsGenie? (y/n): " manual_received
        if [ "$manual_received" != "y" ]; then
            log_error "Problem with basic OpsGenie configuration"
            log_info "Check: team, permissions, notifications"
            exit 1
        fi
    else
        log_error "Failed to send manual alert"
        exit 1
    fi

    # Step 5: Reset state
    log_step "5. Resetting circuit breaker..."
    curl -s -X POST $SERVER_URL/breaker/reset -d '{"confirm": true}' > /dev/null
    curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "reset_normal"}' > /dev/null
    log_success "State reset"

    # Step 6: Configure for easy activation
    log_step "6. Configuring low thresholds for easy activation..."
    curl -s -X POST $SERVER_URL/breaker/latency -d '{"threshold": 200}' > /dev/null
    curl -s -X POST $SERVER_URL/breaker/opsgenie/cooldown -d '{"cooldown_seconds": 5}' > /dev/null
    log_success "Thresholds configured"

    # Step 7: Activate high latency
    log_step "7. Activating high latency scenario..."
    curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "high_latency"}' > /dev/null
    log_success "High latency activated"

    # Step 8: Trigger circuit breaker
    log_step "8. Triggering circuit breaker..."
    for i in {1..8}; do
        echo -n "Request $i: "
        response=$(curl -s $SERVER_URL/test)
        latency=$(echo $response | jq -r '.actual_latency_ms')
        triggered=$(echo $response | jq -r '.breaker_status.triggered')
        echo "${latency}ms (triggered: $triggered)"

        if [ "$triggered" = "true" ]; then
            log_success "Circuit breaker ACTIVATED!"
            break
        fi
        sleep 1
    done

    # Verify activation
    breaker_status=$(curl -s $SERVER_URL/breaker/status)
    is_triggered=$(echo $breaker_status | jq -r '.triggered')

    if [ "$is_triggered" != "true" ]; then
        log_error "Circuit breaker was NOT activated"
        echo "Current status:"
        echo $breaker_status | jq '.triggered, .current_percentile_ms, .latency_threshold_ms'
        exit 1
    fi

    # Step 9: Check pending alerts
    log_step "9. Checking pending alerts immediately..."
    sleep 2  # Give time to process
    pending_status=$(curl -s $SERVER_URL/breaker/staged-alerts)
    echo $pending_status | jq '.'

    pending_count=$(echo $pending_status | jq -r '.pending_alerts_count')
    if [ "$pending_count" = "0" ] || [ "$pending_count" = "null" ]; then
        log_error "NO pending alerts - something is wrong"
        log_info "Check server logs for errors"
        exit 1
    else
        log_success "$pending_count pending alert(s) found"
    fi

    # Step 10: Monitor escalation
    log_step "10. Monitoring escalation..."
    escalation_time=$(echo $staged_config | jq -r '.time_before_alert')
    log_info "Waiting $escalation_time seconds for escalation..."

    # Monitor every 10 seconds
    for i in $(seq 10 10 $((escalation_time + 20))); do
        echo "⏰ Second $i of $escalation_time..."
        status=$(curl -s $SERVER_URL/breaker/staged-alerts)
        pending=$(echo $status | jq -r '.pending_alerts_count')
        echo "   Pending alerts: $pending"

        if [ $i -gt $escalation_time ]; then
            # Check if escalated
            escalated=$(echo $status | jq -r '.pending_alerts | to_entries[0].value.escalated_alert_sent // false')
            if [ "$escalated" = "true" ]; then
                log_success "ESCALATION DETECTED!"
                break
            fi
        fi

        sleep 10
    done

    # Step 11: Final check
    log_step "11. Final verification..."
    final_status=$(curl -s $SERVER_URL/breaker/staged-alerts)
    echo $final_status | jq '.'

    log_info "Did you receive 2 alerts in OpsGenie?"
    log_info "- First: Priority P3 (initial)"
    log_info "- Second: Priority P2 (escalated)"

    read -p "Did you receive both alerts? (y/n): " both_received
    if [ "$both_received" = "y" ]; then
        log_success "SUCCESS! Staged alerts are working correctly"
    else
        log_warning "Check your OpsGenie dashboard and notification settings"
    fi

    # Cleanup
    log_step "12. Cleanup..."
    curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "reset_normal"}' > /dev/null
    curl -s -X POST $SERVER_URL/breaker/reset -d '{"confirm": true}' > /dev/null
    log_success "Cleanup completed"
}

main "$@"