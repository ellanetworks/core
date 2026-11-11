// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehplmn"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/sepp/sepptest"
	"github.com/ellanetworks/core/internal/udm"
	"go.uber.org/zap"
)

const (
	amfTestK    = "465b5ce8b199b49faa5f0a2ee238a6bc"
	amfTestOPc  = "cd63cb71954a9f4e48a5994e37a02baf"
	amfTestIMSI = "001010000000001"
	amfTestSUCI = "suci-0-001-01-0000-0-0-0000000001"
)

var (
	amfVisited  = wire.PlmnID{Mcc: "999", Mnc: "70"}
	amfHPLMNNet = wire.PlmnID{Mcc: "001", Mnc: "01"}
)

type n32Fixture struct {
	home    *fakehplmn.HPLMN
	partner *sepptest.Partner
	ella    *sepptest.Ella
	amf     *roaming.AMFHPLMN
	target  roaming.HPLMNTarget
}

func newN32Fixture(t *testing.T) *n32Fixture {
	t.Helper()

	f := &n32Fixture{home: fakehplmn.New(amfHPLMNNet)}
	f.home.Provision(amfTestIMSI, fakehplmn.Subscriber{
		K:   amfTestK,
		OPc: amfTestOPc,
		SQN: "000000000020",
		AM: wire.AccessAndMobilitySubscriptionData{
			SubscribedUeAmbr: &wire.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
			Nssai:            &wire.Nssai{DefaultSingleNssais: []wire.Snssai{wire.NewSnssai(1, "")}},
		},
		SMFSel: &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
			"1": {DnnInfos: []wire.DnnInfo{{Dnn: "internet", DefaultDnnIndicator: true, LboRoamingAllowed: true}}},
		}},
	})

	f.partner = sepptest.NewPartner(t, amfHPLMNNet, f.home)
	f.ella = sepptest.NewElla(t, amfVisited, 1, f.partner.Config("p"))
	f.amf = roaming.NewAMFHPLMN(roaming.NewHPLMNSBI(f.ella.Manager, roaming.NewDiscovery(f.ella.Manager)))
	f.target = roaming.HPLMNTarget{PartnerID: "p", PLMN: amfHPLMNNet, Serving: amfVisited, RoutingIndicator: "0000"}

	return f
}

func resStar(t *testing.T, c roaming.Challenge, servingNetwork string) [16]byte {
	t.Helper()

	k, _ := hex.DecodeString(amfTestK)
	opc, _ := hex.DecodeString(amfTestOPc)
	res, ck, ik, ak, akstar := make([]byte, 8), make([]byte, 16), make([]byte, 16), make([]byte, 6), make([]byte, 6)

	if err := udm.F2345(opc, k, c.RAND[:], res, ck, ik, ak, akstar); err != nil {
		t.Fatal(err)
	}

	out, err := udm.DeriveXresStar(ck, ik, servingNetwork, c.RAND[:], res)
	if err != nil {
		t.Fatal(err)
	}

	return [16]byte(out)
}

func TestAMFHPLMNAuthenticatesThroughTheSEPP(t *testing.T) {
	f := newN32Fixture(t)
	ctx := context.Background()

	c, err := f.amf.Authenticate(ctx, f.target, amfTestSUCI, nil)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if !strings.HasSuffix(c.ConfirmURI, "/5g-aka-confirmation") {
		t.Fatalf("confirmation URI %q", c.ConfirmURI)
	}

	post := f.home.Requests()[1]

	var info wire.AuthenticationInfo
	if err := json.Unmarshal(post.Body, &info); err != nil {
		t.Fatal(err)
	}

	if info.ServingNetworkName != "5G:mnc070.mcc999.3gppnetwork.org" || info.RoutingIndicator != "0000" || info.SupiOrSuci != amfTestSUCI {
		t.Fatalf("authentication info %+v", info)
	}

	supi, kseaf, err := f.amf.Confirm(ctx, f.target, c.ConfirmURI, amfTestSUCI, resStar(t, c, info.ServingNetworkName))
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	if supi != "imsi-"+amfTestIMSI || kseaf == [32]byte{} {
		t.Fatalf("confirmation returned %q, %x", supi, kseaf)
	}

	if err := f.amf.RemoveAuthentication(ctx, f.target, c.ConfirmURI); err != nil {
		t.Fatalf("RemoveAuthentication: %v", err)
	}
}

