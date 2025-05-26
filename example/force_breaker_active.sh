#!/bin/bash

# Leandro script for debugging. DO NOT USE (it is in spanish lol!)

# Script to force and maintain the circuit breaker active
SERVER_URL="http://localhost:8080"

echo " FORCING CIRCUIT BREAKER ACTIVE"
echo "=================================="

# Step 1: Configure EXTREME conditions
echo "1. Configuring extreme thresholds..."
curl -s -X POST $SERVER_URL/breaker/latency -d '{"threshold": 50}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/percentile -d '{"percentile": 50}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/opsgenie/cooldown -d '{"cooldown_seconds": 1}' > /dev/null
echo " Threshold: 50ms, Percentile: 50%, Cooldown: 1s"

# Step 2: Reset state
echo "2. Resetting state..."
curl -s -X POST $SERVER_URL/breaker/reset -d '{"confirm": true}' > /dev/null
curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "reset_normal"}' > /dev/null
echo " Clean state"

# Step 3: Activate high latency
echo "3. Activating high latency..."
curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "high_latency"}' > /dev/null
echo " High latency activated"

# Step 4: Bombard with requests
echo "4. Bombarding with requests..."
for i in {1..20}; do
response=$(curl -s $SERVER_URL/test)
latency=$(echo $response | jq -r '.actual_latency_ms')
triggered=$(echo $response | jq -r '.breaker_status.triggered')

printf "Request %2d: %4dms -> %s\n" $i $latency $triggered

if [ "$triggered" = "true" ]; then
echo " BREAKER ACTIVATED on request $i!"

# Verify IMMEDIATELY
echo "5. Immediate verification:"
status=$(curl -s $SERVER_URL/breaker/status)
current_triggered=$(echo $status | jq -r '.triggered')
echo "   Current state: triggered = $current_triggered"

# Verify pending alerts IMMEDIATELY
staged=$(curl -s $SERVER_URL/breaker/staged-alerts)
pending_count=$(echo $staged | jq -r '.pending_alerts_count')
echo "   Pending alerts: $pending_count"

if [ "$current_triggered" = "true" ] && [ "$pending_count" != "0" ]; then
echo " SUCCESS! Breaker active and pending alerts"
echo ""
echo "6. Complete alert status:"
echo $staged | jq '.'

echo ""
echo "7. Now wait 70 seconds and verify escalation:"
echo "   curl $SERVER_URL/breaker/staged-alerts | jq"
break
else
echo "  Breaker activated but no pending alerts"
echo "   State: triggered = $current_triggered"
echo "   Alerts: $pending_count"
fi
break
fi

# Small pause but continue bombarding
sleep 0.2
done

if [ "$triggered" != "true" ]; then
echo " Could not activate the breaker after 20 requests"
echo "Current configuration:"
curl -s $SERVER_URL/breaker/status | jq '.latency_threshold_ms, .current_percentile_ms, .triggered'
fi