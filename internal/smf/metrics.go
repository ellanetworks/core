// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// SessionEstablishmentAttempts counts session establishment outcomes by RAT
// ("4g"|"5g") and result ("accept"|"reject").
var SessionEstablishmentAttempts *prometheus.CounterVec

// sessionCounter reports active session counts split by RAT. *SMF satisfies it.
type sessionCounter interface {
	SessionCountByRAT() (fourG, fiveG int)
}

// RegisterMetrics registers the SMF metrics. The sessions gauge reads per-RAT
// counts from sessionCounter on each scrape; pass nil to report 0.
func RegisterMetrics(sessions sessionCounter) {
	SessionEstablishmentAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_session_establishment_attempts_total",
			Help: "Total session establishment attempts by RAT and result (5G PDU sessions, 4G EPS sessions).",
		},
		[]string{"rat", "result"},
	)

	sessionsDesc := prometheus.NewDesc(
		"app_sessions_total",
		"Number of active sessions by RAT (5G PDU sessions, 4G EPS sessions).",
		[]string{"rat"},
		nil,
	)

	prometheus.MustRegister(SessionEstablishmentAttempts)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		var fourG, fiveG int
		if sessions != nil {
			fourG, fiveG = sessions.SessionCountByRAT()
		}

		ch <- prometheus.MustNewConstMetric(sessionsDesc, prometheus.GaugeValue, float64(fiveG), "5g")

		ch <- prometheus.MustNewConstMetric(sessionsDesc, prometheus.GaugeValue, float64(fourG), "4g")
	}))
}

// recordSessionEstablishment counts one session establishment outcome, deriving
// the result from err. Safe to call before RegisterMetrics (no-op).
func recordSessionEstablishment(rat string, err error) {
	result := metrics.ResultAccept
	if err != nil {
		result = metrics.ResultReject
	}

	recordSessionEstablishmentResult(rat, result)
}

// recordSessionEstablishmentResult counts one session establishment outcome. An
// empty result is a no-op, so callers can defer it and leave the result unset on
// paths that are not an establishment attempt. Safe to call before
// RegisterMetrics (no-op).
func recordSessionEstablishmentResult(rat, result string) {
	if SessionEstablishmentAttempts == nil || result == "" {
		return
	}

	SessionEstablishmentAttempts.WithLabelValues(rat, result).Inc()
}
