#!/bin/bash

echo "🔍 OpsGenie Diagnostic Script"
echo "============================="

# 1. Check environment variables
echo "1. Environment Variables:"
if [ -z "$OPSGENIE_API_KEY" ]; then
    echo "   ❌ OPSGENIE_API_KEY not set"
else
    echo "   ✅ OPSGENIE_API_KEY set (${OPSGENIE_API_KEY:0:8}...)"
fi

if [ -z "$Environment" ]; then
    echo "   ⚠️  Environment not set"
else
    echo "   ✅ Environment: $Environment"
fi

echo ""

# 2. Check network connectivity
echo "2. Network Connectivity:"
if curl -s --max-time 5 https://api.opsgenie.com/v2/alerts/count > /dev/null; then
    echo "   ✅ Can reach OpsGenie US API"
else
    echo "   ❌ Cannot reach OpsGenie US API"
fi

if curl -s --max-time 5 https://api.eu.opsgenie.com/v2/alerts/count > /dev/null; then
    echo "   ✅ Can reach OpsGenie EU API"
else
    echo "   ❌ Cannot reach OpsGenie EU API"
fi

echo ""

# 3. Check if server is running
echo "3. Server Status:"
if curl -s http://localhost:8080/status > /dev/null; then
    echo "   ✅ Server is running"
else
    echo "   ❌ Server is not running on localhost:8080"
    echo "   Start with: go run example/server_example.go"
    exit 1
fi

echo ""

# 4. Check OpsGenie validation
echo "4. OpsGenie Validation:"
validation_result=$(curl -s http://localhost:8080/opsgenie/validate)
echo "   Response: $validation_result"

if echo "$validation_result" | grep -q '"status":"success"'; then
    echo "   ✅ Validation passed"
else
    echo "   ❌ Validation failed"
fi

echo ""

# 5. Check OpsGenie connection
echo "5. OpsGenie Connection Test:"
connection_result=$(curl -s http://localhost:8080/opsgenie/test-connection)
echo "   Response: $connection_result"

if echo "$connection_result" | grep -q '"status":"success"'; then
    echo "   ✅ Connection test passed"
else
    echo "   ❌ Connection test failed"
fi

echo ""

# 6. Check configuration file
echo "6. Configuration File:"
if [ -f "breakers.toml" ]; then
    echo "   ✅ breakers.toml exists"
    if grep -q "enabled = true" breakers.toml; then
        echo "   ✅ OpsGenie enabled in config"
    else
        echo "   ❌ OpsGenie not enabled in config"
    fi
else
    echo "   ❌ breakers.toml not found"
fi

echo ""

# 7. Try to send test alert
echo "7. Test Alert:"
alert_result=$(curl -s -X POST http://localhost:8080/opsgenie/send-test-alert)
echo "   Response: $alert_result"

if echo "$alert_result" | grep -q '"status":"success"'; then
    echo "   ✅ Test alert sent successfully!"
    echo "   📱 Check your OpsGenie dashboard"
else
    echo "   ❌ Test alert failed"
fi

echo ""
echo "🏁 Diagnostic complete!"
echo ""
echo "If all tests pass but alerts don't appear:"
echo "1. Check your OpsGenie dashboard"
echo "2. Verify the team name exists in OpsGenie"
echo "3. Check your notification settings"
echo "4. Look for alerts in 'All Alerts' not just assigned to you"