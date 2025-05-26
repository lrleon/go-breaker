// COMPLETELY REPLACE your breaker/staged_alerts.go file with this:

package breaker

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// AlertContext contains complete information about the incident
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

// PendingAlert represents a pending alert for escalation
type PendingAlert struct {
	ID                 string
	TriggerTime        time.Time
	InitialAlertSent   bool
	EscalatedAlertSent bool
	Context            *AlertContext
	ScheduledCheck     time.Time
	BreakerInstance    Breaker // Reference to Breaker to check status
}

// StagedAlertManager handles staged alerts
type StagedAlertManager struct {
	config         *OpsGenieConfig
	opsGenieClient *OpsGenieClient
	mutex          sync.RWMutex
	pendingAlerts  map[string]*PendingAlert
	checkTicker    *time.Ticker
	stopChan       chan bool
	running        bool
}

// NewStagedAlertManager creates a new staged alert manager
func NewStagedAlertManager(config *OpsGenieConfig, opsGenieClient *OpsGenieClient) *StagedAlertManager {
	manager := &StagedAlertManager{
		config:         config,
		opsGenieClient: opsGenieClient,
		pendingAlerts:  make(map[string]*PendingAlert),
		checkTicker:    time.NewTicker(10 * time.Second), // Check every 10 seconds
		stopChan:       make(chan bool),
		running:        false,
	}

	// Start the monitor in the background
	go manager.monitorPendingAlerts()
	manager.running = true

	log.Printf("🔄 Staged Alert Manager initialized with %ds escalation time", config.TimeBeforeSendAlert)
	return manager
}

// OnBreakerTriggered is called when the circuit breaker is triggered
func (sam *StagedAlertManager) OnBreakerTriggered(context *AlertContext, breakerInstance Breaker) {
	sam.mutex.Lock()
	defer sam.mutex.Unlock()

	// Generate a unique ID for this alert
	alertID := fmt.Sprintf("alert-%d", time.Now().Unix())

	log.Printf("🔄 Circuit breaker triggered - Staged alerting activated")
	log.Printf("📊 Peak latency: %dms, Memory: %.1f%%, Reason: %s",
		context.PeakLatency, context.MemoryUsage, context.TriggerReason)
	log.Printf("⏰ Will escalate in %d seconds if issue persists", context.TimeBeforeAlert)

	// Create a pending alert
	pending := &PendingAlert{
		ID:                 alertID,
		TriggerTime:        context.TriggerTime,
		InitialAlertSent:   false,
		EscalatedAlertSent: false,
		Context:            context,
		ScheduledCheck:     context.TriggerTime.Add(time.Duration(sam.config.TimeBeforeSendAlert) * time.Second),
		BreakerInstance:    breakerInstance,
	}

	sam.pendingAlerts[alertID] = pending

	// Send an initial low-priority alert
	go sam.sendInitialAlert(pending)
}

// sendInitialAlert sends the initial low-priority alert
func (sam *StagedAlertManager) sendInitialAlert(pending *PendingAlert) {
	if !sam.config.TriggerOnOpen {
		log.Printf("⚠️ Initial alert skipped - trigger_on_breaker_open is disabled")
		return
	}

	log.Printf("📤 Sending initial low-priority alert (ID: %s)", pending.ID)
	log.Printf("📊 Peak latency: %dms, Memory usage: %.1f%%, Wait time: %ds",
		pending.Context.PeakLatency,
		pending.Context.MemoryUsage,
		pending.Context.WaitTime)

	// Temporarily change the priority for the initial alert
	originalPriority := sam.config.Priority
	sam.config.Priority = sam.config.InitialAlertPriority

	// Send alert using the existing OpsGenie system
	err := sam.opsGenieClient.SendBreakerOpenAlert(
		pending.Context.PeakLatency,
		pending.Context.MemoryUsage < 80, // Invert for the memoryOK parameter
		pending.Context.WaitTime,
	)

	// Restore original priority
	sam.config.Priority = originalPriority

	if err != nil {
		log.Printf("❌ Failed to send initial alert: %v", err)
		return
	}

	sam.mutex.Lock()
	pending.InitialAlertSent = true
	sam.mutex.Unlock()

	log.Printf("📤 Initial monitoring alert sent successfully (Priority: %s, ID: %s)",
		sam.config.InitialAlertPriority, pending.ID)
}

// monitorPendingAlerts monitors pending alerts for escalation
func (sam *StagedAlertManager) monitorPendingAlerts() {
	log.Printf("🔍 Staged Alert Monitor started - checking every 10 seconds")

	for {
		select {
		case <-sam.checkTicker.C:
			sam.checkPendingAlerts()
		case <-sam.stopChan:
			log.Printf("🛑 Staged Alert Monitor stopped")
			return
		}
	}
}

