// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterMetrics(m *Manager) {
	up := prometheus.NewDesc(
		"app_diameter_peer_up",
		"Whether this node's Diameter connection to a configured peer is open (1) or not (0), by peer and role.",
		[]string{"peer", "role"},
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		for _, peer := range m.Peers() {
			value := 0.0
			if peer.State == diameter.PeerOpen {
				value = 1
			}

			ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, value, peer.ID, peer.Role)
		}
	}))
}
