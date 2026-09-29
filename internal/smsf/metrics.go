// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterMetrics(link *Link) {
	up := prometheus.NewDesc(
		"app_smsc_link_up",
		"Whether this node's Diameter link to the SMSC is open (1) or not (0). Absent while SMS is disabled.",
		nil,
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		if link == nil {
			return
		}

		for _, peer := range link.Peers() {
			if peer.Role != PeerRoleSMSC {
				continue
			}

			value := 0.0
			if peer.State == diameter.PeerOpen {
				value = 1
			}

			ch <- prometheus.MustNewConstMetric(up, prometheus.GaugeValue, value)
		}
	}))
}