func TestAMFHPLMNReportsAFailedConfirmationAndANullRESStar(t *testing.T) {
	f := newN32Fixture(t)
	ctx := context.Background()

	c, err := f.amf.Authenticate(ctx, f.target, amfTestSUCI, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := f.amf.Confirm(ctx, f.target, c.ConfirmURI, amfTestSUCI, [16]byte{1}); !errors.Is(err, wire.ErrAuthenticationFailed) {
		t.Fatalf("Confirm with a wrong RES* = %v, want ErrAuthenticationFailed", err)
	}

	c, err = f.amf.Authenticate(ctx, f.target, amfTestSUCI, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.amf.Unconfirmed(ctx, f.target, c.ConfirmURI); err != nil {
		t.Fatalf("Unconfirmed: %v", err)
	}

	confirmations := f.home.Confirmations()
	if len(confirmations) != 2 || confirmations[1] != nil {
		t.Fatalf("confirmations %v, want a null RES* last", confirmations)
	}
}

func TestAMFHPLMNRegistersSubscribesAndPurges(t *testing.T) {
	f := newN32Fixture(t)
	ctx := context.Background()
	supi := "imsi-" + amfTestIMSI
	guami := wire.Guami{PlmnID: &wire.PlmnIDNid{Mcc: "999", Mnc: "70"}, AmfID: "cafe01"}

	callback, err := sepp.Callback{NF: sepp.CallbackAMF, SEPPID: 1, Supi: supi, Kind: sepp.CallbackUECMDereg}.URI(amfVisited)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.amf.Register(ctx, f.target, supi, wire.Amf3GppAccessRegistration{
		AmfInstanceID: roaming.NFInstanceID("amf1.5gc.mnc070.mcc999.3gppnetwork.org"), DeregCallbackURI: callback, Guami: &guami, RatType: wire.RatNR,
	}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	reg, ok := f.home.Registration(supi)
	if !ok || reg.DeregCallbackURI != "https://amf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+supi+"/uecm-dereg" {
		t.Fatalf("home registration %+v", reg)
	}

	data, err := f.amf.SubscriptionData(ctx, f.target, supi)
	if err != nil {
		t.Fatalf("SubscriptionData: %v", err)
	}

	if data.AmData == nil || data.AmData.SubscribedUeAmbr == nil || data.SmfSelData == nil {
		t.Fatalf("subscription data %+v", data)
	}

	sub, err := f.amf.Subscribe(ctx, f.target, supi, wire.SdmSubscription{NfInstanceID: reg.AmfInstanceID, CallbackReference: callback, MonitoredResourceUris: []string{wire.SDMAmDataPath(supi)}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if sub.ID == "" || !strings.HasSuffix(sub.URI, "/sdm-subscriptions/"+sub.ID) {
		t.Fatalf("subscription %+v", sub)
	}

	if err := f.amf.Unsubscribe(ctx, f.target, sub.URI); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	if err := f.amf.Deregister(ctx, f.target, supi, guami); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	if !f.home.Purged(supi) {
		t.Fatal("the home UDM was not told to purge")
	}

	other := guami
	other.AmfID = "cafe02"

	if pd, ok := roaming.ProblemOf(f.amf.Deregister(ctx, f.target, supi, other)); !ok || pd.Status != http.StatusForbidden {
		t.Fatalf("purge with another GUAMI: %+v", pd)
	}
}

func TestAMFHPLMNReturnsTheHPLMNProblem(t *testing.T) {
	f := newN32Fixture(t)
	f.home.Fail(http.MethodPut, "amf-3gpp-access", wire.NewProblem(http.StatusForbidden, wire.CauseRoamingNotAllowed, ""))

	_, err := f.amf.Register(context.Background(), f.target, "imsi-"+amfTestIMSI, wire.Amf3GppAccessRegistration{
		AmfInstanceID: roaming.NFInstanceID("amf1"), DeregCallbackURI: "https://amf1.5gc.mnc070.mcc999.3gppnetwork.org/x", Guami: &wire.Guami{PlmnID: &wire.PlmnIDNid{Mcc: "999", Mnc: "70"}, AmfID: "cafe01"}, RatType: wire.RatNR,
	})

	pd, ok := roaming.ProblemOf(err)
	if !ok || pd.Status != http.StatusForbidden || pd.Cause != wire.CauseRoamingNotAllowed {
		t.Fatalf("error %v, want 403 ROAMING_NOT_ALLOWED", err)
	}
}

func TestAMFHPLMNWithoutAnN32ContextIsUnreachable(t *testing.T) {
	f := newN32Fixture(t)

	target := f.target
	target.PartnerID = "unknown"

	if _, err := f.amf.Authenticate(context.Background(), target, amfTestSUCI, nil); !errors.Is(err, roaming.ErrHPLMNUnreachable) {
		t.Fatalf("error %v, want ErrHPLMNUnreachable", err)
	}
}

type recordedEvents struct {
	mu      sync.Mutex
	known   string
	deregs  []wire.DeregistrationData
	changes []wire.ModificationNotification
}

func (e *recordedEvents) HPLMNDeregistration(_ context.Context, partnerID, imsi string, data wire.DeregistrationData) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if partnerID != "p" || imsi != e.known {
		return roaming.ErrNoVisitorContext
	}

	e.deregs = append(e.deregs, data)

	return nil
}

func (e *recordedEvents) HPLMNSubscriptionChanged(_ context.Context, partnerID, imsi string, n wire.ModificationNotification) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if partnerID != "p" || imsi != e.known {
		return roaming.ErrNoVisitorContext
	}

	e.changes = append(e.changes, n)

	return nil
}

func TestAMFCallbacksArriveThroughTheSEPP(t *testing.T) {
	f := newN32Fixture(t)
	events := &recordedEvents{known: amfTestIMSI}
	f.ella.Manager.SetCallbacks(roaming.NewAMFCallbacks(events, zap.NewNop()))

	ctx := context.Background()
	supi := "imsi-" + amfTestIMSI

	uri := func(id int, s string, kind sepp.CallbackKind) string {
		u, err := sepp.Callback{NF: sepp.CallbackAMF, SEPPID: id, Supi: s, Kind: kind}.URI(amfVisited)
		if err != nil {
			t.Fatal(err)
		}

		return u
	}

	cases := []struct {
		name   string
		target string
		body   any
		status int
		cause  wire.Cause
	}{
		{"deregistration", uri(1, supi, sepp.CallbackUECMDereg), wire.DeregistrationData{DeregReason: wire.DeregSubscriptionWithdrawn, AccessType: wire.Access3GPP}, http.StatusNoContent, ""},
		{"data change", uri(1, supi, sepp.CallbackSDMChange), wire.ModificationNotification{NotifyItems: []wire.NotifyItem{{ResourceID: supi + "/am-data", Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: new(string)}}}}}, http.StatusNoContent, ""},
		{"unknown visitor", uri(1, "imsi-001010000000002", sepp.CallbackUECMDereg), wire.DeregistrationData{DeregReason: wire.DeregSubscriptionWithdrawn}, http.StatusNotFound, wire.CauseContextNotFound},
		{"another node", uri(2, supi, sepp.CallbackSDMChange), wire.ModificationNotification{NotifyItems: []wire.NotifyItem{{ResourceID: "x", Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: new(string)}}}}}, http.StatusNotFound, wire.CauseSubscriptionNotFound},
		{"malformed", uri(1, supi, sepp.CallbackUECMDereg), []string{"not", "an", "object"}, http.StatusBadRequest, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := f.partner.Notify(ctx, f.ella.URL(), tc.target, tc.body)
			if err != nil {
				t.Fatal(err)
			}

			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tc.status {
				t.Fatalf("status %d, want %d", resp.StatusCode, tc.status)
			}

			if tc.cause != "" {
				var pd wire.ProblemDetails

				_ = json.NewDecoder(resp.Body).Decode(&pd)

				if pd.Cause != tc.cause {
					t.Fatalf("cause %q, want %q", pd.Cause, tc.cause)
				}
			}
		})
	}

	events.mu.Lock()
	defer events.mu.Unlock()

	if len(events.deregs) != 1 || events.deregs[0].DeregReason != wire.DeregSubscriptionWithdrawn || len(events.changes) != 1 {
		t.Fatalf("delivered %+v and %+v", events.deregs, events.changes)
	}
}
