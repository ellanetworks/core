// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/s1ap"
)

var mmeTracePLMN = models.PlmnID{Mcc: "001", Mnc: "01"}

func newTracedMMEUE(t *testing.T) (*MME, *tracing.TraceSessions, *UeContext, *UeConn, *captureConn) {
	t.Helper()

	m := New(nil, nil, nil)
	sessions := tracing.NewTraceSessions(func() int { return 3 })
	m.EnableRANTrace(sessions)

	w := &captureConn{}
	c := m.NewUeConn(w, 9)

	ue := NewUeContext()
	ue.SetIMSIForTest("001010000000001")
	m.AttachUeConn(t.Context(), ue, c)

	return m, sessions, ue, c, w
}

func sentTraceStarts(t *testing.T, w *captureConn) []*s1ap.TraceStart {
	t.Helper()

	var out []*s1ap.TraceStart

	for _, b := range w.snapshot() {
		pdu, err := s1ap.Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*s1ap.InitiatingMessage)
		if !ok || im.ProcedureCode != s1ap.ProcTraceStart {
			continue
		}

		ts, err := s1ap.ParseTraceStart(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		out = append(out, ts)
	}

	return out
}

func TestMMETraceStart(t *testing.T) {
	m, sessions, ue, c, w := newTracedMMEUE(t)

	if !m.activateTraceSession(t.Context(), ue, mmeTracePLMN) {
		t.Fatal("trace session not activated")
	}

	ue.TraceConnection(t.Context())
	ue.TraceConnection(t.Context())

	starts := sentTraceStarts(t, w)
	if len(starts) != 1 {
		t.Fatalf("%d Trace Start messages, want 1 per connection", len(starts))
	}

	ts := starts[0]
	if ts.MMEUES1APID != c.MMEUES1APID || ts.ENBUES1APID != 9 || ts.TraceActivation == nil {
		t.Fatalf("Trace Start %+v", ts)
	}

	ta := ts.TraceActivation
	if [3]byte(ta.EUTRANTraceID[:3]) != [3]byte{0x00, 0xf1, 0x10} || ta.InterfacesToTrace != 0xf8 || ta.TraceDepth != s1ap.TraceDepthMinimum {
		t.Fatalf("Trace Activation %+v", ta)
	}

	if id, ok := c.RANTraceID(); !ok || id != tracing.RANTraceID(ta.EUTRANTraceID) {
		t.Fatalf("connection carries %x, sent %x", id, ta.EUTRANTraceID)
	}

	m.RemoveUe(t.Context(), ue)

	if sessions.Active() != 0 {
		t.Fatalf("%d active sessions after the UE context was removed", sessions.Active())
	}
}

func TestMMEAndAMFShareTraceReference(t *testing.T) {
	m, sessions, ue, _, _ := newTracedMMEUE(t)

	m.activateTraceSession(t.Context(), ue, mmeTracePLMN)

	var amfSide tracing.UETrace

	if _, err := amfSide.Activate(sessions, "001010000000001", [3]byte{0x00, 0xf1, 0x10}); err != nil {
		t.Fatal(err)
	}

	mmeID, _ := ue.trace.NewRecording()
	amfID, _ := amfSide.NewRecording()

	if [6]byte(mmeID[:6]) != [6]byte(amfID[:6]) || mmeID == amfID {
		t.Fatalf("MME %x and AMF %x: want one Trace Reference and distinct TRSRs", mmeID, amfID)
	}

	m.RemoveUe(t.Context(), ue)

	if sessions.Active() != 1 {
		t.Fatalf("%d active sessions while the AMF still holds the subscriber", sessions.Active())
	}

	amfSide.Release()

	if sessions.Active() != 0 {
		t.Fatalf("%d active sessions after both released", sessions.Active())
	}
}

