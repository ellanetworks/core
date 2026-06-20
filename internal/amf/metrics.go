// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterMetrics registers the radio/subscriber gauges, broken down by RAT.
// enbCount and epsSubscribers supply the 4G (MME) tallies; pass nil when no MME
// is present.
func RegisterMetrics(amf *AMF, enbCount, epsSubscribers func() int) {
	radiosDesc := prometheus.NewDesc(
		"app_connected_radios",
		"Number of radios currently connected to Ella Core, by RAT (5G gNBs, 4G eNBs).",
		[]string{"rat"},
		nil,
	)

	subscribersDesc := prometheus.NewDesc(
		"app_registered_subscribers",
		"Number of subscribers currently registered in Ella Core, by RAT (5GS, EPS).",
		[]string{"rat"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		ch <- prometheus.MustNewConstMetric(radiosDesc, prometheus.GaugeValue, float64(amf.CountRadios()), "5g")

		ch <- prometheus.MustNewConstMetric(radiosDesc, prometheus.GaugeValue, float64(countOrZero(enbCount)), "4g")

		ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(amf.CountRegisteredSubscribers()), "5g")

		ch <- prometheus.MustNewConstMetric(subscribersDesc, prometheus.GaugeValue, float64(countOrZero(epsSubscribers)), "4g")
	}))
}

func countOrZero(f func() int) int {
	if f == nil {
		return 0
	}

	return f()
}
