// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"slices"
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
	onPush       func()

	mu           sync.Mutex
	terminations []cx.ReasonCode
	pushes       []*diameter.Message
}

func (f *fakeSCSCF) ServeDiameter(_ context.Context, _ *diameter.Conn, req *diameter.Message) *diameter.Message {
	switch req.CommandCode {
	case cx.CommandPushProfile:
		f.mu.Lock()
		f.pushes = append(f.pushes, req)
		f.mu.Unlock()

		if f.onPush != nil {
			f.onPush()
		}

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

func (f *fakeSCSCF) Pushes() []*diameter.Message {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]*diameter.Message(nil), f.pushes...)
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

	return hss.New(store, udm.NewIMSCredentials(store), nodePeers{node: hssNode}, nil, zap.NewNop())
}

func assigned(t *testing.T, store *fakeIMSStore, imsi string, typ cx.AssignmentType) []byte {
	t.Helper()

	ans := serverAssignment(t, store, register(imsi, typ))
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		t.Fatalf("parse SAA: %v", err)
	}

	return sa.UserData
}

func registration(t *testing.T, store *fakeIMSStore, imsi string) (hss.Registration, bool) {
	t.Helper()

	store.mu.Lock()
	defer store.mu.Unlock()

	reg, ok := store.registrations[imsi]

	return reg, ok
}

func mcpttCriteria() []cx.InitialFilterCriteria {
	register := "REGISTER"

	return []cx.InitialFilterCriteria{{
		Trigger:           &cx.TriggerPoint{ServicePointTriggers: []cx.ServicePointTrigger{{Groups: []int32{0}, Method: &register}}},
		ApplicationServer: cx.ApplicationServer{ServerName: "sip:mcptt.example.org"},
	}}
}

func TestServerAssignmentAlwaysDownloadsTheProfile_TS29228_6_6(t *testing.T) {
	store := newFakeIMSStore()

	first := assigned(t, store, testIMSI, cx.AssignmentRegistration)
	reg, _ := registration(t, store, testIMSI)

	if reg.PrivateIdentity != impiOf(testIMSI) || reg.DefaultIdentity != "sip:+"+testMSISDN+"@"+testIMSDomain || reg.ProfileDigest == "" {
		t.Fatalf("registration = %+v, want what the SAA delivered", reg)
	}

	store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	reRegister := register(testIMSI, cx.AssignmentReRegistration)
	reRegister.UserDataAlreadyAvailable = true

	ans := serverAssignment(t, store, reRegister)

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil || len(sa.UserData) == 0 || bytes.Equal(sa.UserData, first) {
		t.Fatalf("re-registration SAA = %d octets, %v; want the changed profile despite USER_DATA_ALREADY_AVAILABLE", len(sa.UserData), err)
	}

	again, _ := registration(t, store, testIMSI)
	if again.ProfileDigest == reg.ProfileDigest {
		t.Fatalf("registration = %+v after the re-registration, want the digest of the new profile", again)
	}
}

