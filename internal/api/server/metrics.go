// Copyright 2026 Ella Networks

package server

import "github.com/prometheus/client_golang/prometheus"

var (
	APIRequestsTotal       *prometheus.CounterVec
	APIRequestDuration     *prometheus.HistogramVec
	APIAuthAttempts        *prometheus.CounterVec
	APINetworkRulesCreated prometheus.Counter
	APINetworkRulesDeleted prometheus.Counter
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

	APINetworkRulesCreated = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_network_rules_created_total",
			Help: "Total number of network rules created via API",
		},
	)

	APINetworkRulesDeleted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "api_network_rules_deleted_total",
			Help: "Total number of network rules deleted via API",
		},
	)

	prometheus.MustRegister(APIRequestsTotal)
	prometheus.MustRegister(APIRequestDuration)
	prometheus.MustRegister(APIAuthAttempts)
	prometheus.MustRegister(APINetworkRulesCreated)
	prometheus.MustRegister(APINetworkRulesDeleted)
}
