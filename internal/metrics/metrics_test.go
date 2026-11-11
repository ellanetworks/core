// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"testing"
	"time"

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
	RegistrationAttempt(RAT4G, "Attach", ResultAccept, false)
	SignalingMessage(RAT5G, "inbound", "InitialUEMessage")
	PeerRequest(PeerS6a, PeerSuccess, time.Millisecond)

	RegisterMetrics()

	RegistrationAttempt(RAT4G, "Attach", ResultAccept, false)
	RegistrationAttempt(RAT5G, "Initial Registration", ResultReject, true)
	SignalingMessage(RAT4G, "inbound", "InitialUEMessage")
	SignalingMessage(RAT5G, "outbound", "DownlinkNASTransport")
	PeerRequest(PeerN32, PeerTimeout, 2*time.Second)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	checkPeerAndRoamingSeries(t, families)

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

func TestRadioGaugesByRAT(t *testing.T) {
	RegisterRadioGauges(
		func() int { return 2 },
		func() (int, int) { return 3, 1 },
		nil,
		func() (int, int) { return 5, 0 },
	)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	want := map[string]float64{
		"app_connected_radios{rat=5g}":                     2,
		"app_connected_radios{rat=4g}":                     0,
		"app_registered_subscribers{rat=5g,roaming=false}": 3,
		"app_registered_subscribers{rat=5g,roaming=true}":  1,
		"app_registered_subscribers{rat=4g,roaming=false}": 5,
		"app_registered_subscribers{rat=4g,roaming=true}":  0,
	}

	for _, fam := range families {
		for _, m := range fam.GetMetric() {
			key := fam.GetName() + "{rat=" + labelValue(m.GetLabel(), "rat")
			if roaming := labelValue(m.GetLabel(), "roaming"); roaming != "" {
				key += ",roaming=" + roaming
			}

			key += "}"

			exp, ok := want[key]
			if !ok {
				continue
			}

			if got := m.GetGauge().GetValue(); got != exp {
				t.Errorf("%s = %v, want %v", key, got, exp)
			}

			delete(want, key)
		}
	}

	for key := range want {
		t.Errorf("missing series %s", key)
	}
}

func checkPeerAndRoamingSeries(t *testing.T, families []*dto.MetricFamily) {
	t.Helper()

	requests := map[string]float64{}
	durations := map[string]uint64{}
	roaming := map[string]bool{}

	for _, fam := range families {
		for _, m := range fam.GetMetric() {
			switch fam.GetName() {
			case "app_peer_requests_total":
				requests[labelValue(m.GetLabel(), "interface")+"/"+labelValue(m.GetLabel(), "result")] = m.GetCounter().GetValue()
			case "app_peer_request_duration_seconds":
				durations[labelValue(m.GetLabel(), "interface")] = m.GetHistogram().GetSampleCount()
			case "app_registration_attempts_total":
				roaming[labelValue(m.GetLabel(), "rat")+"/"+labelValue(m.GetLabel(), "roaming")] = true
			}
		}
	}

	for _, iface := range []string{PeerN32, PeerS6a} {
		for _, result := range []string{PeerSuccess, PeerFailure, PeerError, PeerTimeout} {
			if _, ok := requests[iface+"/"+result]; !ok {
				t.Errorf("app_peer_requests_total{interface=%q,result=%q} is not initialized", iface, result)
			}
		}

		if _, ok := durations[iface]; !ok {
			t.Errorf("app_peer_request_duration_seconds{interface=%q} is not initialized", iface)
		}
	}

	if got := requests[PeerN32+"/"+PeerTimeout]; got != 1 {
		t.Errorf("n32 timeouts = %v, want 1", got)
	}

	if got := durations[PeerN32]; got != 1 {
		t.Errorf("n32 duration samples = %d, want 1", got)
	}

	for _, key := range []string{RAT4G + "/" + RoamingFalse, RAT5G + "/" + RoamingTrue} {
		if !roaming[key] {
			t.Errorf("app_registration_attempts_total has no series for rat/roaming %s", key)
		}
	}
}

func TestRoamingPartnerGauge(t *testing.T) {
	RegisterRoamingPartnerGauge(func() int { return 2 }, nil)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	want := map[string]float64{PeerN32: 2, PeerS6a: 0}

	for _, fam := range families {
		if fam.GetName() != "app_connected_roaming_partners" {
			continue
		}

		for _, m := range fam.GetMetric() {
			iface := labelValue(m.GetLabel(), "interface")

			exp, ok := want[iface]
			if !ok {
				t.Errorf("unexpected interface %q", iface)
				continue
			}

			if got := m.GetGauge().GetValue(); got != exp {
				t.Errorf("app_connected_roaming_partners{interface=%q} = %v, want %v", iface, got, exp)
			}

			delete(want, iface)
		}
	}

	for iface := range want {
		t.Errorf("missing app_connected_roaming_partners{interface=%q}", iface)
	}
}