func TestReconcile_TS29228_6_5_2_1(t *testing.T) {
	for _, tc := range []struct {
		name           string
		imsi           string
		assignment     cx.AssignmentType
		change         func(store *fakeIMSStore)
		emergency      bool
		wantPushes     int
		wantTerminated []cx.ReasonCode
		wantCleared    bool
	}{
		{name: "nothing changed", imsi: testIMSI, assignment: cx.AssignmentRegistration},
		{name: "filter criteria changed", imsi: testIMSI, assignment: cx.AssignmentRegistration, wantPushes: 1, change: func(store *fakeIMSStore) {
			store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}
		}},
		{name: "MSISDN changed and changed back", imsi: testIMSI, assignment: cx.AssignmentRegistration, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].MSISDN = "15559990000"
			store.subscribers[testIMSI].MSISDN = testMSISDN
		}},
		{name: "MSISDN changed", imsi: testIMSI, assignment: cx.AssignmentRegistration, wantTerminated: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].MSISDN = "15559990000"
		}},
		{name: "MSISDN removed", imsi: testIMSI, assignment: cx.AssignmentRegistration, wantTerminated: []cx.ReasonCode{cx.ReasonPermanentTermination}, wantCleared: true, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].MSISDN = ""
		}},
		{name: "IMS data network lost", imsi: testIMSI, assignment: cx.AssignmentRegistration, wantTerminated: []cx.ReasonCode{cx.ReasonPermanentTermination}, wantCleared: true, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].IMSDataNetwork = false
		}},
		{name: "unregistered, MSISDN changed", imsi: testIMSI, assignment: cx.AssignmentUnregisteredUser, wantPushes: 1, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].MSISDN = "15559990000"
		}},
		{name: "unregistered, MSISDN removed", imsi: testIMSI, assignment: cx.AssignmentUnregisteredUser, wantTerminated: []cx.ReasonCode{cx.ReasonPermanentTermination}, wantCleared: true, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].MSISDN = ""
		}},
		{name: "emergency registration, MSISDN added", imsi: testNoMSISDN, assignment: cx.AssignmentRegistration, wantTerminated: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true, change: func(store *fakeIMSStore) {
			store.subscribers[testNoMSISDN].MSISDN = "15559990000"
		}},
		{name: "emergency registration without an MSISDN", imsi: testNoMSISDN, assignment: cx.AssignmentRegistration, emergency: true, wantTerminated: []cx.ReasonCode{cx.ReasonPermanentTermination}},
		{name: "emergency registration with an MSISDN", imsi: testIMSI, assignment: cx.AssignmentRegistration, emergency: true, wantTerminated: []cx.ReasonCode{cx.ReasonPermanentTermination}, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].IMSDataNetwork = false
		}},
		{name: "unregistered emergency registration, IMS data network lost", imsi: testIMSI, assignment: cx.AssignmentUnregisteredUser, emergency: true, wantTerminated: []cx.ReasonCode{cx.ReasonPermanentTermination}, change: func(store *fakeIMSStore) {
			store.subscribers[testIMSI].IMSDataNetwork = false
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			scscf := &fakeSCSCF{}

			if tc.emergency {
				scscf.rtrResult = tgpp.Result{Code: diameter.ResultUnableToComply}
				scscf.rtrEmergency = []cx.EmergencyIdentity{{PrivateIdentity: impiOf(tc.imsi), PublicIdentity: "sip:" + impiOf(tc.imsi)}}
			}

			h := connectedHSS(t, store, scscf)

			assigned(t, store, tc.imsi, tc.assignment)

			if tc.change != nil {
				tc.change(store)
			}

			if err := h.Reconcile(context.Background(), tc.imsi); err != nil {
				t.Fatalf("Reconcile: %v", err)
			}

			if got := len(scscf.Pushes()); got != tc.wantPushes {
				t.Fatalf("profile pushes = %d, want %d", got, tc.wantPushes)
			}

			if got := scscf.Terminations(); !slices.Equal(got, tc.wantTerminated) {
				t.Fatalf("registration terminations = %v, want %v", got, tc.wantTerminated)
			}

			reg, kept := registration(t, store, tc.imsi)
			if kept == tc.wantCleared {
				t.Fatalf("registration kept = %t, want %t", kept, !tc.wantCleared)
			}

			if tc.emergency && (reg.State != hss.Unregistered || !reg.Emergency || reg.ServerName == "") {
				t.Fatalf("registration = %+v, want the S-CSCF kept for the emergency registration", reg)
			}

			if err := h.Reconcile(context.Background(), tc.imsi); err != nil {
				t.Fatalf("Reconcile again: %v", err)
			}

			if got := len(scscf.Pushes()); got != tc.wantPushes || len(scscf.Terminations()) != len(tc.wantTerminated) {
				t.Fatalf("a second reconciliation sent more: %d pushes, %v terminations", got, scscf.Terminations())
			}
		})
	}
}

func TestReconcileSkipsUsersNotAssigned(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{}
	h := connectedHSS(t, store, scscf)

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile without a registration: %v", err)
	}

	challenge(t, store, testSCSCF)
	store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile while authentication is pending: %v", err)
	}

	if len(scscf.Pushes()) != 0 || len(scscf.Terminations()) != 0 {
		t.Fatalf("sent %d pushes and %v terminations to a user not registered", len(scscf.Pushes()), scscf.Terminations())
	}
}

func TestPushProfileRequest_TS29229_6_1_11(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{}
	h := connectedHSS(t, store, scscf)

	assigned(t, store, testIMSI, cx.AssignmentRegistration)
	store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	pushes := scscf.Pushes()
	if len(pushes) != 1 {
		t.Fatalf("profile pushes = %d, want 1", len(pushes))
	}

	req := pushes[0]

	ppr, err := cx.ParsePushProfileRequest(req)
	if err != nil {
		t.Fatalf("parse PPR: %v", err)
	}

	env := tgpp.ParseEnvelope(req)
	if ppr.PrivateIdentity != impiOf(testIMSI) || env.DestinationHost != cscfIdentity.OriginHost || env.DestinationRealm != cscfIdentity.OriginRealm {
		t.Fatalf("PPR User-Name %q to %q in %q, want the S-CSCF's own and the identity it knows", ppr.PrivateIdentity, env.DestinationHost, env.DestinationRealm)
	}

	if _, ok := req.Find(tgpp.AVPSupportedFeatures, tgpp.VendorID); ok || ppr.Charging != nil || ppr.AllowedWebRTC != nil {
		t.Fatalf("PPR = %+v, want only the user data", ppr)
	}

	saa := assigned(t, store, testIMSI, cx.AssignmentNoAssignment)
	if !bytes.Equal(ppr.UserData, saa) {
		t.Fatalf("PPR user data differs from the SAA's:\n%s\n%s", ppr.UserData, saa)
	}

	sub, err := cx.ParseUserData(ppr.UserData)
	if err != nil || len(sub.ServiceProfiles) != 1 || len(sub.ServiceProfiles[0].InitialFilterCriteria) != 1 {
		t.Fatalf("PPR user data = %+v, %v; want the whole profile with its iFC", sub, err)
	}
}

