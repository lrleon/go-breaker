#!/bin/bash

# Leandro script for debugging. DO NOT USE (it is in spanish lol!)

# Script para forzar y mantener el circuit breaker activo
SERVER_URL="http://localhost:8080"

echo "🔧 FORZANDO CIRCUIT BREAKER ACTIVO"
echo "=================================="

# Paso 1: Configurar condiciones EXTREMAS
echo "1. Configurando thresholds extremos..."
curl -s -X POST $SERVER_URL/breaker/latency -d '{"threshold": 50}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/percentile -d '{"percentile": 50}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/opsgenie/cooldown -d '{"cooldown_seconds": 1}' > /dev/null
echo "✅ Threshold: 50ms, Percentile: 50%, Cooldown: 1s"

# Paso 2: Reiniciar estado
echo "2. Reiniciando estado..."
curl -s -X POST $SERVER_URL/breaker/reset -d '{"confirm": true}' > /dev/null
curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "reset_normal"}' > /dev/null
echo "✅ Estado limpio"

# Paso 3: Activar alta latencia
echo "3. Activando alta latencia..."
curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "high_latency"}' > /dev/null
echo "✅ Alta latencia activada"

# Paso 4: Bombardear con requests
echo "4. Bombardeando con requests..."
for i in {1..20}; do
response=$(curl -s $SERVER_URL/test)
latency=$(echo $response | jq -r '.actual_latency_ms')
triggered=$(echo $response | jq -r '.breaker_status.triggered')

printf "Request %2d: %4dms -> %s\n" $i $latency $triggered

if [ "$triggered" = "true" ]; then
echo "🎯 ¡BREAKER ACTIVADO en request $i!"

# Verificar INMEDIATAMENTE
echo "5. Verificación inmediata:"
status=$(curl -s $SERVER_URL/breaker/status)
current_triggered=$(echo $status | jq -r '.triggered')
echo "   Estado actual: triggered = $current_triggered"

# Verificar alertas pendientes INMEDIATAMENTE
staged=$(curl -s $SERVER_URL/breaker/staged-alerts)
pending_count=$(echo $staged | jq -r '.pending_alerts_count')
echo "   Alertas pendientes: $pending_count"

if [ "$current_triggered" = "true" ] && [ "$pending_count" != "0" ]; then
echo "🎉 ¡ÉXITO! Breaker activo y alertas pendientes"
echo ""
echo "6. Estado completo de alertas:"
echo $staged | jq '.'

echo ""
echo "7. Ahora espera 70 segundos y verifica escalación:"
echo "   curl $SERVER_URL/breaker/staged-alerts | jq"
break
else
echo "⚠️  Breaker se activó pero no hay alertas pendientes"
echo "   Estado: triggered = $current_triggered"
echo "   Alertas: $pending_count"
fi
break
fi

# Pequeña pausa pero seguir bombardeando
sleep 0.2
done

if [ "$triggered" != "true" ]; then
echo "❌ No se pudo activar el breaker después de 20 requests"
echo "Configuración actual:"
curl -s $SERVER_URL/breaker/status | jq '.latency_threshold_ms, .current_percentile_ms, .triggered'
fi