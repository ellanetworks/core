// Copyright 2026 Ella Networks

package pdusession

import "github.com/prometheus/client_golang/prometheus"

var PDUSessionEstablishmentAttempts *prometheus.CounterVec

func RegisterMetrics() {
	PDUSessionEstablishmentAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_pdu_session_establishment_attempts_total",
			Help: "Total PDU session establishment attempts by result",
		},
		[]string{"result"}, // accept|reject
	)

	prometheus.MustRegister(PDUSessionEstablishmentAttempts)
}
