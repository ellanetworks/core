// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRadioInfoNamesEachRadioOnce(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(radioInfoCollector(func() []radioIdentity {
		return []radioIdentity{
			{ID: "gNB:001-01:000001@22", Name: "hall"},
			{ID: "eNB:001-01:MacroeNB-00002", Name: ""},
			{ID: "gNB:001-01:000001@22", Name: "hall"},
			{ID: "", Name: "not set up"},
			{ID: "gNB:001-01:000009@22"},
		}
	}))

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	if len(families) != 1 || families[0].GetName() != "app_radio_info" {
		t.Fatalf("families = %v, want app_radio_info", families)
	}

	got := map[string]string{}

	for _, m := range families[0].GetMetric() {
		labels := map[string]string{}
		for _, l := range m.GetLabel() {
			labels[l.GetName()] = l.GetValue()
		}

		if m.GetGauge().GetValue() != 1 {
			t.Errorf("%v = %v, want 1", labels, m.GetGauge().GetValue())
		}

		got[labels["radio_id"]] = labels["radio"]
	}

	want := map[string]string{"gNB:001-01:000001@22": "hall", "eNB:001-01:MacroeNB-00002": "", "gNB:001-01:000009@22": ""}
	if len(got) != len(want) {
		t.Fatalf("radios = %v, want %v", got, want)
	}

	for id, name := range want {
		if got[id] != name {
			t.Errorf("radio %s named %q, want %q", id, got[id], name)
		}
	}
}
