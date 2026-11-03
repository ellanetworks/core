// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	handshakes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_sepp_n32c_handshakes_total",
		Help: "N32-c exchange-capability procedures, by partner, role (initiator or responder) and result.",
	}, []string{"partner", "role", "result"})

	tlsFailures = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_sepp_tls_handshake_failures_total",
		Help: "N32 TLS handshakes rejected by this node, by reason.",
	}, []string{"reason"})

	n32fRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_sepp_n32f_requests_total",
		Help: "N32-f requests handled by this node, by partner, direction (inbound or outbound), service and HTTP status.",
	}, []string{"partner", "direction", "service", "status"})

	n32fDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "app_sepp_n32f_request_duration_seconds",
		Help:    "Duration of outbound N32-f requests, by partner and service.",
		Buckets: []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 4},
	}, []string{"partner", "service"})

	n32fRejected = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "app_sepp_n32f_rejected_total",
		Help: "Inbound N32-f requests rejected by the SEPP checks, by partner and reason.",
	}, []string{"partner", "reason"})

	stateDesc = prometheus.NewDesc(
		"app_sepp_n32c_state",
		"N32-c context state per partner: 0 idle, 1 establishing, 2 established, 3 failed, 4 terminated.",
		[]string{"partner"},
		nil,
	)

	expiryDesc = prometheus.NewDesc(
		"app_sepp_certificate_expiry_timestamp_seconds",
		"Expiry (notAfter) of the SEPP certificate, as a Unix timestamp.",
		nil,
		nil,
	)
)

func RegisterMetrics(reg prometheus.Registerer, m *Manager) {
	reg.MustRegister(handshakes, tlsFailures, n32fRequests, n32fDuration, n32fRejected, &collector{m: m})
}

type collector struct {
	m *Manager
}

func (c *collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- stateDesc

	ch <- expiryDesc
}

func (c *collector) Collect(ch chan<- prometheus.Metric) {
	snap := c.m.snap.Load()

	if snap.identity != nil {
		ch <- prometheus.MustNewConstMetric(expiryDesc, prometheus.GaugeValue, float64(snap.identity.leaf.NotAfter.Unix()))
	}

	contexts := c.m.Contexts()

	for id, p := range snap.partners {
		st, ok := contexts[id]
		if !ok {
			st.State = StateIdle
		}

		ch <- prometheus.MustNewConstMetric(stateDesc, prometheus.GaugeValue, st.State.Gauge(), p.name)
	}
}
