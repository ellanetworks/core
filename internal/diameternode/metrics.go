// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"github.com/ellanetworks/core/diameter"
	"github.com/prometheus/client_golang/prometheus"
)

const rejectNotAllowed = "not_allowed"

var rejectedPeers = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "app_diameter_rejected_peers_total",
	Help: "Diameter peers whose capabilities exchange was rejected, by reason.",
}, []string{"reason"})

func RegisterMetrics(m *Manager) {
	prometheus.MustRegister(rejectedPeers)

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
		case PeerRoleIMS, PeerRoleUnknown:
			ims++
		}
	}

	return smsc, ims
}
