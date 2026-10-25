// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const registeredCountTimeout = 5 * time.Second

const (
	pushDelivered    = "delivered"
	pushUnsupported  = "unsupported"
	pushRejected     = "rejected"
	pushUserUnknown  = "user_unknown"
	pushNotConnected = "not_connected"
	pushFailed       = "failed"
)

var pushes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "app_ims_profile_pushes_total",
	Help: "Cx profile pushes to the S-CSCF, by outcome.",
}, []string{"outcome"})

func (h *HSS) RegisterMetrics(reg prometheus.Registerer) {
	registered := prometheus.NewDesc(
		"app_ims_registered_subscribers",
		"Number of subscribers currently registered in IMS, across the cluster.",
		nil,
		nil,
	)

	reg.MustRegister(pushes)

	reg.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
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
