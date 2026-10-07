// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import "github.com/prometheus/client_golang/prometheus"

const (
	directionMO = "mo"
	directionMT = "mt"
)

const (
	moForwarded       = "forwarded"
	moSMSCRejected    = "smsc_rejected"
	moSMSCUnavailable = "smsc_unavailable"
	moUnknownSC       = "unknown_service_centre"
	moNotAllowed      = "not_allowed"
	moInvalid         = "invalid"
	moError           = "error"
)

const (
	mtDelivered   = "delivered"
	mtAbsent      = "absent"
	mtUERejected  = "ue_rejected"
	mtNoResponse  = "no_response"
	mtBusy        = "busy"
	mtUnknownUser = "unknown_user"
	mtInvalid     = "invalid"
	mtError       = "error"
)

var attempts *prometheus.CounterVec

func RegisterMetrics() {
	attempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_sms_attempts_total",
		Help: "Total SMS attempts by direction (mo: mobile-originated, mt: mobile-terminated) and result.",
	}, []string{"direction", "result"})

	prometheus.MustRegister(attempts)
}

func recordAttempt(direction, result string) {
	if attempts != nil {
		attempts.WithLabelValues(direction, result).Inc()
	}
}
