// REEMPLAZAR COMPLETAMENTE tu archivo breaker/staged_alerts.go con esto:

package breaker

import (
	"log"
	"time"
)

// AlertContext contiene información completa del incidente
type AlertContext struct {
	TriggerTime     time.Time `json:"trigger_time"`
	PeakLatency     int64     `json:"peak_latency_ms"`
	AverageLatency  int64     `json:"average_latency_ms"`
	TriggerReason   string    `json:"trigger_reason"`
	MemoryUsage     float64   `json:"memory_usage_percent"`
	RecentLatencies []int64   `json:"recent_latencies_ms"`
	WaitTime        int       `json:"wait_time_seconds"`
	TimeBeforeAlert int       `json:"time_before_alert_seconds"`
}

// StagedAlertManager maneja las alertas escalonadas
type StagedAlertManager struct {
	config         *OpsGenieConfig
	opsGenieClient *OpsGenieClient
}

// NewStagedAlertManager crea un nuevo manager de alertas escalonadas
func NewStagedAlertManager(config *OpsGenieConfig, opsGenieClient *OpsGenieClient) *StagedAlertManager {
	log.Printf("🔄 Initializing staged alert manager (escalation after %ds)", config.TimeBeforeSendAlert)

	return &StagedAlertManager{
		config:         config,
		opsGenieClient: opsGenieClient,
	}
}

// OnBreakerTriggered se llama cuando el circuit breaker se dispara
func (sam *StagedAlertManager) OnBreakerTriggered(context *AlertContext, breakerInstance Breaker) {
	log.Printf("🔄 Circuit breaker triggered - Staged alerting activated")
	log.Printf("📊 Peak latency: %dms, Memory: %.1f%%, Reason: %s",
		context.PeakLatency, context.MemoryUsage, context.TriggerReason)
	log.Printf("⏰ Will escalate in %d seconds if issue persists", context.TimeBeforeAlert)

	// Por ahora, enviar alerta con la prioridad inicial (P3)
	// TODO: Implementar la lógica de escalación completa más tarde
	if sam.opsGenieClient != nil {
		go func() {
			log.Printf("📤 Sending initial staged alert (Priority: %s)", sam.config.InitialAlertPriority)

			err := sam.opsGenieClient.SendBreakerOpenAlert(
				context.PeakLatency,
				context.MemoryUsage > 80,
				context.WaitTime,
			)

			if err != nil {
				log.Printf("❌ Failed to send staged alert: %v", err)
			} else {
				log.Printf("✅ Initial staged alert sent successfully")
				// TODO: Programar verificación para escalación después de TimeBeforeAlert segundos
				log.Printf("🔮 Future enhancement: Will check for escalation in %d seconds", context.TimeBeforeAlert)
			}
		}()
	} else {
		log.Printf("⚠️ No OpsGenie client available for staged alerting")
	}
}

// Stop detiene el manager (por ahora no hace nada)
func (sam *StagedAlertManager) Stop() {
	log.Printf("🛑 Staged alert manager stopped")
}
