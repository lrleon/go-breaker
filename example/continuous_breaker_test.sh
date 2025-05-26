#!/bin/bash

# Leandro script for debugging. DO NOT USE (it is in spanish lol!)

# Script para mantener el circuit breaker activo CONTINUAMENTE
SERVER_URL="http://localhost:8080"

echo "🔄 MANTENIENDO CIRCUIT BREAKER ACTIVO CONTINUAMENTE"
echo "=================================================="

# Configurar condiciones extremas
echo "1. Configurando condiciones extremas..."
curl -s -X POST $SERVER_URL/breaker/latency -d '{"threshold": 100}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/percentile -d '{"percentile": 80}' > /dev/null
curl -s -X POST $SERVER_URL/breaker/wait -d '{"wait_time": 8}' > /dev/null  # Tiempo largo antes de reset
curl -s -X POST $SERVER_URL/breaker/opsgenie/cooldown -d '{"cooldown_seconds": 1}' > /dev/null
echo "✅ Configurado: threshold=100ms, percentile=80%, wait=8s"

# Reiniciar
echo "2. Reiniciando..."
curl -s -X POST $SERVER_URL/breaker/reset -d '{"confirm": true}' > /dev/null
curl -s -X POST $SERVER_URL/test/trigger -d '{"scenario": "high_latency"}' > /dev/null
echo "✅ Estado limpio y alta latencia activada"

echo "3. Bombardeo continuo para activar y mantener el breaker..."

# Función para hacer request y reportar
make_request() {
    local i=$1
    response=$(curl -s $SERVER_URL/test)
    latency=$(echo $response | jq -r '.actual_latency_ms')
    triggered=$(echo $response | jq -r '.breaker_status.triggered')
    printf "Request %2d: %4dms -> %-5s" $i $latency $triggered
    echo $triggered
}

# Bombardeo inicial hasta activar
echo "Fase 1: Activando el breaker..."
breaker_activated=false
for i in {1..30}; do
    triggered=$(make_request $i)

    if [ "$triggered" = "true" ]; then
        echo "🎯 ¡Breaker activado en request $i!"
        breaker_activated=true
        break
    fi
    sleep 0.1
done

if [ "$breaker_activated" = "false" ]; then
    echo "❌ No se pudo activar el breaker"
    exit 1
fi

echo ""
echo "Fase 2: Manteniendo el breaker activo con requests continuos..."

# Función que hace requests continuos en background
continuous_requests() {
    local counter=1
    while true; do
        curl -s $SERVER_URL/test > /dev/null
        printf "."
        sleep 0.5
        counter=$((counter + 1))
        if [ $counter -gt 200 ]; then  # Max 100 segundos
            break
        fi
    done
}

# Iniciar requests continuos en background
continuous_requests &
BACKGROUND_PID=$!

# Monitorear estado cada 2 segundos
echo "Monitoreando estado (requests continuos en background)..."
for check in {1..50}; do  # 50 checks = ~100 segundos
    printf "\nCheck %2d: " $check

    # Verificar estado del breaker
    status=$(curl -s $SERVER_URL/breaker/status)
    current_triggered=$(echo $status | jq -r '.triggered')
    current_latency=$(echo $status | jq -r '.current_percentile_ms')

    # Verificar alertas pendientes
    staged=$(curl -s $SERVER_URL/breaker/staged-alerts)
    pending_count=$(echo $staged | jq -r '.pending_alerts_count')

    printf "triggered=%s, latency=%sms, pending=%s" $current_triggered $current_latency $pending_count

    # Si tenemos alertas pendientes, ¡éxito!
    if [ "$pending_count" != "0" ] && [ "$pending_count" != "null" ]; then
        echo ""
        echo "🎉 ¡ÉXITO! Tenemos alertas pendientes:"
        echo $staged | jq '.'

        echo ""
        echo "⏰ Ahora esperaremos 70 segundos para ver la escalación..."
        echo "   (manteniendo requests activos)"

        # Esperar escalación mientras mantenemos requests
        for escalation_check in {1..35}; do  # 35 * 2 = 70 segundos
            printf "Escalation wait %2d/35..." $escalation_check

            # Verificar si ya escaló
            escalation_status=$(curl -s $SERVER_URL/breaker/staged-alerts)
            escalated_count=$(echo $escalation_status | jq -r '.pending_alerts | to_entries | map(select(.value.escalated_alert_sent == true)) | length')

            if [ "$escalated_count" != "0" ]; then
                echo ""
                echo "🚨 ¡ESCALACIÓN DETECTADA!"
                echo $escalation_status | jq '.'
                break
            fi

            sleep 2
        done

        break
    fi

    # Si el breaker se desactivó, intentar reactivar
    if [ "$current_triggered" = "false" ]; then
        echo " (reactivando...)"
        # Hacer algunos requests rápidos para reactivar
        for reactivate in {1..5}; do
            curl -s $SERVER_URL/test > /dev/null
            sleep 0.1
        done
    fi

    sleep 2
done

# Detener requests en background
kill $BACKGROUND_PID 2>/dev/null

echo ""
echo "🔍 Estado final:"
curl -s $SERVER_URL/breaker/staged-alerts | jq '.'

echo ""
echo "📋 Instrucciones:"
echo "1. ¿Viste alertas pendientes durante la ejecución?"
echo "2. ¿Se escalaron las alertas después de ~60 segundos?"
echo "3. ¿Recibiste alertas en tu dashboard de OpsGenie?"