// checkPendingAlerts checks if alerts should be escalated or resolved
func (sam *StagedAlertManager) checkPendingAlerts() {
	sam.mutex.Lock()
	defer sam.mutex.Unlock()

	now := time.Now()
	alertsToRemove := []string{}

	for alertID, pending := range sam.pendingAlerts {
		// Check if it's time to evaluate this alert
		if now.After(pending.ScheduledCheck) && !pending.EscalatedAlertSent {

			// Check if the breaker is still triggered
			isStillTriggered := pending.BreakerInstance.TriggeredByLatencies()

			if isStillTriggered {
				// Escalate: The problem persists
				log.Printf("🚨 Escalating alert %s: Breaker has been triggered for %d seconds",
					alertID, sam.config.TimeBeforeSendAlert)
				go sam.sendEscalatedAlert(pending)
				pending.EscalatedAlertSent = true
			} else {
				// Resolve: the breaker is no longer triggered
				log.Printf("✅ Resolving alert %s: Breaker recovered within monitoring period",
					alertID)
				go sam.sendResolutionAlert(pending, "automatic_recovery")
				alertsToRemove = append(alertsToRemove, alertID)
			}
		}

		// Clean up very old alerts (safety mechanism)
		maxAge := time.Duration(sam.config.TimeBeforeSendAlert*3) * time.Second
		if now.Sub(pending.TriggerTime) > maxAge {
			if !pending.EscalatedAlertSent {
				log.Printf("⚠️ Cleaning up stale alert: %s (age: %v)",
					alertID, now.Sub(pending.TriggerTime))
			}
			alertsToRemove = append(alertsToRemove, alertID)
		}
	}

	// Remove alerts marked for deletion
	for _, alertID := range alertsToRemove {
		delete(sam.pendingAlerts, alertID)
	}

	// Log statistics if there are active alerts
	if len(sam.pendingAlerts) > 0 {
		log.Printf("📊 Staged Alert Status: %d pending alerts", len(sam.pendingAlerts))
	}
}

// sendEscalatedAlert sends an escalated alert
func (sam *StagedAlertManager) sendEscalatedAlert(pending *PendingAlert) {
	duration := time.Since(pending.TriggerTime)

	log.Printf("🚨 Sending ESCALATED alert (ID: %s) - Issue persists after %v",
		pending.ID, duration)
	log.Printf("📊 Context: Latency %dms, Memory %.1f%%, Reason: %s",
		pending.Context.PeakLatency,
		pending.Context.MemoryUsage,
		pending.Context.TriggerReason)

	// Temporarily change the priority for the escalated alert
	originalPriority := sam.config.Priority
	sam.config.Priority = sam.config.EscalatedAlertPriority

	// Use the existing OpsGenie system but with escalation context
	err := sam.opsGenieClient.SendBreakerOpenAlert(
		pending.Context.PeakLatency,
		pending.Context.MemoryUsage < 80, // Invert for the memoryOK parameter
		pending.Context.WaitTime,
	)

	// Restore original priority
	sam.config.Priority = originalPriority

	if err != nil {
		log.Printf("❌ Failed to send escalated alert: %v", err)
		return
	}

	log.Printf("🚨 ESCALATED alert sent successfully (Priority: %s, Duration: %v, ID: %s)",
		sam.config.EscalatedAlertPriority, duration, pending.ID)
}

// sendResolutionAlert sends a resolution alert
func (sam *StagedAlertManager) sendResolutionAlert(pending *PendingAlert, method string) {
	duration := time.Since(pending.TriggerTime)

	log.Printf("✅ Sending resolution alert (ID: %s) - Recovered after %v using %s",
		pending.ID, duration, method)

	// Use the existing OpsGenie system for resolution
	err := sam.opsGenieClient.SendBreakerResetAlert()

	if err != nil {
		log.Printf("❌ Failed to send resolution alert: %v", err)
		return
	}

	log.Printf("✅ Resolution alert sent successfully - Circuit breaker recovered after %v (ID: %s)",
		duration, pending.ID)
}

// OnBreakerRecovered is called when the circuit breaker recovers manually
func (sam *StagedAlertManager) OnBreakerRecovered() {
	sam.mutex.Lock()
	defer sam.mutex.Unlock()

	if len(sam.pendingAlerts) == 0 {
		return
	}

	log.Printf("✅ Manual breaker recovery - resolving all %d pending alerts", len(sam.pendingAlerts))

	// Mark all pending alerts as resolved
	for alertID, pending := range sam.pendingAlerts {
		if !pending.EscalatedAlertSent {
			go sam.sendResolutionAlert(pending, "manual_reset")
		}
		delete(sam.pendingAlerts, alertID)
	}

	log.Printf("✅ All pending alerts resolved due to manual breaker recovery")
}

// GetPendingAlertsCount returns the number of pending alerts (for debugging)
func (sam *StagedAlertManager) GetPendingAlertsCount() int {
	sam.mutex.RLock()
	defer sam.mutex.RUnlock()
	return len(sam.pendingAlerts)
}

// GetPendingAlertsInfo returns information about pending alerts (for debugging)
func (sam *StagedAlertManager) GetPendingAlertsInfo() map[string]map[string]interface{} {
	sam.mutex.RLock()
	defer sam.mutex.RUnlock()

	info := make(map[string]map[string]interface{})
	for alertID, pending := range sam.pendingAlerts {
		info[alertID] = map[string]interface{}{
			"trigger_time":         pending.TriggerTime,
			"initial_alert_sent":   pending.InitialAlertSent,
			"escalated_alert_sent": pending.EscalatedAlertSent,
			"scheduled_check":      pending.ScheduledCheck,
			"age_seconds":          time.Since(pending.TriggerTime).Seconds(),
			"peak_latency":         pending.Context.PeakLatency,
			"trigger_reason":       pending.Context.TriggerReason,
		}
	}
	return info
}

// Stop stops the staged alert manager
func (sam *StagedAlertManager) Stop() {
	if !sam.running {
		return
	}

	log.Printf("🛑 Stopping Staged Alert Manager...")

	sam.mutex.Lock()
	pendingCount := len(sam.pendingAlerts)
	sam.mutex.Unlock()

	if pendingCount > 0 {
		log.Printf("⚠️ Stopping with %d pending alerts", pendingCount)
	}

	close(sam.stopChan)
	sam.checkTicker.Stop()
	sam.running = false

	log.Printf("🛑 Staged Alert Manager stopped")
}
