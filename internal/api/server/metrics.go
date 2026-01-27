// Copyright 2026 Ella Networks

package server

import "github.com/prometheus/client_golang/prometheus"

var (
	APIRequestsTotal   *prometheus.CounterVec
	APIRequestDuration *prometheus.HistogramVec
	APIAuthAttempts    *prometheus.CounterVec
)

func RegisterMetrics() {
	APIRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_api_requests_total",
			Help: "Total number of HTTP requests by method, endpoint, and status code",
		},
		[]string{"method", "endpoint", "status"},
	)

	APIRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "app_api_request_duration_seconds",
			Help:    "HTTP request duration histogram in seconds",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		},
		[]string{"method", "endpoint"},
	)

	APIAuthAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "app_api_authentication_attempts_total",
			Help: "Total number of authentication attempts by type and result",
		},
		[]string{"type", "result"},
	)

	prometheus.MustRegister(APIRequestsTotal)
	prometheus.MustRegister(APIRequestDuration)
	prometheus.MustRegister(APIAuthAttempts)
}
