// Copyright 2026 Ella Networks

package ngap

import "github.com/prometheus/client_golang/prometheus"

var NGAPMessages *prometheus.CounterVec

func RegisterMetrics() {
	NGAPMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_ngap_messages_total",
		Help: "Total number of received NGAP message per type",
	}, []string{"type"})

	prometheus.MustRegister(NGAPMessages)
}
