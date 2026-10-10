// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/prometheus/client_golang/prometheus"
)

func RegisterMetrics(m *Manager) {
	serviceCentres := prometheus.NewDesc(
		"app_connected_service_centers",
		"Number of SMS service centers (SMSCs) connected to this node.",
		nil,
		nil,
	)

	imsNodes := prometheus.NewDesc(
		"app_connected_ims_nodes",
		"Number of IMS nodes connected to this node, which use it as their HSS or PCRF.",
		nil,
		nil,
	)

	prometheus.MustRegister(prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		smsc, ims := connectedPeers(m.Peers())

		ch <- prometheus.MustNewConstMetric(serviceCentres, prometheus.GaugeValue, float64(smsc))

		ch <- prometheus.MustNewConstMetric(imsNodes, prometheus.GaugeValue, float64(ims))
	}))
}

func connectedPeers(peers []PeerStatus) (smsc, ims int) {
	for _, p := range peers {
		if p.State != diameter.PeerOpen {
			continue
		}

		switch p.Role {
		case PeerRoleSMSC:
			smsc++
		case PeerRoleIMS:
			ims++
		}
	}

	return smsc, ims
}
