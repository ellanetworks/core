// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package metrics owns the Prometheus metrics emitted from more than one network
// function, so a single series is registered once and incremented by both the AMF
// (5G) and the MME (4G). Single-NF metrics stay in their own packages.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// RAT label values distinguishing the radio access technology a metric belongs to.
const (
	RAT4G = "4g"
	RAT5G = "5g"
)

// Result label values for attempt-style counters.
const (
	ResultAccept = "accept"
	ResultReject = "reject"
)

var (
	signalingMessages    *prometheus.CounterVec
	registrationAttempts *prometheus.CounterVec
)

// RegisterMetrics registers the cross-NF metrics. Called once at startup.
func RegisterMetrics() {
	signalingMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_signaling_messages_total",
		Help: "Total radio signaling messages by RAT (NGAP for 5G, S1AP for 4G), direction, and type.",
	}, []string{"rat", "direction", "type"})

	registrationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_registration_attempts_total",
		Help: "Total UE registration (5G) and attach/tracking-area-update (4G) attempts by RAT, type, and result.",
	}, []string{"rat", "type", "result"})

	prometheus.MustRegister(signalingMessages, registrationAttempts)
}

// SignalingMessage records one NGAP/S1AP message. rat is RAT4G or RAT5G; direction
// is "inbound" or "outbound"; msgType is the procedure/message name.
func SignalingMessage(rat, direction, msgType string) {
	if signalingMessages == nil {
		return
	}

	signalingMessages.WithLabelValues(rat, direction, msgType).Inc()
}

// RegistrationAttempt records one registration/attach/TAU outcome. rat is RAT4G or
// RAT5G; regType is the 3GPP procedure name; result is ResultAccept or ResultReject.
func RegistrationAttempt(rat, regType, result string) {
	if registrationAttempts == nil {
		return
	}

	registrationAttempts.WithLabelValues(rat, regType, result).Inc()
}
