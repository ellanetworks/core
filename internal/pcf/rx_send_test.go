// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf_test

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/rx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
)

type liveDiameter struct{ node *diameter.Node }

func (d liveDiameter) Node() *diameter.Node { return d.node }

func (liveDiameter) Handle(uint32, uint32, diameter.Handler) {}

type recordingAF struct {
	mu   sync.Mutex
	asrs []*diameter.Message
	rars []*diameter.Message
}

func (a *recordingAF) received(asr bool) []*diameter.Message {
	a.mu.Lock()
	defer a.mu.Unlock()

	if asr {
		return slices.Clone(a.asrs)
	}

	return slices.Clone(a.rars)
}

func (a *recordingAF) await(t *testing.T, asr bool) *diameter.Message {
	t.Helper()

	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if got := a.received(asr); len(got) > 0 {
			return got[0]
		}
	}

	t.Fatal("the AF received no request")

	return nil
}

func connectAF(t *testing.T, p *pcf.PCF) *recordingAF {
	t.Helper()

	loopback := netip.MustParseAddr("127.0.0.1")
	rxApp := diameter.Application{ID: rx.ApplicationID, VendorID: tgpp.VendorID}
	af := &recordingAF{}

	mux := diameter.NewMux()
	mux.Handle(rx.ApplicationID, rx.CommandAbortSession, diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		af.mu.Lock()
		af.asrs = append(af.asrs, req)
		af.mu.Unlock()

		ans, _ := rx.NewAbortSessionAnswer(req, c.LocalIdentity(), rx.AbortSessionAnswer{Result: success})

		return ans
	}))
	mux.Handle(rx.ApplicationID, rx.CommandReAuth, diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		af.mu.Lock()
		af.rars = append(af.rars, req)
		af.mu.Unlock()

		ans, _ := rx.NewReAuthAnswer(req, c.LocalIdentity(), rx.ReAuthAnswer{Result: success})

		return ans
	}))

	afNode, err := diameter.New(diameter.Config{
		Identity:                diameter.Identity{OriginHost: afIdentity.OriginHost, OriginRealm: afIdentity.OriginRealm, HostIPAddresses: []netip.Addr{loopback}, ProductName: "af"},
		Handler:                 mux,
		AcceptUnknownPeers:      true,
		UnknownPeerApplications: []diameter.Application{rxApp},
	})
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: loopback.AsSlice()})
	if err != nil {
		t.Fatal(err)
	}

	go func() { _ = afNode.Serve(diameter.NewTCPListener(ln)) }()

	pcfNode, err := diameter.New(diameter.Config{
		Identity: diameter.Identity{OriginHost: pcfIdentity.OriginHost, OriginRealm: pcfIdentity.OriginRealm, HostIPAddresses: []netip.Addr{loopback}, ProductName: "pcf"},
		Handler:  diameter.NewMux(),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = pcfNode.Shutdown(context.Background())
		_ = afNode.Shutdown(context.Background())
		_ = ln.Close()
	})

	port := uint16(ln.Addr().(*net.TCPAddr).Port)
	if err := pcfNode.SetPeers([]diameter.Peer{{
		ID: "af", Host: afIdentity.OriginHost, Addresses: []netip.Addr{loopback},
		Transports: []diameter.Transport{diameter.TransportTCP}, Dial: &diameter.Dial{Port: port}, Applications: []diameter.Application{rxApp},
	}}); err != nil {
		t.Fatal(err)
	}

	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if s, ok := pcfNode.Peer("af"); ok && s.State == diameter.PeerOpen {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the AF peer never opened")
		}
	}

	p.Attach(liveDiameter{node: pcfNode}, nil)

	return af
}

func TestLosingEveryFlowSendsTheAFAnAbort(t *testing.T) {
	p, n := newEnforcedPCF(t)
	af := connectAF(t, p)

	requireResult(t, aa(t, p, "af;call", callAAR(50000, rx.ServiceInfoFinal)), success)
	d := n.next(t)

	p.ReportEnforcementFailure(testRef, failedRules(d.Rules...), smf.BearerReleased)

	req := af.await(t, true)

	asr, err := rx.ParseAbortSessionRequest(req)
	if err != nil {
		t.Fatalf("parse ASR: %v", err)
	}

	if id := tgpp.ParseEnvelope(req); id.SessionID != "af;call" || asr.Cause != rx.AbortBearerReleased {
		t.Fatalf("ASR for session %q with cause %s, want af;call with BEARER_RELEASED (TS 29.214 §4.4.6.1)", id.SessionID, asr.Cause)
	}
}

