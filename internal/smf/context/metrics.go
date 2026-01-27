// Copyright 2026 Ella Networks

package context

import (
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterMetrics() {
	pduSessions := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_pdu_sessions_total",
		Help: "Number of PDU sessions currently in Ella Core",
	}, func() float64 {
		return float64(SMFSelf().GetPDUSessionCount())
	})

	prometheus.MustRegister(pduSessions)
}
