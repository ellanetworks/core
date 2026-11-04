// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/fakehplmn"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/sepp/sepptest"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const (
	vsmfIMSI        = "001010000000001"
	vsmfSupi        = "imsi-" + vsmfIMSI
	vsmfUDMRoot     = "http://udm.5gc.mnc001.mcc001.3gppnetwork.org"
	vsmfSMDataBody  = `[{"singleNssai": {"sst": 1, "sd": "010203"}, "dnnConfigurations": {"internet": {"pduSessionTypes": {"defaultSessionType": "IPV4", "allowedSessionTypes": ["IPV4V6"]}, "sscModes": {"defaultSscMode": "SSC_MODE_1"}, "5gQosProfile": {"5qi": 7, "arp": {"priorityLevel": 8, "preemptCap": "NOT_PREEMPT", "preemptVuln": "PREEMPTABLE"}}, "sessionAmbr": {"uplink": "50 Mbps", "downlink": "150 Mbps"}}}}]`
	vsmfSMDataLower = `[{"singleNssai": {"sst": 1, "sd": "010203"}, "dnnConfigurations": {"internet": {"pduSessionTypes": {"defaultSessionType": "IPV4"}, "sscModes": {"defaultSscMode": "SSC_MODE_1"}, "sessionAmbr": {"uplink": "10 Mbps", "downlink": "20 Mbps"}}}}]`
	vsmfSMDataOther = `[{"singleNssai": {"sst": 1, "sd": "010203"}, "dnnConfigurations": {"corporate": {"pduSessionTypes": {"defaultSessionType": "IPV4"}, "sscModes": {"defaultSscMode": "SSC_MODE_1"}}}}]`
)

var (
	vsmfSnssai = models.Snssai{Sst: 1, Sd: "010203"}
	vsmfHPLMN  = wire.PlmnID{Mcc: "001", Mnc: "01"}
	vsmfOP     = wire.PlmnID{Mcc: "999", Mnc: "70"}
)

type vsmfN32 struct {
	*fakeN32
	down bool
}

func (n *vsmfN32) SEPPID() int { return 1 }

func (n *vsmfN32) WaitEstablished(context.Context, string, time.Duration) error { return nil }

func (n *vsmfN32) Client(partnerID string) *http.Client {
	if n.down {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, sepp.ErrNoContext })}
	}

	return n.fakeN32.Client(partnerID)
}

type vsmfReleased struct {
	psi uint8
	h   smf.HPLMNSubscription
	how smf.HPLMNRelease
}

type vsmfControl struct {
	releases   chan vsmfReleased
	reconciles chan etsi.SUPI
	found      bool
}

func (f *vsmfControl) ReleaseHPLMNBoundSession(_ context.Context, _ etsi.SUPI, psi uint8, h smf.HPLMNSubscription, how smf.HPLMNRelease) bool {
	f.releases <- vsmfReleased{psi: psi, h: h, how: how}
	return f.found
}

func (f *vsmfControl) ReconcileHPLMNBoundSessions(_ context.Context, supi etsi.SUPI) {
	f.reconciles <- supi
}

type vsmfFixture struct {
	v       *VSMF
	home    *fakehplmn.HPLMN
	n32     *vsmfN32
	control *vsmfControl
	target  smf.HPLMNTarget
}

func vsmfTarget() smf.HPLMNTarget {
	return smf.HPLMNTarget{PartnerID: "p", HPLMN: vsmfHPLMN.Model(), ServingNetwork: vsmfOP.Model(), RoutingIndicator: "0000", HPLMNSnssai: vsmfSnssai, LBO: true}
}

func (v *VSMF) holdsUE(imsi string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	_, ok := v.ues[imsi]

	return ok
}

func (s *vsmfSession) subscribed() bool {
	s.v.mu.Lock()
	defer s.v.mu.Unlock()

	return s.subscription != ""
}

func newVSMFHome() *fakehplmn.HPLMN {
	home := fakehplmn.New(vsmfHPLMN)
	home.Provision(vsmfIMSI, fakehplmn.Subscriber{SMData: json.RawMessage(vsmfSMDataBody)})

	return home
}

func newVSMFControl() *vsmfControl {
	return &vsmfControl{releases: make(chan vsmfReleased, 4), reconciles: make(chan etsi.SUPI, 4), found: true}
}