func TestMMEHandoverTraceActivationFollowsSource(t *testing.T) {
	m, _, ue, c, _ := newTracedMMEUE(t)

	m.activateTraceSession(t.Context(), ue, mmeTracePLMN)
	ue.TraceConnection(t.Context())

	target := &UeConn{m: m}
	target.setConn(&captureConn{})
	target.trace.Inherit(&c.trace)

	ta, err := target.HandoverTraceActivation()
	if err != nil || ta == nil {
		t.Fatalf("no Trace Activation for the target: %v", err)
	}

	source, _ := c.RANTraceID()
	if tracing.RANTraceID(ta.EUTRANTraceID) != source {
		t.Fatalf("target %x, source %x", ta.EUTRANTraceID, source)
	}
}

func TestMMETraceFailure(t *testing.T) {
	m, _, ue, c, w := newTracedMMEUE(t)

	m.activateTraceSession(t.Context(), ue, mmeTracePLMN)
	ue.TraceConnection(t.Context())

	id, _ := c.RANTraceID()
	other := id
	other[7]++

	c.TraceFailed(t.Context(), s1ap.EUTRANTraceID(other), s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5})
	ue.TraceConnection(t.Context())

	if n := len(sentTraceStarts(t, w)); n != 1 {
		t.Fatalf("%d Trace Start messages after a failure for another Trace ID, want 1", n)
	}

	c.TraceFailed(t.Context(), s1ap.EUTRANTraceID(id), s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5})
	ue.TraceConnection(t.Context())

	starts := sentTraceStarts(t, w)
	if len(starts) != 2 || tracing.RANTraceID(starts[1].TraceActivation.EUTRANTraceID) != id {
		t.Fatalf("%d Trace Start messages; want the connection's Trace ID %x resent", len(starts), id)
	}
}

func TestMMENewConnectionGetsNewRecording(t *testing.T) {
	m, _, ue, c, w := newTracedMMEUE(t)

	m.activateTraceSession(t.Context(), ue, mmeTracePLMN)
	ue.TraceConnection(t.Context())

	first, _ := c.RANTraceID()

	next := m.NewUeConn(w, 10)
	m.AttachUeConn(t.Context(), ue, next)

	second, ok := next.RANTraceID()
	if !ok || [6]byte(second[:6]) != [6]byte(first[:6]) || second == first || !next.trace.Activated() {
		t.Fatalf("second connection %x, first %x", second, first)
	}
}

func TestMMEImplicitDetachReleasesTrace(t *testing.T) {
	m := New(nil, nil, nil)
	sessions := tracing.NewTraceSessions(func() int { return 3 })
	m.EnableRANTrace(sessions)

	ue := NewUeContext()
	ue.SetIMSIForTest("001010000000001")

	if !m.activateTraceSession(t.Context(), ue, mmeTracePLMN) {
		t.Fatal("trace session not activated")
	}

	m.onImplicitDetachExpiry(ue, ue.idleGen)

	if sessions.Active() != 0 {
		t.Fatalf("%d active sessions after implicit detach", sessions.Active())
	}
}

func TestMMEUETraceContextMatchesRadio(t *testing.T) {
	m, _, ue, c, w := newTracedMMEUE(t)

	m.activateTraceSession(t.Context(), ue, mmeTracePLMN)
	ue.TraceConnection(t.Context())

	want, _ := c.RANTraceID()

	if got, ok := tracing.PinnedTraceID(m.UETraceContext(t.Context(), w, c.MMEUES1APID, false)); !ok || got != want.OTelTraceID() {
		t.Fatalf("pinned %s, want %s", got, want.OTelTraceID())
	}

	if _, ok := tracing.PinnedTraceID(m.UETraceContext(t.Context(), &captureConn{}, c.MMEUES1APID, false)); ok {
		t.Fatal("a message from another eNB pinned this UE's trace")
	}

	if _, ok := tracing.PinnedTraceID(m.UETraceContext(t.Context(), &captureConn{}, c.MMEUES1APID, true)); !ok {
		t.Fatal("a path switch from the target eNB did not pin this UE's trace")
	}
}
