// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
)

type traceCapture struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (c *traceCapture) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.msgs = append(c.msgs, append([]byte(nil), b...))

	return len(b), nil
}

func (c *traceCapture) WriteMsgWait(_ context.Context, b []byte, info *sctp.SndRcvInfo) (int, error) {
	return c.WriteMsg(b, info)
}

func (c *traceCapture) traceStarts(t *testing.T) []*ngap.TraceStart {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()

	var out []*ngap.TraceStart

	for _, b := range c.msgs {
		pdu, err := ngap.Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		im, ok := pdu.(*ngap.InitiatingMessage)
		if !ok || im.ProcedureCode != ngap.ProcTraceStart {
			continue
		}

		ts, err := ngap.ParseTraceStart(im.Value)
		if err != nil {
			t.Fatal(err)
		}

		out = append(out, ts)
	}

	return out
}

var tracePLMN = models.PlmnID{Mcc: "001", Mnc: "01"}

func newTracedUE(t *testing.T) (*AMF, *tracing.TraceSessions, *UeContext, *UeConn, *traceCapture) {
	t.Helper()

	a := New(nil, nil, nil)
	sessions := tracing.NewTraceSessions(tracing.TraceConfig{TCEAddress: netip.MustParseAddr("10.0.0.1")}, func() int { return 3 })
	a.EnableRANTrace(sessions)

	w := &traceCapture{}
	radio := &Radio{Conn: w, amf: a}
	conn := NewUeConnForTest(radio, 2, 1)

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	ue := NewUeContext()
	ue.SetSupi(supi)
	a.AttachUeConn(t.Context(), ue, conn)

	return a, sessions, ue, conn, w
}

func TestTraceStartCarriesSessionAndConnection(t *testing.T) {
	a, sessions, ue, conn, w := newTracedUE(t)

	if !a.activateTraceSession(t.Context(), ue, tracePLMN) {
		t.Fatal("trace session not activated")
	}

	ue.StartTraceRecording(t.Context())

	starts := w.traceStarts(t)
	if len(starts) != 1 {
		t.Fatalf("%d Trace Start messages, want 1", len(starts))
	}

	ts := starts[0]
	if ts.AMFUENGAPID != 1 || ts.RANUENGAPID != 2 || ts.TraceActivation == nil {
		t.Fatalf("Trace Start %+v", ts)
	}

	ta := ts.TraceActivation
	if [3]byte(ta.NGRANTraceID[:3]) != [3]byte{0x00, 0xf1, 0x10} {
		t.Fatalf("Trace ID %x does not carry the operator PLMN", ta.NGRANTraceID)
	}

	if ta.InterfacesToTrace != 0xf8 || ta.TraceDepth != ngap.TraceDepthMinimum {
		t.Fatalf("interfaces %08b depth %d", ta.InterfacesToTrace, ta.TraceDepth)
	}

	if ip, _ := ta.TraceCollectionEntityIPAddress.IPs(); ip != netip.MustParseAddr("10.0.0.1") {
		t.Fatalf("TCE address %v", ip)
	}

	id, ok := conn.RANTraceID()
	if !ok || id != tracing.RANTraceID(ta.NGRANTraceID) {
		t.Fatalf("connection carries %x, sent %x", id, ta.NGRANTraceID)
	}

	if sessions.Active() != 1 {
		t.Fatalf("%d active sessions", sessions.Active())
	}
}

func TestOngoingRecordingSendsNoSecondTraceStart(t *testing.T) {
	a, _, ue, _, w := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.StartTraceRecording(t.Context())
	ue.StartTraceRecording(t.Context())

	if n := len(w.traceStarts(t)); n != 1 {
		t.Fatalf("%d Trace Start messages, want 1", n)
	}
}

func TestNewRecordingKeepsConnectionTraceID(t *testing.T) {
	a, _, ue, conn, w := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.StartTraceRecording(t.Context())

	first, _ := conn.RANTraceID()

	ue.StopTraceRecording()
	ue.StartTraceRecording(t.Context())

	starts := w.traceStarts(t)
	if len(starts) != 2 {
		t.Fatalf("%d Trace Start messages, want 2", len(starts))
	}

	if tracing.RANTraceID(starts[1].TraceActivation.NGRANTraceID) == first {
		t.Fatal("second recording session reused the TRSR")
	}

	if id, _ := conn.RANTraceID(); id != first {
		t.Fatalf("connection trace ID changed to %x", id)
	}
}

func TestTraceFailureResendsOnNextTrigger(t *testing.T) {
	a, _, ue, conn, w := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.StartTraceRecording(t.Context())

	first, _ := conn.RANTraceID()

	conn.TraceFailed(t.Context(), ngap.NGRANTraceID(first), ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverCancelled})

	if conn.trace.Activated() {
		t.Fatal("connection still activated after a Trace Failure Indication")
	}

	ue.StartTraceRecording(t.Context())

	starts := w.traceStarts(t)
	if len(starts) != 2 {
		t.Fatalf("%d Trace Start messages after the failure, want 2", len(starts))
	}

	if tracing.RANTraceID(starts[1].TraceActivation.NGRANTraceID) != first || !conn.trace.Activated() {
		t.Fatalf("resent %x, want the ongoing recording %x", starts[1].TraceActivation.NGRANTraceID, first)
	}

	conn.TraceFailed(t.Context(), ngap.NGRANTraceID(first), ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverCancelled})
	ue.StopTraceRecording()
	ue.StartTraceRecording(t.Context())

	starts = w.traceStarts(t)
	next := tracing.RANTraceID(starts[len(starts)-1].TraceActivation.NGRANTraceID)

	if id, _ := conn.RANTraceID(); id != next || id == first {
		t.Fatalf("connection carries %x, want the new recording %x", id, next)
	}
}

func TestHandoverTargetInheritsTrace(t *testing.T) {
	a, _, ue, conn, _ := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.StartTraceRecording(t.Context())

	target := NewUeConnForTest(&Radio{Conn: &traceCapture{}, amf: a}, 7, 8)
	target.InheritTrace(conn)

	ta, err := target.handoverTraceActivation()
	if err != nil || ta == nil {
		t.Fatalf("no Trace Activation for the target: %v", err)
	}

	source, _ := conn.RANTraceID()
	if tracing.RANTraceID(ta.NGRANTraceID) != source {
		t.Fatalf("target Trace ID %x, source %x", ta.NGRANTraceID, source)
	}
}

func TestNoTraceWithoutSession(t *testing.T) {
	_, _, ue, conn, w := newTracedUE(t)

	ue.StartTraceRecording(t.Context())

	if n := len(w.traceStarts(t)); n != 0 {
		t.Fatalf("%d Trace Start messages without a trace session", n)
	}

	if ta, err := conn.handoverTraceActivation(); ta != nil || err != nil {
		t.Fatalf("Trace Activation %+v err %v without a trace session", ta, err)
	}
}

func TestTraceDisabledActivatesNothing(t *testing.T) {
	a := New(nil, nil, nil)
	ue := NewUeContext()

	supi, _ := etsi.NewSUPIFromIMSI("001010000000001")
	ue.SetSupi(supi)

	if a.activateTraceSession(t.Context(), ue, tracePLMN) {
		t.Fatal("trace session activated with RAN trace disabled")
	}
}

func TestReleaseOnContextRemoval(t *testing.T) {
	a, sessions, ue, _, _ := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)

	a.DeregisterAndRemoveUeContext(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))

	if sessions.Active() != 0 {
		t.Fatalf("%d active sessions after the UE context was removed", sessions.Active())
	}
}
