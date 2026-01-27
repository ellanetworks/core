// Copyright 2026 Ella Networks

package gmm

import "github.com/prometheus/client_golang/prometheus"

var UERegistrationAttempts *prometheus.CounterVec

func RegisterMetrics() {
	UERegistrationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_registration_attempts_total",
		Help: "Total number of UE registration attempts by type and result",
	}, []string{"type", "result"})

	prometheus.MustRegister(UERegistrationAttempts)
}
