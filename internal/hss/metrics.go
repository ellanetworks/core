// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const (
	registrationAccepted = "accept"
	registrationRejected = "reject"

	registeredCountTimeout = 5 * time.Second
)

var registrationAttempts *prometheus.CounterVec

func (h *HSS) RegisterMetrics() {
	registrationAttempts = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_ims_registration_attempts_total",
		Help: "Total IMS registration and re-registration attempts, by result.",
	}, []string{"result"})

	registered := prometheus.NewDesc(
		"app_ims_registered_subscribers",
		"Number of subscribers currently registered in IMS.",
		nil,
		nil,
	)

	prometheus.MustRegister(registrationAttempts, prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ctx, cancel := context.WithTimeout(context.Background(), registeredCountTimeout)
		defer cancel()

		n, err := h.store.CountRegistered(ctx)
		if err != nil {
			h.log.Warn("count IMS registrations failed", zap.Error(err))
			return
		}

		ch <- prometheus.MustNewConstMetric(registered, prometheus.GaugeValue, float64(n))
	}))
}

func recordRegistrationAttempt(result string) {
	if registrationAttempts != nil {
		registrationAttempts.WithLabelValues(result).Inc()
	}
}