func newVSMFFixture(t *testing.T) *vsmfFixture {
	t.Helper()

	home := newVSMFHome()
	n32 := &vsmfN32{fakeN32: &fakeN32{handler: home}}
	control := newVSMFControl()
	v := NewVSMF(zap.NewNop())
	v.SetN32(n32)
	v.SetSessions(control)

	return &vsmfFixture{v: v, home: home, n32: n32, control: control, target: vsmfTarget()}
}

func vsmfSUPI(t *testing.T) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(vsmfIMSI)
	if err != nil {
		t.Fatal(err)
	}

	return supi
}

func (f *vsmfFixture) open(t *testing.T, psi uint8, dnn string) (smf.HPLMNSubscription, error) {
	t.Helper()

	return f.v.OpenSubscription(context.Background(), smf.HPLMNSubscriptionRequest{Supi: vsmfSUPI(t), PDUSessionID: psi, Dnn: dnn, Target: f.target})
}

func (f *vsmfFixture) mustOpen(t *testing.T, psi uint8) *vsmfSession {
	t.Helper()

	h, err := f.open(t, psi, "internet")
	if err != nil || h == nil {
		t.Fatalf("OpenSubscription = %v, %v", h, err)
	}

	return h.(*vsmfSession)
}

func (f *vsmfFixture) mustRegister(t *testing.T, psi uint8) *vsmfSession {
	t.Helper()

	h := f.mustOpen(t, psi)
	if err := h.Register(context.Background()); err != nil {
		t.Fatalf("Register: %v", err)
	}

	return h
}

func (f *vsmfFixture) setSMData(body string) {
	f.home.Update(vsmfIMSI, func(s *fakehplmn.Subscriber) { s.SMData = json.RawMessage(body) })
}

func vsmfRequests(home *fakehplmn.HPLMN, method, contains string) []fakehplmn.Request {
	var out []fakehplmn.Request

	for _, r := range home.Requests() {
		if r.Method == method && strings.Contains(r.Path, contains) {
			out = append(out, r)
		}
	}

	return out
}

func vsmfEventually(t *testing.T, what string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}

		time.Sleep(5 * time.Millisecond)
	}
}

func vsmfAwait(t *testing.T, home *fakehplmn.HPLMN, method, contains string, n int) []fakehplmn.Request {
	t.Helper()

	var got []fakehplmn.Request

	vsmfEventually(t, method+" "+contains, func() bool {
		got = vsmfRequests(home, method, contains)
		return len(got) >= n
	})

	return got
}

func vsmfJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode %q: %v", b, err)
	}

	return out
}

func TestVSMFOpenGetsSubscribesAndMergesTheHomeSubscription(t *testing.T) {
	f := newVSMFFixture(t)

	h := f.mustOpen(t, 5)

	rules := h.Rules()
	if rules.HPLMNSnssai == nil || *rules.HPLMNSnssai != vsmfSnssai || len(rules.PDUSessionTypes) != 2 || rules.PDUSessionTypes[0] != fgs.PDUSessionTypeIPv4 || rules.PDUSessionTypes[1] != fgs.PDUSessionTypeIPv4v6 {
		t.Fatalf("rules = %+v", rules)
	}

	gets := vsmfRequests(f.home, http.MethodGet, "/nudm-sdm/v2/"+vsmfSupi+"/sm-data")
	if len(gets) != 1 {
		t.Fatalf("sm-data GETs = %d", len(gets))
	}

	q, _ := url.ParseQuery(gets[0].Query)
	if q.Get("single-nssai") != `{"sst":1,"sd":"010203"}` || q.Get("dnn") != "internet" || q.Get("plmn-id") != `{"mcc":"999","mnc":"70"}` {
		t.Fatalf("sm-data query = %v", q)
	}

	if rules.SessionAMBR == nil || rules.SessionAMBR.Uplink.Bps() != 50_000_000 || rules.SessionAMBR.Downlink.Bps() != 150_000_000 {
		t.Fatalf("Session-AMBR = %+v, want the home 50/150 Mbps", rules.SessionAMBR)
	}

	if gets[0].Header.Get("User-Agent") != "SMF" {
		t.Fatalf("User-Agent = %q", gets[0].Header.Get("User-Agent"))
	}

	nrf := vsmfRequests(f.home, http.MethodGet, "/nnrf-disc/v1/nf-instances")
	if len(nrf) == 0 {
		t.Fatal("no discovery")
	}

	if dq, _ := url.ParseQuery(nrf[0].Query); dq.Get("requester-nf-type") != "SMF" || dq.Get("target-nf-type") != "UDM" || dq.Get("service-names") != "nudm-sdm" {
		t.Fatalf("discovery = %v", dq)
	}

	f.v.RenewSubscriptions(context.Background())

	if n := len(vsmfRequests(f.home, http.MethodPost, "/sdm-subscriptions")); n != 0 {
		t.Fatalf("SDM subscriptions before the UECM registration = %d, want none", n)
	}

	if err := h.Register(context.Background()); err != nil {
		t.Fatal(err)
	}

	subs := vsmfAwait(t, f.home, http.MethodPost, "/nudm-sdm/v2/"+vsmfSupi+"/sdm-subscriptions", 1)

	sub := vsmfJSON(t, subs[0].Body)
	if sub["callbackReference"] != "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+vsmfSupi+"/sdm-change" ||
		sub["dnn"] != "internet" || sub["implicitUnsubscribe"] != true ||
		sub["monitoredResourceUris"].([]any)[0] != "/nudm-sdm/v2/"+vsmfSupi+"/sm-data" ||
		sub["nfInstanceId"] != f.v.instanceID(vsmfOP) {
		t.Fatalf("SDM subscription = %s", subs[0].Body)
	}

	if _, ok := sub["expires"]; ok {
		t.Fatalf("SDM subscription = %s, want no proposed expiry", subs[0].Body)
	}

	f.mustRegister(t, 6)

	if n := len(vsmfRequests(f.home, http.MethodGet, "/sm-data")); n != 2 {
		t.Fatalf("sm-data GETs after a second session on the same DNN = %d, want one per session", n)
	}

	vsmfAwait(t, f.home, http.MethodPost, "/sdm-subscriptions", 2)
}

