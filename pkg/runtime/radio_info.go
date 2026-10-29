// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/prometheus/client_golang/prometheus"
)

type radioIdentity struct {
	ID   string
	Name string
}

func knownRadios(amfInstance *amf.AMF, mmeInstance *mme.MME) func() []radioIdentity {
	return func() []radioIdentity {
		var out []radioIdentity

		for _, r := range amfInstance.ListRadios() {
			out = append(out, radioIdentity{ID: r.Ref, Name: r.Name})
		}

		for _, r := range mmeInstance.ListRadios() {
			out = append(out, radioIdentity{ID: r.Ref, Name: r.Name})
		}

		return out
	}
}

func radioInfoCollector(radios func() []radioIdentity) prometheus.Collector {
	desc := prometheus.NewDesc(
		"app_radio_info",
		"Radios (gNBs and eNBs) known to this node, always 1. radio_id is the radio's global RAN node ID, as in the radios API; radio is its name. Join on radio_id to name the series of app_upf_radio_* metrics.",
		[]string{"radio_id", "radio"},
		nil,
	)

	return prometheus.CollectorFunc(func(ch chan<- prometheus.Metric) {
		seen := make(map[string]bool)

		for _, r := range radios() {
			if r.ID == "" || seen[r.ID] {
				continue
			}

			seen[r.ID] = true

			ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, 1, r.ID, r.Name)
		}
	})
}
