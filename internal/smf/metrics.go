// Copyright 2026 Ella Networks

package smf

import "github.com/prometheus/client_golang/prometheus"

// PDUSessionEstablishmentAttempts tracks accept/reject counts.
var PDUSessionEstablishmentAttempts *prometheus.CounterVec

func RegisterMetrics() {
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
		inst := Instance()
		if inst == nil {
			return 0
		}

		return float64(inst.SessionCount())
	})

	prometheus.MustRegister(PDUSessionEstablishmentAttempts, pduSessions)
}
