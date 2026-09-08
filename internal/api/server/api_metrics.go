// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsScrapeTimeout       = 10 * time.Second
	metricsMaxRequestsInFlight = 4
)

var metricsHandler = sync.OnceValue(func() http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
			CoalesceGather:      true,
			Timeout:             metricsScrapeTimeout,
			MaxRequestsInFlight: metricsMaxRequestsInFlight,
		}),
	)
})

func GetMetrics() http.Handler {
	return metricsHandler()
}
