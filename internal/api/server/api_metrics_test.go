// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/prometheus/client_golang/prometheus"
)

var panicDesc = prometheus.NewDesc("panic_test_metric", "panics on collect", nil, nil)

type panicCollector struct{}

func (panicCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- panicDesc
}

func (panicCollector) Collect(chan<- prometheus.Metric) {
	panic("collector exploded")
}

func TestMetricsHandlerServesTheRestWhenOneCollectorFails(t *testing.T) {
	canary := prometheus.NewGauge(prometheus.GaugeOpts{Name: "failover_canary", Help: "canary"})
	canary.Set(1)

	broken := panicCollector{}

	prometheus.MustRegister(canary, broken)

	defer prometheus.Unregister(canary)

	defer func() {
		if !prometheus.Unregister(broken) {
			t.Error("failing collector stayed registered and will pollute later tests")
		}
	}()

	rr := httptest.NewRecorder()
	server.GetMetrics().ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/metrics", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status %d, want %d: one failing collector must not blank the endpoint", rr.Code, http.StatusOK)
	}

	if !strings.Contains(rr.Body.String(), "failover_canary") {
		t.Error("unrelated metric was suppressed by the failing collector")
	}

	if !strings.Contains(rr.Body.String(), "go_goroutines") {
		t.Error("Go runtime metrics were suppressed by the failing collector")
	}
}

func TestMetricsHandlerCoalescesConcurrentScrapes(t *testing.T) {
	const scrapers = 8

	var collects atomic.Int64

	slowDesc := prometheus.NewDesc("slow_test_metric", "slow", nil, nil)

	slow := prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		collects.Add(1)
		time.Sleep(300 * time.Millisecond)

		ch <- prometheus.MustNewConstMetric(slowDesc, prometheus.GaugeValue, 1)
	})

	prometheus.MustRegister(slow)

	defer prometheus.Unregister(slow)

	collects.Store(0)

	handler := server.GetMetrics()

	var ready, done sync.WaitGroup

	start := make(chan struct{})

	ready.Add(scrapers)
	done.Add(scrapers)

	codes := make([]int, scrapers)

	for i := range scrapers {
		go func() {
			defer done.Done()

			ready.Done()
			<-start

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/metrics", nil))
			codes[i] = rr.Code
		}()
	}

	ready.Wait()
	close(start)
	done.Wait()

	if got := collects.Load(); got != 1 {
		t.Errorf("collector ran %d times for %d concurrent scrapes, want 1", got, scrapers)
	}

	for i, code := range codes {
		if code != http.StatusOK {
			t.Errorf("scrape %d: status %d, want %d", i, code, http.StatusOK)
		}
	}
}