func TestPushProfileAnswer_TS29228_6_2_2_1(t *testing.T) {
	for _, tc := range []struct {
		name             string
		result           tgpp.Result
		rtrResult        tgpp.Result
		wantErr          bool
		wantTerminations []cx.ReasonCode
		wantCleared      bool
		wantDelivered    bool
	}{
		{name: "accepted", result: tgpp.Result{Code: diameter.ResultSuccess}, wantDelivered: true},
		{name: "profile push unsupported", result: tgpp.Result{Code: diameter.ResultCommandUnsupported}},
		{name: "unsupported user data", result: tgpp.Experimental(tgpp.ResultErrorNotSupportedUserData), wantTerminations: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true},
		{name: "too much data", result: tgpp.Experimental(tgpp.ResultErrorTooMuchData), wantTerminations: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true},
		{name: "unsupported user data, de-registration refused", result: tgpp.Experimental(tgpp.ResultErrorNotSupportedUserData), rtrResult: tgpp.Result{Code: diameter.ResultUnableToComply}, wantErr: true, wantTerminations: []cx.ReasonCode{cx.ReasonServerChange}, wantCleared: true},
		{name: "user unknown", result: tgpp.Experimental(tgpp.ResultErrorUserUnknown), wantCleared: true},
		{name: "unable to comply", result: tgpp.Result{Code: diameter.ResultUnableToComply}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			scscf := &fakeSCSCF{pushResult: tc.result, rtrResult: tc.rtrResult}
			h := connectedHSS(t, store, scscf)

			assigned(t, store, testIMSI, cx.AssignmentRegistration)
			before, _ := registration(t, store, testIMSI)
			store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

			err := h.Reconcile(context.Background(), testIMSI)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Reconcile: %v, want error %t", err, tc.wantErr)
			}

			if got := scscf.Terminations(); !slices.Equal(got, tc.wantTerminations) {
				t.Fatalf("registration terminations = %v, want %v", got, tc.wantTerminations)
			}

			after, kept := registration(t, store, testIMSI)
			if kept == tc.wantCleared {
				t.Fatalf("registration kept = %t, want %t", kept, !tc.wantCleared)
			}

			if kept && (after.ProfileDigest != before.ProfileDigest) != tc.wantDelivered {
				t.Fatalf("delivered digest changed = %t, want %t", after.ProfileDigest != before.ProfileDigest, tc.wantDelivered)
			}
		})
	}
}

func TestUserUnknownSparesANewerRegistration_TS29228_6_2_2_1(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{pushResult: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}
	h := connectedHSS(t, store, scscf)

	assigned(t, store, testIMSI, cx.AssignmentRegistration)
	store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	var fresh hss.Registration

	scscf.onPush = func() {
		assigned(t, store, testIMSI, cx.AssignmentRegistration)
		fresh, _ = registration(t, store, testIMSI)
	}

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if got, kept := registration(t, store, testIMSI); !kept || got != fresh {
		t.Fatalf("registration = %+v (kept %t), want the registration made while the push was answered", got, kept)
	}
}

func TestReconcileFailsFastWithoutTheSCSCF(t *testing.T) {
	store := newFakeIMSStore()
	node := newTestDiameterNode(t, hssIdentity, netip.MustParseAddr("127.0.0.1"), diameter.HandlerFunc(func(_ context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return c.Answer(req, diameter.ResultCommandUnsupported)
	}))
	h := hss.New(store, udm.NewIMSCredentials(store), nodePeers{node: node}, nil, zap.NewNop())

	assigned(t, store, testIMSI, cx.AssignmentRegistration)
	store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	start := time.Now()

	if err := h.Reconcile(context.Background(), testIMSI); !errors.Is(err, diameter.ErrNotConnected) && !errors.Is(err, diameter.ErrUnableToDeliver) {
		t.Fatalf("Reconcile: %v, want no route to the S-CSCF", err)
	}

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Reconcile took %s, want it to fail fast", elapsed)
	}
}