func TestVSMFRegisterThenCloseDeregistersAndUnsubscribes(t *testing.T) {
	f := newVSMFFixture(t)

	first := f.mustOpen(t, 5)
	second := f.mustOpen(t, 6)

	if err := first.Register(context.Background()); err != nil {
		t.Fatal(err)
	}

	if err := first.Register(context.Background()); err != nil {
		t.Fatal(err)
	}

	puts := vsmfRequests(f.home, http.MethodPut, "/nudm-uecm/v1/"+vsmfSupi+"/registrations/smf-registrations/5")
	if len(puts) != 1 {
		t.Fatalf("UECM registrations = %d, want 1", len(puts))
	}

	reg := vsmfJSON(t, puts[0].Body)
	if reg["smfInstanceId"] != f.v.instanceID(vsmfOP) || reg["pduSessionId"] != float64(5) || reg["dnn"] != "internet" ||
		reg["deregCallbackUri"] != "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+vsmfSupi+"/uecm-dereg" {
		t.Fatalf("SMF registration = %s", puts[0].Body)
	}

	if p := reg["plmnId"].(map[string]any); p["mcc"] != "999" || p["mnc"] != "70" {
		t.Fatalf("plmnId = %v, want the serving PLMN", p)
	}

	if s := reg["singleNssai"].(map[string]any); s["sst"] != float64(1) || s["sd"] != "010203" {
		t.Fatalf("singleNssai = %v", s)
	}

	vsmfEventually(t, "the registered session is subscribed", first.subscribed)

	if second.subscribed() {
		t.Fatal("subscribed for a session that never registered")
	}

	first.Close()
	first.Close()

	dels := vsmfAwait(t, f.home, http.MethodDelete, "/smf-registrations/5", 1)
	if q, _ := url.ParseQuery(dels[0].Query); q.Get("smf-instance-id") != f.v.instanceID(vsmfOP) {
		t.Fatalf("smf-instance-id = %q, want this SMF's instance", q.Get("smf-instance-id"))
	}

	vsmfAwait(t, f.home, http.MethodDelete, "/sdm-subscriptions/", 1)

	if _, ok := f.home.SMFRegistration(vsmfSupi, "5"); ok {
		t.Fatal("the home UDM kept the closed session's registration")
	}

	if n := len(f.home.Subscriptions()); n != 0 {
		t.Fatalf("SDM subscriptions after closing the registered session = %d, want none", n)
	}

	second.Close()

	vsmfEventually(t, "the visitor is forgotten", func() bool { return !f.v.holdsUE(vsmfIMSI) })

	if n := len(vsmfRequests(f.home, http.MethodDelete, "/smf-registrations/6")); n != 0 {
		t.Fatal("deregistered a session that never registered")
	}
}

