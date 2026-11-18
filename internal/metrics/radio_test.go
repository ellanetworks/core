// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package metrics

import (
	"fmt"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type knownRadios struct {
	mu  sync.Mutex
	ids []string
}

func (k *knownRadios) set(ids ...string) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.ids = ids
}

func (k *knownRadios) list() []string {
	k.mu.Lock()
	defer k.mu.Unlock()

	return append([]string(nil), k.ids...)
}

func newTestRadioSeries(t *testing.T, known *knownRadios) (*radioSeries, *prometheus.Registry) {
	t.Helper()

	s := newRadioSeries(known.list)
	reg := prometheus.NewRegistry()
	reg.MustRegister(s)

	return s, reg
}

func gatherFamily(t *testing.T, reg *prometheus.Registry, name string) []*dto.Metric {
	t.Helper()

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, f := range families {
		if f.GetName() == name {
			return f.GetMetric()
		}
	}

	return nil
}

func radioIDs(metrics []*dto.Metric) map[string]bool {
	out := map[string]bool{}
	for _, m := range metrics {
		out[labelValue(m.GetLabel(), radioIDLabel)] = true
	}

	return out
}

const (
	gnbA = "gNB:001-01:000001@22"
	enbB = "eNB:001-01:MacroeNB-00002"
)

func TestRadioSeriesLabels(t *testing.T) {
	known := &knownRadios{}
	known.set(gnbA, enbB)

	s, reg := newTestRadioSeries(t, known)

	s.signalingMessage(gnbA, "inbound", "HandoverRequired")
	s.signalingMessage(gnbA, "inbound", "HandoverRequired")
	s.registrationAttempt(enbB, "Attach", ResultReject)

	signaling := gatherFamily(t, reg, "app_radio_signaling_messages_total")
	if len(signaling) != 1 {
		t.Fatalf("signaling series = %d, want 1", len(signaling))
	}

	m := signaling[0]
	if got := labelValue(m.GetLabel(), radioIDLabel); got != gnbA {
		t.Errorf("radio_id = %q, want %q", got, gnbA)
	}

	if got := labelValue(m.GetLabel(), "direction"); got != "inbound" {
		t.Errorf("direction = %q, want inbound", got)
	}

	if got := labelValue(m.GetLabel(), "type"); got != "HandoverRequired" {
		t.Errorf("type = %q, want HandoverRequired", got)
	}

	if got := m.GetCounter().GetValue(); got != 2 {
		t.Errorf("value = %v, want 2", got)
	}

	if len(m.GetLabel()) != 3 {
		t.Errorf("labels = %v, want radio_id, direction and type only", m.GetLabel())
	}

	registrations := gatherFamily(t, reg, "app_radio_registration_attempts_total")
	if len(registrations) != 1 {
		t.Fatalf("registration series = %d, want 1", len(registrations))
	}

	r := registrations[0]
	if got := labelValue(r.GetLabel(), radioIDLabel); got != enbB {
		t.Errorf("radio_id = %q, want %q", got, enbB)
	}

	if got := labelValue(r.GetLabel(), "type"); got != "Attach" {
		t.Errorf("type = %q, want Attach", got)
	}

	if got := labelValue(r.GetLabel(), "result"); got != ResultReject {
		t.Errorf("result = %q, want %q", got, ResultReject)
	}

	if len(r.GetLabel()) != 3 {
		t.Errorf("labels = %v, want radio_id, type and result only", r.GetLabel())
	}
}

func TestRadioSeriesSkipsUnidentifiedRadio(t *testing.T) {
	known := &knownRadios{}
	s, reg := newTestRadioSeries(t, known)

	s.signalingMessage("", "inbound", "NGSetupRequest")
	s.registrationAttempt("", "Initial Registration", ResultAccept)

	if got := gatherFamily(t, reg, "app_radio_signaling_messages_total"); len(got) != 0 {
		t.Errorf("signaling series = %d, want 0", len(got))
	}

	if got := gatherFamily(t, reg, "app_radio_registration_attempts_total"); len(got) != 0 {
		t.Errorf("registration series = %d, want 0", len(got))
	}
}

func TestRadioSeriesDeletedWhenRadioForgotten(t *testing.T) {
	known := &knownRadios{}
	known.set(gnbA, enbB)

	s, reg := newTestRadioSeries(t, known)

	s.signalingMessage(gnbA, "outbound", "NGSetupResponse")
	s.registrationAttempt(gnbA, "Initial Registration", ResultAccept)
	s.signalingMessage(enbB, "inbound", "S1SetupRequest")
	s.registrationAttempt(enbB, "Attach", ResultAccept)

	known.set(enbB)

	for _, name := range []string{"app_radio_signaling_messages_total", "app_radio_registration_attempts_total"} {
		got := radioIDs(gatherFamily(t, reg, name))
		if got[gnbA] {
			t.Errorf("%s kept a series for a forgotten radio", name)
		}

		if !got[enbB] {
			t.Errorf("%s lost the series of a known radio", name)
		}
	}

	known.set(gnbA, enbB)
	s.signalingMessage(gnbA, "inbound", "NGSetupRequest")

	for _, m := range gatherFamily(t, reg, "app_radio_signaling_messages_total") {
		if labelValue(m.GetLabel(), radioIDLabel) == gnbA && m.GetCounter().GetValue() != 1 {
			t.Errorf("a returning radio resumed at %v, want a fresh series at 1", m.GetCounter().GetValue())
		}
	}
}

func TestRadioSeriesBoundedByKnownRadios(t *testing.T) {
	known := &knownRadios{}
	s, reg := newTestRadioSeries(t, known)

	for i := range 1000 {
		id := fmt.Sprintf("gNB:001-01:%06d@22", i)
		known.set(id)
		s.signalingMessage(id, "inbound", "NGSetupRequest")
		s.registrationAttempt(id, "Initial Registration", ResultReject)
	}

	last := fmt.Sprintf("gNB:001-01:%06d@22", 999)

	for _, name := range []string{"app_radio_signaling_messages_total", "app_radio_registration_attempts_total"} {
		got := radioIDs(gatherFamily(t, reg, name))
		if len(got) != 1 || !got[last] {
			t.Errorf("%s radios = %v, want only %s", name, got, last)
		}
	}

	tracked := 0

	s.ids.Range(func(_, _ any) bool {
		tracked++

		return true
	})

	if tracked != 1 {
		t.Errorf("tracked radios = %d, want 1", tracked)
	}
}

func TestRadioSeriesPrunesWithoutScrape(t *testing.T) {
	known := &knownRadios{}
	known.set(gnbA)

	s := newRadioSeries(known.list)

	s.signalingMessage(gnbA, "inbound", "NGSetupRequest")

	known.set(enbB)
	s.prune()

	if n := s.signaling.DeletePartialMatch(prometheus.Labels{radioIDLabel: gnbA}); n != 0 {
		t.Errorf("found %d series of a forgotten radio after pruning, want 0", n)
	}
}

func TestRadioHelpersBeforeRegistration(t *testing.T) {
	RadioSignalingMessage(gnbA, "inbound", "NGSetupRequest")
	RadioRegistrationAttempt(gnbA, "Initial Registration", ResultAccept)
}
