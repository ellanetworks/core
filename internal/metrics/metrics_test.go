// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// labelValue returns the value of the named label on a metric, or "".
func labelValue(pairs []*dto.LabelPair, name string) string {
	for _, p := range pairs {
		if p.GetName() == name {
			return p.GetValue()
		}
	}

	return ""
}

func TestSharedMetricsByRAT(t *testing.T) {
	// Helpers must be no-ops before registration (mirrors test binaries that never
	// call RegisterMetrics).
	RegistrationAttempt(RAT4G, "Attach", ResultAccept)
	SignalingMessage(RAT5G, "inbound", "InitialUEMessage")

	RegisterMetrics()

	RegistrationAttempt(RAT4G, "Attach", ResultAccept)
	RegistrationAttempt(RAT5G, "Initial Registration", ResultReject)
	SignalingMessage(RAT4G, "inbound", "InitialUEMessage")
	SignalingMessage(RAT5G, "outbound", "DownlinkNASTransport")

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	wantRATs := map[string]map[string]bool{
		"app_registration_attempts_total": {RAT4G: false, RAT5G: false},
		"app_signaling_messages_total":    {RAT4G: false, RAT5G: false},
	}

	for _, fam := range families {
		seen, ok := wantRATs[fam.GetName()]
		if !ok {
			continue
		}

		for _, m := range fam.GetMetric() {
			if rat := labelValue(m.GetLabel(), "rat"); rat != "" {
				seen[rat] = true
			}
		}
	}

	for name, rats := range wantRATs {
		for rat, found := range rats {
			if !found {
				t.Errorf("%s: missing series for rat=%q", name, rat)
			}
		}
	}
}