func TestVSMFRefusals(t *testing.T) {
	fail := func(status int, cause wire.Cause) func(f *vsmfFixture) {
		return func(f *vsmfFixture) { f.home.Fail(http.MethodGet, "/sm-data", wire.NewProblem(status, cause, "")) }
	}

	data := func(body string) func(f *vsmfFixture) {
		return func(f *vsmfFixture) { f.setSMData(body) }
	}

	for name, tc := range map[string]struct {
		dnn     string
		setup   func(f *vsmfFixture)
		want    fgs.GSMCause
		wantErr error
	}{
		"packet services barred":   {dnn: "internet", setup: data(strings.Replace(vsmfSMDataBody, `"dnnConfigurations"`, `"odbPacketServices": "ALL_PACKET_SERVICES", "dnnConfigurations"`, 1)), want: fgs.GSMCauseOperatorDeterminedBarring, wantErr: ErrSessionBarred},
		"VPLMN APs barred":         {dnn: "internet", setup: data(strings.Replace(vsmfSMDataBody, `"dnnConfigurations"`, `"odbPacketServices": "ROAMER_ACCESS_VPLMN_AP", "dnnConfigurations"`, 1)), want: fgs.GSMCauseOperatorDeterminedBarring, wantErr: ErrSessionBarred},
		"user unknown":             {dnn: "internet", setup: fail(http.StatusNotFound, wire.CauseUserNotFound), want: fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, wantErr: ErrSessionHPLMNRefused},
		"data not found":           {dnn: "internet", setup: fail(http.StatusNotFound, wire.CauseDataNotFound), want: fgs.GSMCauseRequestedServiceOptionNotSubscribed, wantErr: ErrSessionNotSubscribed},
		"DNN not in sm-data":       {dnn: "internet", setup: data(vsmfSMDataOther), want: fgs.GSMCauseRequestedServiceOptionNotSubscribed, wantErr: ErrSessionNotSubscribed},
		"home UDM overloaded":      {dnn: "internet", setup: fail(http.StatusServiceUnavailable, wire.CauseNFCongestion), want: fgs.GSMCauseNetworkFailure, wantErr: ErrSessionHPLMNUnreachable},
		"N32 down":                 {dnn: "internet", setup: func(f *vsmfFixture) { f.n32.down = true }, want: fgs.GSMCauseNetworkFailure, wantErr: ErrSessionHPLMNUnreachable},
		"SSC mode 1 not allowed":   {dnn: "internet", setup: data(strings.Replace(vsmfSMDataBody, "SSC_MODE_1", "SSC_MODE_2", 1)), want: fgs.GSMCauseNotSupportedSSCMode, wantErr: ErrSessionSSCMode},
		"no IP PDU session type":   {dnn: "internet", setup: data(strings.NewReplacer(`"IPV4"`, `"ETHERNET"`, `["IPV4V6"]`, `["UNSTRUCTURED"]`).Replace(vsmfSMDataBody)), want: fgs.GSMCauseUnknownPDUSessionType, wantErr: ErrSessionNotIP},
		"shared data not resolved": {dnn: "internet", setup: data(`{"sharedSmSubsDataIds": ["001-01-1"]}`), want: fgs.GSMCauseRequestRejectedUnspecified, wantErr: ErrSessionHPLMNData},
	} {
		t.Run(name, func(t *testing.T) {
			f := newVSMFFixture(t)
			tc.setup(f)

			h, err := f.open(t, 5, tc.dnn)

			var se *SessionError
			if h != nil || !errors.As(err, &se) || se.GSMCause() != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("OpenSubscription = %v, %v; want cause %d and %v", h, err, tc.want, tc.wantErr)
			}

			if n := len(vsmfRequests(f.home, http.MethodPost, "/sdm-subscriptions")); n != 0 {
				t.Fatal("subscribed for a refused session")
			}

			if f.v.holdsUE(vsmfIMSI) {
				t.Fatal("kept home data for a refused session")
			}
		})
	}
}

func TestVSMFRegistrationErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		pd    wire.ProblemDetails
		cause fgs.GSMCause
	}{
		"roaming not allowed": {wire.NewProblem(http.StatusForbidden, wire.CauseRoamingNotAllowed, ""), fgs.GSMCauseUserAuthenticationOrAuthorizationFailed},
		"DNN not allowed":     {wire.NewProblem(http.StatusForbidden, wire.CauseDNNNotAllowed, ""), fgs.GSMCauseMissingOrUnknownDNN},
		"user not found":      {wire.NewProblem(http.StatusNotFound, wire.CauseUserNotFound, ""), fgs.GSMCauseUserAuthenticationOrAuthorizationFailed},
		"home UDM failing":    {wire.NewProblem(http.StatusInternalServerError, "", ""), 0},
	} {
		t.Run(name, func(t *testing.T) {
			f := newVSMFFixture(t)
			f.home.Fail(http.MethodPut, "/smf-registrations/5", tc.pd)

			err := f.mustOpen(t, 5).Register(context.Background())

			var se *SessionError

			switch {
			case err == nil:
				t.Fatal("Register succeeded")
			case tc.cause != 0 && (!errors.As(err, &se) || se.GSMCause() != tc.cause):
				t.Fatalf("Register = %v, want cause %d", err, tc.cause)
			case tc.cause == 0 && errors.As(err, &se):
				t.Fatalf("Register = %v, want no 5GSM cause for a transient failure", err)
			}

			if n := len(vsmfRequests(f.home, http.MethodPut, "/smf-registrations/5")); n != 1 {
				t.Fatalf("UECM PUTs = %d, want one attempt", n)
			}
		})
	}
}

func vsmfCallback(t *testing.T, v *VSMF, partnerID string, kind sepp.CallbackKind, body string) *httptest.ResponseRecorder {
	t.Helper()

	peer := sepp.Peer{PartnerID: partnerID, PLMNs: []wire.PlmnID{vsmfHPLMN}, Callback: sepp.Callback{NF: sepp.CallbackSMF, SEPPID: 1, Supi: vsmfSupi, Kind: kind}}
	v.peer = func(context.Context) (sepp.Peer, bool) { return peer, true }

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org"+peer.Callback.Path(), bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	v.ServeHTTP(rec, req)

	return rec
}

