// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"net"
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

func (c *traceCapture) LocalAddr() net.Addr {
	return &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, Port: 38412}
}

func (c *traceCapture) RemoteAddr() net.Addr {
	return &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.2")}}, Port: 38412}
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
	sessions := tracing.NewTraceSessions(func() int { return 3 })
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

	ue.TraceConnection(t.Context())
	ue.TraceConnection(t.Context())

	starts := w.traceStarts(t)
	if len(starts) != 1 {
		t.Fatalf("%d Trace Start messages, want 1 per connection", len(starts))
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

	if ip, _ := ta.TraceCollectionEntityIPAddress.IPs(); ip != netip.MustParseAddr("127.0.0.1") {
		t.Fatalf("TCE address %v", ip)
	}

	id, ok := conn.RANTraceID()
	if !ok || id != tracing.RANTraceID(ta.NGRANTraceID) || !conn.trace.Activated() {
		t.Fatalf("connection carries %x, sent %x", id, ta.NGRANTraceID)
	}

	if sessions.Active() != 1 {
		t.Fatalf("%d active sessions", sessions.Active())
	}
}

func TestNewConnectionGetsNewRecording(t *testing.T) {
	a, _, ue, conn, w := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.TraceConnection(t.Context())

	first, _ := conn.RANTraceID()

	next := NewUeConnForTest(&Radio{Conn: w, amf: a}, 3, 4)
	a.AttachUeConn(t.Context(), ue, next)

	starts := w.traceStarts(t)
	if len(starts) != 2 {
		t.Fatalf("%d Trace Start messages, want one per connection", len(starts))
	}

	second := tracing.RANTraceID(starts[1].TraceActivation.NGRANTraceID)
	if starts[1].AMFUENGAPID != 4 || [6]byte(second[:6]) != [6]byte(first[:6]) || second == first {
		t.Fatalf("second connection got %x, first %x: want the same Trace Reference and a new TRSR", second, first)
	}
}

func TestTraceFailureResendsOnlyForTheConnectionID(t *testing.T) {
	a, _, ue, conn, w := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.TraceConnection(t.Context())

	id, _ := conn.RANTraceID()
	cause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverCancelled}

	other := id
	other[7]++
	conn.TraceFailed(t.Context(), ngap.NGRANTraceID(other), cause)

	if !conn.trace.Activated() {
		t.Fatal("a failure for another Trace ID deactivated the connection")
	}

	conn.TraceFailed(t.Context(), ngap.NGRANTraceID(id), cause)

	if conn.trace.Activated() {
		t.Fatal("connection still activated after a failure for its Trace ID")
	}

	ue.TraceConnection(t.Context())

	starts := w.traceStarts(t)
	if len(starts) != 2 || tracing.RANTraceID(starts[1].TraceActivation.NGRANTraceID) != id {
		t.Fatalf("%d Trace Start messages; want the connection's Trace ID %x resent", len(starts), id)
	}

	if got, _ := conn.RANTraceID(); got != id || !conn.trace.Activated() {
		t.Fatalf("connection carries %x activated=%v", got, conn.trace.Activated())
	}
}

func TestHandoverTargetInheritsTrace(t *testing.T) {
	a, _, ue, conn, _ := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.TraceConnection(t.Context())

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

func TestRelocationTargetGetsNewRecording(t *testing.T) {
	a, _, ue, conn, _ := newTracedUE(t)

	a.activateTraceSession(t.Context(), ue, tracePLMN)
	ue.TraceConnection(t.Context())

	target := NewUeConnForTest(&Radio{Conn: &traceCapture{}, amf: a}, 7, 8)
	ue.traceRelocationTarget(target)

	ta, err := target.handoverTraceActivation()
	if err != nil || ta == nil {
		t.Fatalf("no Trace Activation for the relocation target: %v", err)
	}

	source, _ := conn.RANTraceID()
	got := tracing.RANTraceID(ta.NGRANTraceID)

	if got == source || [6]byte(got[:6]) != [6]byte(source[:6]) {
		t.Fatalf("relocation target %x, source %x: want the same Trace Reference and a new TRSR", got, source)
	}
}

func TestNoTraceWithoutSession(t *testing.T) {
	_, _, ue, conn, w := newTracedUE(t)

	ue.TraceConnection(t.Context())

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

	if a.activateTraceSession(t.Context(), ue, tracePLMN) || a.RANTraceEnabled() {
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

func TestSupersedingContextKeepsTraceReference(t *testing.T) {
	a, sessions, old, conn, _ := newTracedUE(t)

	if err := a.CommitUEIdentity(t.Context(), old, MintAuthProofForInterworking()); err != nil {
		t.Fatal(err)
	}

	a.activateTraceSession(t.Context(), old, tracePLMN)
	old.TraceConnection(t.Context())

	before, _ := conn.RANTraceID()

	fresh := NewUeContext()
	fresh.SetSupi(old.Supi())

	if err := a.CommitUEIdentity(t.Context(), fresh, MintAuthProofForInterworking()); err != nil {
		t.Fatal(err)
	}

	after, ok := fresh.trace.NewRecording()
	if !ok || [6]byte(after[:6]) != [6]byte(before[:6]) {
		t.Fatalf("Trace Reference changed from %x to %x across the re-registration", before[:6], after[:6])
	}

	if sessions.Active() != 1 {
		t.Fatalf("%d active sessions after the supersede", sessions.Active())
	}
}
