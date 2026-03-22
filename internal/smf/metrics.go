// Copyright 2026 Ella Networks

package smf

import "github.com/prometheus/client_golang/prometheus"

// PDUSessionEstablishmentAttempts tracks accept/reject counts.
var PDUSessionEstablishmentAttempts *prometheus.CounterVec

// RegisterMetrics registers Prometheus metrics for the SMF.
// The provided SessionQuerier is captured by the gauge callback to report
// the current session count on each scrape. Pass nil if no SMF is available
// (the gauge will report 0).
func RegisterMetrics(sessions SessionQuerier) {
	PDUSessionEstablishmentAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_pdu_session_establishment_attempts_total",
			Help: "Total PDU session establishment attempts by result",
		},
		[]string{"result"}, // accept|reject
	)

	pduSessions := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_pdu_sessions_total",
		Help: "Number of PDU sessions currently in Ella Core",
	}, func() float64 {
		if sessions == nil {
			return 0
		}

		return float64(sessions.SessionCount())
	})

	prometheus.MustRegister(PDUSessionEstablishmentAttempts, pduSessions)
}