func TestVSMFDataChangeRefreshesAndReconciles(t *testing.T) {
	f := newVSMFFixture(t)
	h := f.mustRegister(t, 5)
	vsmfEventually(t, "the session is subscribed", h.subscribed)

	change := `{"notifyItems": [{"resourceId": "` + vsmfUDMRoot + `/nudm-sdm/v2/` + vsmfSupi + `/sm-data", "changes": [{"op": "REPLACE", "path": "/0/dnnConfigurations/internet/sessionAmbr"}]}]}`

	if rec := vsmfCallback(t, f.v, "other", sepp.CallbackSDMChange, change); rec.Code != http.StatusNotFound || wire.ParseProblem(rec.Code, rec.Body.Bytes()).Cause != wire.CauseSubscriptionNotFound {
		t.Fatalf("notification from another partner: %d %s", rec.Code, rec.Body)
	}

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, `{"notifyItems": [], "subscriptionId": "sub-9"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty notification: %d %s", rec.Code, rec.Body)
	}

	f.setSMData(vsmfSMDataLower)

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, change); rec.Code != http.StatusNoContent {
		t.Fatalf("notification: %d %s", rec.Code, rec.Body)
	}

	select {
	case supi := <-f.control.reconciles:
		if supi != vsmfSUPI(t) {
			t.Fatalf("reconciled %v", supi)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the visitor's sessions were not reconciled")
	}

	if r := h.Rules(); r.Withdrawn || r.SessionAMBR == nil || r.SessionAMBR.Uplink.Bps() != 10_000_000 {
		t.Fatalf("rules after the change = %+v", r)
	}

	f.setSMData(vsmfSMDataOther)

	var id string
	for k := range f.home.Subscriptions() {
		id = k
	}

	relative := strings.Replace(change, vsmfUDMRoot+"/nudm-sdm/v2/", "", 1)
	relative = strings.Replace(relative, `"notifyItems"`, `"subscriptionId": "`+id+`", "notifyItems"`, 1)

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, relative); rec.Code != http.StatusNoContent {
		t.Fatalf("notification: %d %s", rec.Code, rec.Body)
	}

	<-f.control.reconciles

	if r := h.Rules(); !r.Withdrawn {
		t.Fatalf("rules after a withdrawal = %+v, want withdrawn", r)
	}

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, strings.Replace(change, `"notifyItems"`, `"subscriptionId": "not-`+id+`", "notifyItems"`, 1)); rec.Code != http.StatusNotFound {
		t.Fatalf("notification for another subscription: %d %s", rec.Code, rec.Body)
	}
}

func TestVSMFDeregistrationNotification(t *testing.T) {
	for name, tc := range map[string]struct {
		reason     wire.DeregistrationReason
		how        smf.HPLMNRelease
		deregister int
	}{
		"duplicate":    {wire.DeregDuplicatePDUSession, smf.HPLMNReleaseDuplicate, 0},
		"transferred":  {wire.DeregSMFContextTransferred, smf.HPLMNReleaseDuplicate, 0},
		"ePDG":         {wire.DeregDuplicatePDUSessionEPDG, smf.HPLMNReleaseDuplicateNotify, 0},
		"reactivation": {wire.DeregPDUSessionReactivationRequired, smf.HPLMNReleaseReactivate, 1},
		"other":        {wire.DeregSubscriptionWithdrawn, smf.HPLMNReleaseDeactivate, 0},
	} {
		t.Run(name, func(t *testing.T) {
			f := newVSMFFixture(t)
			h := f.mustOpen(t, 5)

			if rec := vsmfCallback(t, f.v, "p", sepp.CallbackUECMDereg, `{"deregReason": "`+string(tc.reason)+`", "pduSessionId": 5}`); rec.Code != http.StatusNotFound {
				t.Fatalf("notification before the registration: %d %s", rec.Code, rec.Body)
			}

			if err := h.Register(context.Background()); err != nil {
				t.Fatal(err)
			}

			for _, bad := range []struct {
				partner, body string
				status        int
			}{
				{"p", `{"deregReason": "DUPLICATE_PDU_SESSION"}`, http.StatusBadRequest},
				{"p", `{"deregReason": "DUPLICATE_PDU_SESSION", "pduSessionId": 6}`, http.StatusNotFound},
				{"p", `{"deregReason": "DUPLICATE_PDU_SESSION", "pduSessionId": 300}`, http.StatusNotFound},
				{"other", `{"deregReason": "DUPLICATE_PDU_SESSION", "pduSessionId": 5}`, http.StatusNotFound},
			} {
				if rec := vsmfCallback(t, f.v, bad.partner, sepp.CallbackUECMDereg, bad.body); rec.Code != bad.status {
					t.Fatalf("%s from %s: %d %s", bad.body, bad.partner, rec.Code, rec.Body)
				}
			}

			if rec := vsmfCallback(t, f.v, "p", sepp.CallbackUECMDereg, `{"deregReason": "`+string(tc.reason)+`", "pduSessionId": 5}`); rec.Code != http.StatusNoContent {
				t.Fatalf("notification: %d %s", rec.Code, rec.Body)
			}

			select {
			case r := <-f.control.releases:
				if r.psi != 5 || r.h != h || r.how != tc.how {
					t.Fatalf("release = %+v, want PDU session 5 with %d", r, tc.how)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("the SMF was not asked to release the session")
			}

			vsmfEventually(t, "the session is subscribed", h.subscribed)

			h.Close()

			vsmfAwait(t, f.home, http.MethodDelete, "/sdm-subscriptions/", 1)

			if n := len(vsmfRequests(f.home, http.MethodDelete, "/smf-registrations/5")); n != tc.deregister {
				t.Fatalf("UECM deregistrations = %d, want %d", n, tc.deregister)
			}
		})
	}
}

func TestVSMFDeregistrationOfAGoneSessionClosesIt(t *testing.T) {
	f := newVSMFFixture(t)
	f.control.found = false

	h := f.mustOpen(t, 5)
	if err := h.Register(context.Background()); err != nil {
		t.Fatal(err)
	}

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackUECMDereg, `{"deregReason": "DUPLICATE_PDU_SESSION", "pduSessionId": 5}`); rec.Code != http.StatusNoContent {
		t.Fatalf("notification: %d %s", rec.Code, rec.Body)
	}

	<-f.control.releases

	vsmfEventually(t, "the home session is dropped", func() bool { return !f.v.holdsUE(vsmfIMSI) })

	if n := len(vsmfRequests(f.home, http.MethodDelete, "/smf-registrations/5")); n != 0 {
		t.Fatal("deregistered a registration the home UDM moved to another SMF")
	}
}

func TestVSMFRenewsOnlyAnExpiryTheUDMImposes(t *testing.T) {
	f := newVSMFFixture(t)
	h := f.mustRegister(t, 5)
	vsmfEventually(t, "the session is subscribed", h.subscribed)

	f.v.RenewSubscriptions(context.Background())

	f.home.ImposeSDMExpiry(time.Minute)
	g := f.mustRegister(t, 6)
	vsmfEventually(t, "the second session is subscribed", g.subscribed)

	f.v.RenewSubscriptions(context.Background())

	patches := vsmfAwait(t, f.home, http.MethodPatch, "/sdm-subscriptions/", 1)
	if len(patches) != 1 || patches[0].Header.Get("Content-Type") != wire.ContentTypeMergePatch || !bytes.Contains(patches[0].Body, []byte(`"expires"`)) {
		t.Fatalf("renewals = %+v, want one for the subscription with an imposed expiry", patches)
	}

	f.home.Fail(http.MethodPatch, "/sdm-subscriptions/", wire.NewProblem(http.StatusNotFound, wire.CauseSubscriptionNotFound, ""))
	f.v.now = func() time.Time { return time.Now().Add(24 * time.Hour) }
	f.v.RenewSubscriptions(context.Background())

	vsmfEventually(t, "the failed renewal clears the subscription", func() bool { return !g.subscribed() })

	f.home.Heal()
	f.v.RenewSubscriptions(context.Background())

	vsmfAwait(t, f.home, http.MethodPost, "/sdm-subscriptions", 3)
	vsmfEventually(t, "the session subscribes again", g.subscribed)
}

func TestVSMFSubscribesAgainAtTheNextReconcile(t *testing.T) {
	f := newVSMFFixture(t)
	f.home.Fail(http.MethodPost, "/sdm-subscriptions", wire.NewProblem(http.StatusServiceUnavailable, wire.CauseNFCongestion, ""))

	h := f.mustRegister(t, 5)
	vsmfAwait(t, f.home, http.MethodPost, "/sdm-subscriptions", 1)

	f.home.Heal()
	f.v.RenewSubscriptions(context.Background())

	vsmfEventually(t, "the session subscribes", h.subscribed)

	f.v.RenewSubscriptions(context.Background())
	f.mustRegister(t, 6)
	vsmfAwait(t, f.home, http.MethodPost, "/sdm-subscriptions", 3)

	posts := len(vsmfRequests(f.home, http.MethodPost, "/sdm-subscriptions"))
	if posts != 3 {
		t.Fatalf("SDM subscription attempts = %d, want no retry once subscribed", posts)
	}
}

func TestVSMFRegistersAgainAtTheNextReconcile(t *testing.T) {
	f := newVSMFFixture(t)
	f.home.Fail(http.MethodPut, "/smf-registrations/", wire.NewProblem(http.StatusServiceUnavailable, wire.CauseNFCongestion, ""))

	unopened := f.mustOpen(t, 6)
	h := f.mustOpen(t, 5)

	if err := h.Register(context.Background()); err == nil {
		t.Fatal("Register succeeded while the home UDM is unavailable")
	}

	f.home.Heal()
	f.v.RenewSubscriptions(context.Background())

	vsmfEventually(t, "the session registers and subscribes", h.subscribed)

	if n := len(vsmfRequests(f.home, http.MethodPut, "/smf-registrations/5")); n != 2 {
		t.Fatalf("UECM registrations = %d, want the failed one and its retry", n)
	}

	if n := len(vsmfRequests(f.home, http.MethodPut, "/smf-registrations/6")); n != 0 || unopened.subscribed() {
		t.Fatal("registered a session the SMF never asked to register")
	}
}

func TestVSMFReleasesASessionTheHomeUDMRefusesOnRetry(t *testing.T) {
	f := newVSMFFixture(t)
	f.home.Fail(http.MethodPut, "/smf-registrations/", wire.NewProblem(http.StatusServiceUnavailable, wire.CauseNFCongestion, ""))

	h := f.mustOpen(t, 5)
	if err := h.Register(context.Background()); err == nil {
		t.Fatal("Register succeeded while the home UDM is unavailable")
	}

	f.home.Heal()
	f.home.Fail(http.MethodPut, "/smf-registrations/", wire.NewProblem(http.StatusForbidden, "", ""))
	f.v.RenewSubscriptions(context.Background())

	select {
	case r := <-f.control.releases:
		if r.psi != 5 || r.how != smf.HPLMNReleaseRejected {
			t.Fatalf("release = %+v, want session 5 rejected", r)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the refused session was not released")
	}
}

func TestVSMFCallbacksForAnotherNF(t *testing.T) {
	f := newVSMFFixture(t)
	f.v.peer = func(context.Context) (sepp.Peer, bool) {
		return sepp.Peer{PartnerID: "p", Callback: sepp.Callback{NF: sepp.CallbackAMF, SEPPID: 1, Supi: vsmfSupi, Kind: sepp.CallbackUECMDereg}}, true
	}

	rec := httptest.NewRecorder()
	f.v.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("AMF callback served by the V-SMF: %d", rec.Code)
	}
}

func TestVSMFThroughTheSEPP(t *testing.T) {
	home := newVSMFHome()
	partner := sepptest.NewPartner(t, vsmfHPLMN, home)
	ella := sepptest.NewElla(t, vsmfOP, 1, partner.Config("p"))
	control := newVSMFControl()

	v := NewVSMF(zap.NewNop())
	v.SetN32(ella.Manager)
	v.SetSessions(control)
	ella.Manager.SetCallbacks(v)

	ctx := context.Background()

	opened, err := v.OpenSubscription(ctx, smf.HPLMNSubscriptionRequest{Supi: vsmfSUPI(t), PDUSessionID: 5, Dnn: "internet", Target: vsmfTarget()})
	if err != nil || opened == nil {
		t.Fatalf("OpenSubscription over N32 = %v, %v", opened, err)
	}

	h := opened.(*vsmfSession)

	if err := h.Register(ctx); err != nil {
		t.Fatalf("Register over N32: %v", err)
	}

	vsmfEventually(t, "the session is subscribed over N32", h.subscribed)

	puts := vsmfRequests(home, http.MethodPut, "/smf-registrations/5")
	if len(puts) != 1 || puts[0].Header.Get(sepp.HeaderOriginatingNetworkID) != "999-70" {
		t.Fatalf("UECM registration over N32 = %+v", puts)
	}

	change := wire.ModificationNotification{NotifyItems: []wire.NotifyItem{{ResourceID: vsmfSupi + "/sm-data", Changes: []wire.ChangeItem{{Op: wire.ChangeReplace, Path: new("/sessionAmbr")}}}}}

	resp, err := partner.Notify(ctx, ella.URL(), "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+vsmfSupi+"/sdm-change", change)
	if err != nil {
		t.Fatal(err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("SDM change through the SEPP: %d", resp.StatusCode)
	}

	<-control.reconciles

	psi := 5

	resp, err = partner.Notify(ctx, ella.URL(), "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+vsmfSupi+"/uecm-dereg", wire.DeregistrationData{DeregReason: wire.DeregDuplicatePDUSession, PduSessionID: &psi})
	if err != nil {
		t.Fatal(err)
	}

	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("SMF deregistration through the SEPP: %d", resp.StatusCode)
	}

	if r := <-control.releases; r.psi != 5 || r.how != smf.HPLMNReleaseDuplicate {
		t.Fatalf("release = %+v", r)
	}

	h.Close()

	vsmfAwait(t, home, http.MethodDelete, "/sdm-subscriptions/", 1)

	if len(vsmfRequests(home, http.MethodDelete, "/smf-registrations/5")) != 0 {
		t.Fatal("after a duplicate the V-SMF must keep the new SMF's registration and drop only its own subscription")
	}
}

func TestVSMFRegistrationNamesThePGWForEPSInterworking(t *testing.T) {
	for _, iwk := range []bool{false, true} {
		f := newVSMFFixture(t)
		f.target.EPSInterworking = iwk

		if err := f.mustOpen(t, 5).Register(context.Background()); err != nil {
			t.Fatal(err)
		}

		reg, ok := f.home.SMFRegistration(vsmfSupi, "5")
		if !ok {
			t.Fatal("no SMF registration")
		}

		want := ""
		if iwk {
			want = "pgw1.node.epc.mnc070.mcc999.3gppnetwork.org"
		}

		if reg.PgwFqdn != want {
			t.Fatalf("EPS interworking %v: pgwFqdn = %q, want %q", iwk, reg.PgwFqdn, want)
		}
	}
}

func TestVSMFRefusesNewSessionsOnceBarredButKeepsTheSession(t *testing.T) {
	f := newVSMFFixture(t)
	h := f.mustOpen(t, 5)

	f.setSMData(strings.Replace(vsmfSMDataBody, `"dnnConfigurations"`, `"odbPacketServices": "ROAMER_ACCESS_VPLMN_AP", "dnnConfigurations"`, 1))

	f.v.refresh(context.Background(), h)

	if r := h.Rules(); r.Withdrawn {
		t.Fatal("a change of barring released the existing session; only the AMF acts on existing sessions (TS 29.503 NOTE 2)")
	}

	_, err := f.open(t, 6, "internet")

	var se *SessionError
	if !errors.As(err, &se) || se.GSMCause() != fgs.GSMCauseOperatorDeterminedBarring {
		t.Fatalf("OpenSubscription after barring = %v, want cause #8", err)
	}
}
