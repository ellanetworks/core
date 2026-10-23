// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
	"github.com/ellanetworks/core/internal/udm"
	"go.uber.org/zap"
)

var cxApplication = diameter.Application{ID: cx.ApplicationID, VendorID: tgpp.VendorID}

type fakeSCSCF struct {
	pushResult   tgpp.Result
	rtrResult    tgpp.Result
	rtrEmergency []cx.EmergencyIdentity

	mu           sync.Mutex
	terminations []cx.ReasonCode
}

func (f *fakeSCSCF) ServeDiameter(_ context.Context, _ *diameter.Conn, req *diameter.Message) *diameter.Message {
	switch req.CommandCode {
	case cx.CommandPushProfile:
		if f.pushResult.Failure() {
			return cx.NewErrorAnswer(req, cscfIdentity, &cx.ResultError{Result: f.pushResult}, 0)
		}

		ans, _ := cx.NewPushProfileAnswer(req, cscfIdentity, cx.PushProfile{Result: f.pushResult})

		return ans
	case cx.CommandRegistrationTermination:
		r, err := cx.ParseRegistrationTerminationRequest(req)
		if err != nil {
			return cx.NewErrorAnswer(req, cscfIdentity, err, 0)
		}

		f.mu.Lock()
		f.terminations = append(f.terminations, r.Reason.Code)
		f.mu.Unlock()

		if f.rtrResult.Failure() {
			ans, _ := cx.NewRegistrationTerminationErrorAnswer(req, cscfIdentity, cx.RegistrationTerminationError{ResultError: cx.ResultError{Result: f.rtrResult}, EmergencyIdentities: f.rtrEmergency})

			return ans
		}

		ans, _ := cx.NewRegistrationTerminationAnswer(req, cscfIdentity, cx.RegistrationTermination{Result: f.rtrResult.OrSuccess(), EmergencyIdentities: f.rtrEmergency})

		return ans
	default:
		return cx.NewErrorAnswer(req, cscfIdentity, &cx.ResultError{Result: tgpp.Result{Code: diameter.ResultCommandUnsupported}}, 0)
	}
}

func (f *fakeSCSCF) Terminations() []cx.ReasonCode {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]cx.ReasonCode(nil), f.terminations...)
}

type nodePeers struct{ node *diameter.Node }

func (p nodePeers) Node() *diameter.Node { return p.node }

func newTestDiameterNode(t *testing.T, id diameter.Identity, addr netip.Addr, handler diameter.Handler) *diameter.Node {
	t.Helper()

	id.HostIPAddresses = []netip.Addr{addr}
	id.VendorID = tgpp.VendorID
	id.ProductName = "test"

	n, err := diameter.New(diameter.Config{
		Identity:                    id,
		Handler:                     handler,
		AcceptUnknownPeers:          true,
		UnknownPeerApplications:     []diameter.Application{cxApplication},
		UnknownPeerSupportedVendors: []uint32{tgpp.VendorID},
		HandshakeTimeout:            5 * time.Second,
		ReconnectInterval:           50 * time.Millisecond,
		MaxReconnectInterval:        200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("diameter.New: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = n.Shutdown(ctx)
	})

	return n
}

func connectedHSS(t *testing.T, store *fakeIMSStore, scscf *fakeSCSCF) *hss.HSS {
	t.Helper()

	loopback := netip.MustParseAddr("127.0.0.1")

	hssNode := newTestDiameterNode(t, hssIdentity, loopback, diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return c.Answer(req, diameter.ResultCommandUnsupported)
	}))

	ln, err := net.ListenTCP("tcp", net.TCPAddrFromAddrPort(netip.AddrPortFrom(loopback, 0)))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	served := make(chan error, 1)

	go func() { served <- hssNode.Serve(diameter.NewTCPListener(ln)) }()

	t.Cleanup(func() {
		_ = ln.Close()

		if err := <-served; err != nil {
			t.Logf("serve: %v", err)
		}
	})

	cscfNode := newTestDiameterNode(t, cscfIdentity, loopback, scscf)

	port := netip.MustParseAddrPort(ln.Addr().String()).Port()
	if err := cscfNode.SetPeers([]diameter.Peer{{
		ID: "hss", Addresses: []netip.Addr{loopback}, Transports: []diameter.Transport{diameter.TransportTCP},
		Applications: []diameter.Application{cxApplication}, SupportedVendors: []uint32{tgpp.VendorID}, Dial: &diameter.Dial{Port: port},
	}}); err != nil {
		t.Fatalf("SetPeers: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)

	for {
		if s, ok := cscfNode.Peer("hss"); ok && s.State == diameter.PeerOpen {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the S-CSCF did not connect to the HSS")
		}

		time.Sleep(10 * time.Millisecond)
	}

	return hss.New(store, udm.NewIMSCredentials(store), nodePeers{node: hssNode}, zap.NewNop())
}

func TestPushProfileAnswer_TS29228_6_2_2_1(t *testing.T) {
	registered := hss.Registration{State: hss.Registered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm}

	for _, tc := range []struct {
		name             string
		result           tgpp.Result
		rtrResult        tgpp.Result
		wantErr          bool
		wantTerminations []cx.ReasonCode
		wantCleared      bool
	}{
		{name: "accepted", result: tgpp.Result{Code: diameter.ResultSuccess}},
		{name: "profile push unsupported", result: tgpp.Result{Code: diameter.ResultCommandUnsupported}},
		{name: "unsupported user data", result: tgpp.Experimental(tgpp.ResultErrorNotSupportedUserData), wantTerminations: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true},
		{name: "too much data", result: tgpp.Experimental(tgpp.ResultErrorTooMuchData), wantTerminations: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true},
		{name: "unsupported user data, de-registration refused", result: tgpp.Experimental(tgpp.ResultErrorNotSupportedUserData), rtrResult: tgpp.Result{Code: diameter.ResultUnableToComply}, wantErr: true, wantTerminations: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true},
		{name: "user unknown", result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), wantCleared: true},
		{name: "unable to comply", result: tgpp.Result{Code: diameter.ResultUnableToComply}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			store.registrations[testIMSI] = registered
			scscf := &fakeSCSCF{pushResult: tc.result, rtrResult: tc.rtrResult}

			err := connectedHSS(t, store, scscf).PushProfile(context.Background(), testIMSI)
			if (err != nil) != tc.wantErr {
				t.Fatalf("PushProfile: %v, want error %t", err, tc.wantErr)
			}

			if got := scscf.Terminations(); len(got) != len(tc.wantTerminations) || (len(got) > 0 && got[0] != tc.wantTerminations[0]) {
				t.Fatalf("registration terminations = %v, want %v", got, tc.wantTerminations)
			}

			_, kept := store.registrations[testIMSI]
			if kept == tc.wantCleared {
				t.Fatalf("registration kept = %t, want %t", kept, !tc.wantCleared)
			}
		})
	}
}