func TestPartialFailureReportsTheFailedFlowsAfterAnUpdate(t *testing.T) {
	p, n := newEnforcedPCF(t)
	af := connectAF(t, p)

	r := callAAR(50000, rx.ServiceInfoFinal)
	second := r.MediaComponents[0]
	second.Number = 2
	second.SubComponents = []rx.MediaSubComponent{{
		FlowNumber: 1,
		FlowDescriptions: []string{
			"permit out 17 from 192.0.2.20 49002 to 10.60.0.1 50004",
			"permit in 17 from 10.60.0.1 50004 to 192.0.2.20 49002",
		},
	}}
	r.MediaComponents = append(r.MediaComponents, second)

	requireResult(t, aa(t, p, "af;call", r), success)
	d := n.next(t)

	update := rx.RequestUpdate
	requireResult(t, aa(t, p, "af;call", rx.AARequest{RequestType: &update, SpecificActions: []rx.SpecificAction{rx.ActionChargingCorrelationExchange}}), success)

	i := slices.IndexFunc(d.Rules, func(r smf.PCCRule) bool { return r.ID == "af;call#2.1" })
	if i < 0 {
		t.Fatalf("rules %+v carry no rule for the second component", d.Rules)
	}

	p.ReportEnforcementFailure(testRef, failedRules(d.Rules[i]), smf.ResourcesNotAllocated)

	rar, err := rx.ParseReAuthRequest(af.await(t, false))
	if err != nil {
		t.Fatalf("parse RAR: %v", err)
	}

	if !slices.Equal(rar.SpecificActions, []rx.SpecificAction{rx.ActionIndicationOfFailedResourcesAllocation}) ||
		len(rar.Flows) != 1 || rar.Flows[0].MediaComponentNumber != 2 || !slices.Equal(rar.Flows[0].FlowNumbers, []uint32{1}) {
		t.Fatalf("RAR %+v, want INDICATION_OF_FAILED_RESOURCES_ALLOCATION for flow 2.1: the update kept the subscription (TS 29.214 §4.4.1)", rar)
	}

	if got := af.received(true); len(got) != 0 {
		t.Fatal("the AF was told the session was aborted although one component kept its rule")
	}
}

func TestRejectedUpdateKeepsNoSubscription(t *testing.T) {
	p, n := newEnforcedPCF(t)
	af := connectAF(t, p)

	r := callAAR(50000, rx.ServiceInfoFinal)
	r.SpecificActions = nil
	extra := r.MediaComponents[0]
	extra.Number = 2
	extra.SubComponents = []rx.MediaSubComponent{{
		FlowNumber: 1,
		FlowDescriptions: []string{
			"permit out 17 from 192.0.2.20 49002 to 10.60.0.1 50004",
			"permit in 17 from 10.60.0.1 50004 to 192.0.2.20 49002",
		},
	}}
	r.MediaComponents = append(r.MediaComponents, extra)

	requireResult(t, aa(t, p, "af;call", r), success)
	d := n.next(t)

	bad := callAAR(50000, rx.ServiceInfoFinal)
	update := rx.RequestUpdate
	bad.RequestType = &update
	bad.MediaComponents[0].SubComponents[0].FlowDescriptions = []string{"deny out 17 from 192.0.2.10 49000 to 10.60.0.1 50000"}
	bad.SpecificActions = []rx.SpecificAction{rx.ActionIndicationOfFailedResourcesAllocation}
	requireResult(t, aa(t, p, "af;call", bad), tgpp.Experimental(tgpp.ResultFilterRestrictions))

	second := slices.IndexFunc(d.Rules, func(r smf.PCCRule) bool { return r.ID == "af;call#2.1" })
	first := slices.IndexFunc(d.Rules, func(r smf.PCCRule) bool { return r.ID != "af;call#2.1" })

	p.ReportEnforcementFailure(testRef, failedRules(d.Rules[second]), smf.ResourcesNotAllocated)
	n.next(t)
	p.ReportEnforcementFailure(testRef, failedRules(d.Rules[first]), smf.ResourcesNotAllocated)
	af.await(t, true)

	if got := af.received(false); len(got) != 0 {
		t.Fatal("the AF was sent a RAR for an event it subscribed to in a rejected AAR (TS 29.214 §5.5.3)")
	}
}
