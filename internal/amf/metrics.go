// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterMetrics registers the converged radio/subscriber gauges. enbCount and
// epsSubscribers contribute the 4G (MME) tallies so the gauges reflect both
// access technologies; pass nil when no MME is present.
func RegisterMetrics(amf *AMF, enbCount, epsSubscribers func() int) {
	connectedRadios := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_connected_radios",
		Help: "Number of radios (5G gNBs and 4G eNBs) currently connected to Ella Core",
	}, func() float64 {
		return float64(amf.CountRadios() + countOrZero(enbCount))
	})

	registeredSubscribers := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "app_registered_subscribers",
		Help: "Number of subscribers (5GS and EPS) currently registered in Ella Core",
	}, func() float64 {
		return float64(amf.CountRegisteredSubscribers() + countOrZero(epsSubscribers))
	})

	prometheus.MustRegister(connectedRadios)
	prometheus.MustRegister(registeredSubscribers)
}

func countOrZero(f func() int) int {
	if f == nil {
		return 0
	}

	return f()
}
