// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	metricsScrapeTimeout       = 8 * time.Second
	metricsMaxRequestsInFlight = 32
)

type metricsErrorLogger struct{}

func (metricsErrorLogger) Println(v ...any) {
	if logger.MetricsLog == nil {
		return
	}

	logger.MetricsLog.Warn(strings.TrimSuffix(fmt.Sprintln(v...), "\n"))
}

var metricsHandler = sync.OnceValue(func() http.Handler {
	return newMetricsHandler(prometheus.DefaultGatherer)
})

func metricsHandlerFor(g prometheus.Gatherer) http.Handler {
	if g == nil {
		return GetMetrics()
	}

	return newMetricsHandler(g)
}

func newMetricsHandler(g prometheus.Gatherer) http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(g, promhttp.HandlerOpts{
			CoalesceGather:      true,
			Timeout:             metricsScrapeTimeout,
			MaxRequestsInFlight: metricsMaxRequestsInFlight,
			ErrorHandling:       promhttp.ContinueOnError,
			ErrorLog:            metricsErrorLogger{},
		}),
	)
}

func GetMetrics() http.Handler {
	return metricsHandler()
}