func TestRegistrationTermination_TS29228_6_1_3_1(t *testing.T) {
	registered := hss.Registration{State: hss.Registered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm}
	emergency := []cx.EmergencyIdentity{{PrivateIdentity: testIMSI + "@" + testIMSDomain, PublicIdentity: "tel:+" + testMSISDN}}
	unableToComply := tgpp.Result{Code: diameter.ResultUnableToComply}

	for _, tc := range []struct {
		name         string
		msisdn       string
		rtrResult    tgpp.Result
		rtrEmergency []cx.EmergencyIdentity
		wantReason   cx.ReasonCode
		wantErr      bool
		want         *hss.Registration
	}{
		{name: "server change answered", msisdn: "15559990000", wantReason: cx.ReasonServerChange},
		{name: "server change refused", msisdn: "15559990000", rtrResult: unableToComply, wantReason: cx.ReasonServerChange, wantErr: true},
		{name: "permanent termination answered", wantReason: cx.ReasonPermanentTermination},
		{name: "permanent termination refused", rtrResult: unableToComply, wantReason: cx.ReasonPermanentTermination, wantErr: true, want: &registered},
		{
			name: "permanent termination of an emergency registration", rtrResult: unableToComply, rtrEmergency: emergency, wantReason: cx.ReasonPermanentTermination,
			want: &hss.Registration{State: hss.Unregistered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm},
		},
		{
			name: "permanent termination with limited success", rtrResult: tgpp.Result{Code: diameter.ResultLimitedSuccess}, rtrEmergency: emergency, wantReason: cx.ReasonPermanentTermination,
			want: &hss.Registration{State: hss.Unregistered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			store.registrations[testIMSI] = registered
			store.subscribers[testIMSI].MSISDN = tc.msisdn
			scscf := &fakeSCSCF{rtrResult: tc.rtrResult, rtrEmergency: tc.rtrEmergency}

			err := connectedHSS(t, store, scscf).IdentitiesChanged(context.Background(), testIMSI, testMSISDN)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IdentitiesChanged: %v, want error %t", err, tc.wantErr)
			}

			if got := scscf.Terminations(); len(got) != 1 || got[0] != tc.wantReason {
				t.Fatalf("registration terminations = %v, want [%v]", got, tc.wantReason)
			}

			got, kept := store.registrations[testIMSI]
			switch {
			case tc.want == nil && kept:
				t.Fatalf("registration = %+v, want it cleared", got)
			case tc.want != nil && (!kept || got != *tc.want):
				t.Fatalf("registration = %+v (kept %t), want %+v", got, kept, *tc.want)
			}
		})
	}
}
