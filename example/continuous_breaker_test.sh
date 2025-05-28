#!/bin/bash

# Leandro script for debugging. DO NOT USE (it is in spanish lol!)

# Script to keep the circuit breaker CONTINUOUSLY active
SERVER_URL="http://localhost:8080"

echo "🔄 KEEPING CIRCUIT BREAKER CONTINUOUSLY ACTIVE"
echo "=================================================="

# Configure extreme conditions
echo "1. Configuring extreme conditions..."
curl -s -X POST $SERVER_URL/breaker/latency -d '{"threshold": 100}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/percentile -d '{"percentile": 80}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/wait -d '{"wait_time": 8}' > /dev/null  # Long time before reset
curl -s -X POST $SERVER_URL/breaker/opsgenie/cooldown -d '{"cooldown_seconds": 1}' > /dev/null
echo "✅ Configured: threshold=100ms, percentile=80%, wait=8s"

# Reset
echo "2. Resetting..."
curl -s -X POST $SERVER_URL/breaker/reset -d '{"confirm": true}' > /dev/null
curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "high_latency"}' > /dev/null
echo "✅ Clean state and high latency activated"

echo "3. Continuous bombardment to activate and maintain the breaker..."

# Function to make request and report
make_request() {
    local i=$1
    response=$(curl -s $SERVER_URL/test)
    latency=$(echo $response | jq -r '.actual_latency_ms')
    triggered=$(echo $response | jq -r '.breaker_status.triggered')
    printf "Request %2d: %4dms -> %-5s" $i $latency $triggered
    echo $triggered
}

# Initial bombardment until activation
echo "Phase 1: Activating the breaker..."
breaker_activated=false
for i in {1..30}; do
    triggered=$(make_request $i)

    if [ "$triggered" = "true" ]; then
        echo "🎯 Breaker activated on request $i!"
        breaker_activated=true
        break
    fi
    sleep 0.1
done

if [ "$breaker_activated" = "false" ]; then
    echo "❌ Could not activate the breaker"
    exit 1
fi

echo ""
echo "Phase 2: Keeping the breaker active with continuous requests..."

# Function that makes continuous requests in background
continuous_requests() {
    local counter=1
    while true; do
        curl -s $SERVER_URL/test > /dev/null
        printf "."
        sleep 0.5
        counter=$((counter + 1))
        if [ $counter -gt 200 ]; then  # Max 100 seconds
            break
        fi
    done
}

# Start continuous requests in background
continuous_requests &
BACKGROUND_PID=$!

# Monitor status every 2 seconds
echo "Monitoring status (continuous requests in background)..."
for check in {1..50}; do  # 50 checks = ~100 seconds
    printf "\nCheck %2d: " $check

    # Verify breaker status
    status=$(curl -s $SERVER_URL/breaker/status)
    current_triggered=$(echo $status | jq -r '.triggered')
    current_latency=$(echo $status | jq -r '.current_percentile_ms')

    # Verify pending alerts
    staged=$(curl -s $SERVER_URL/breaker/staged-alerts)
    pending_count=$(echo $staged | jq -r '.pending_alerts_count')

    printf "triggered=%s, latency=%sms, pending=%s" $current_triggered $current_latency $pending_count

    # If we have pending alerts, success!
    if [ "$pending_count" != "0" ] && [ "$pending_count" != "null" ]; then
        echo ""
        echo "🎉 SUCCESS! We have pending alerts:"
        echo $staged | jq '.'

        echo ""
        echo "⏰ Now we'll wait 70 seconds to see the escalation..."
        echo "   (keeping requests active)"

        # Wait for escalation while maintaining requests
        for escalation_check in {1..35}; do  # 35 * 2 = 70 seconds
            printf "Escalation wait %2d/35..." $escalation_check

            # Check if already escalated
            escalation_status=$(curl -s $SERVER_URL/breaker/staged-alerts)
            escalated_count=$(echo $escalation_status | jq -r '.pending_alerts | to_entries | map(select(.value.escalated_alert_sent == true)) | length')

            if [ "$escalated_count" != "0" ]; then
                echo ""
                echo "🚨 ESCALATION DETECTED!"
                echo $escalation_status | jq '.'
                break
            fi

            sleep 2
        done

        break
    fi

    # If the breaker deactivated, try to reactivate
    if [ "$current_triggered" = "false" ]; then
        echo " (reactivating...)"
        # Make some quick requests to reactivate
        for reactivate in {1..5}; do
            curl -s $SERVER_URL/test > /dev/null
            sleep 0.1
        done
    fi

    sleep 2
done

# Stop background requests
kill $BACKGROUND_PID 2>/dev/null

echo ""
echo "🔍 Final status:"
curl -s $SERVER_URL/breaker/staged-alerts | jq '.'

echo ""
echo "📋 Instructions:"
echo "1. Did you see pending alerts during execution?"
echo "2. Were the alerts escalated after ~60 seconds?"
echo "3. Did you receive alerts in your OpsGenie dashboard?"