// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsHandlerCoalescesConcurrentScrapes(t *testing.T) {
	const scrapers = 4

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
