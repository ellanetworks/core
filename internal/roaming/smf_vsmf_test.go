// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/sepp/sepptest"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

const (
	vsmfIMSI        = "001010000000001"
	vsmfSupi        = "imsi-" + vsmfIMSI
	vsmfUDMRoot     = "http://udm.5gc.mnc001.mcc001.3gppnetwork.org"
	vsmfUDMSearch   = `{"validityPeriod": 30, "nfInstances": [{"nfInstanceId": "00000000-0000-0000-0000-0000000000d1", "nfType": "UDM", "nfStatus": "REGISTERED", "fqdn": "udm.5gc.mnc001.mcc001.3gppnetwork.org", "nfServices": [{"serviceInstanceId": "sdm", "serviceName": "nudm-sdm", "versions": [{"apiVersionInUri": "v2", "apiFullVersion": "2.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED"}, {"serviceInstanceId": "uecm", "serviceName": "nudm-uecm", "versions": [{"apiVersionInUri": "v1", "apiFullVersion": "1.0.0"}], "scheme": "http", "nfServiceStatus": "REGISTERED"}]}]}`
	vsmfSMDataBody  = `[{"singleNssai": {"sst": 1, "sd": "010203"}, "dnnConfigurations": {"internet": {"pduSessionTypes": {"defaultSessionType": "IPV4", "allowedSessionTypes": ["IPV4V6"]}, "sscModes": {"defaultSscMode": "SSC_MODE_1"}, "5gQosProfile": {"5qi": 7, "arp": {"priorityLevel": 8, "preemptCap": "NOT_PREEMPT", "preemptVuln": "PREEMPTABLE"}}, "sessionAmbr": {"uplink": "50 Mbps", "downlink": "150 Mbps"}}}}]`
	vsmfSMDataLower = `[{"singleNssai": {"sst": 1, "sd": "010203"}, "dnnConfigurations": {"internet": {"pduSessionTypes": {"defaultSessionType": "IPV4"}, "sscModes": {"defaultSscMode": "SSC_MODE_1"}, "sessionAmbr": {"uplink": "10 Mbps", "downlink": "20 Mbps"}}}}]`
	vsmfSMDataOther = `[{"singleNssai": {"sst": 1, "sd": "010203"}, "dnnConfigurations": {"corporate": {"pduSessionTypes": {"defaultSessionType": "IPV4"}, "sscModes": {"defaultSscMode": "SSC_MODE_1"}}}}]`
)

var (
	vsmfSnssai = models.Snssai{Sst: 1, Sd: "010203"}
	vsmfHome   = wire.PlmnID{Mcc: "001", Mnc: "01"}
	vsmfOP     = wire.PlmnID{Mcc: "999", Mnc: "70"}
)

type vsmfRequest struct {
	method string
	path   string
	query  url.Values
	header http.Header
	body   string
}

type vsmfFakeReply struct {
	status int
	body   string
}

type vsmfFakeUDM struct {
	mu            sync.Mutex
	requests      []vsmfRequest
	smData        []vsmfFakeReply
	uecm          []vsmfFakeReply
	expires       string
	subscriptions int
}

func (f *vsmfFakeUDM) next(list *[]vsmfFakeReply, fallback vsmfFakeReply) vsmfFakeReply {
	if len(*list) == 0 {
		return fallback
	}

	r := (*list)[0]
	if len(*list) > 1 {
		*list = (*list)[1:]
	}

	return r
}

func (f *vsmfFakeUDM) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}

	f.mu.Lock()
	f.requests = append(f.requests, vsmfRequest{method: r.Method, path: r.URL.Path, query: r.URL.Query(), header: r.Header.Clone(), body: string(body)})

	var rep vsmfFakeReply

	p := r.URL.Path

	switch {
	case p == "/nnrf-disc/v1/nf-instances":
		rep = vsmfFakeReply{http.StatusOK, vsmfUDMSearch}
	case strings.HasSuffix(p, "/sm-data") && r.Method == http.MethodGet:
		rep = f.next(&f.smData, vsmfFakeReply{http.StatusOK, vsmfSMDataBody})
	case strings.HasSuffix(p, "/sdm-subscriptions") && r.Method == http.MethodPost:
		f.subscriptions++
		w.Header().Set("Location", vsmfUDMRoot+p+"/sub-"+string(rune('0'+f.subscriptions)))
		rep = vsmfFakeReply{http.StatusCreated, strings.TrimSuffix(string(body), "}") + `,"expires":"` + f.expires + `"}`}

		if f.expires == "" {
			rep.body = string(body)
		}
	case strings.Contains(p, "/sdm-subscriptions/") && r.Method == http.MethodDelete:
		rep = vsmfFakeReply{http.StatusNoContent, ""}
	case strings.Contains(p, "/sdm-subscriptions/") && r.Method == http.MethodPatch:
		rep = vsmfFakeReply{http.StatusNoContent, ""}
	case strings.Contains(p, "/smf-registrations/") && r.Method == http.MethodPut:
		rep = f.next(&f.uecm, vsmfFakeReply{http.StatusCreated, string(body)})
	case strings.Contains(p, "/smf-registrations/") && r.Method == http.MethodDelete:
		rep = vsmfFakeReply{http.StatusNoContent, ""}
	default:
		rep = vsmfFakeReply{http.StatusNotFound, `{"cause":"RESOURCE_URI_STRUCTURE_NOT_FOUND"}`}
	}
	f.mu.Unlock()

	if rep.status >= 400 {
		w.Header().Set("Content-Type", wire.ContentTypeProblem)
	} else if rep.body != "" {
		w.Header().Set("Content-Type", wire.ContentTypeJSON)
	}

	w.WriteHeader(rep.status)
	_, _ = w.Write([]byte(rep.body))
}

func (f *vsmfFakeUDM) find(method, suffix string) []vsmfRequest {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []vsmfRequest

	for _, r := range f.requests {
		if r.method == method && strings.HasSuffix(r.path, suffix) {
			out = append(out, r)
		}
	}

	return out
}

func (f *vsmfFakeUDM) count(method, contains string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	n := 0

	for _, r := range f.requests {
		if r.method == method && strings.Contains(r.path, contains) {
			n++
		}
	}

	return n
}

type vsmfN32 struct {
	*fakeN32
	down bool
}

func (n *vsmfN32) SEPPID() int { return 1 }

func (n *vsmfN32) Client(partnerID string) *http.Client {
	if n.down {
		return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, sepp.ErrNoContext })}
	}

	return n.fakeN32.Client(partnerID)
}

type vsmfVisitors map[string]SessionVisitor

func (f vsmfVisitors) SessionVisitor(imsi string) (SessionVisitor, bool) {
	v, ok := f[imsi]
	return v, ok
}

type vsmfLocal struct{}

func (vsmfLocal) SessionManagement(context.Context, string) (*udm.SessionManagementSubscription, error) {
	gbps := models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}

	return &udm.SessionManagementSubscription{Visitor: true, DNNs: []udm.DNNConfiguration{
		{Snssai: vsmfSnssai, DNN: "internet", Default: true, Var5qi: 9, Arp: models.Arp{PriorityLevel: 1}, SessionAMBR: gbps},
		{Snssai: vsmfSnssai, DNN: "corporate", Var5qi: 9, Arp: models.Arp{PriorityLevel: 1}, SessionAMBR: gbps},
		{Snssai: vsmfSnssai, DNN: "ims", IMS: true, Var5qi: 5, Arp: models.Arp{PriorityLevel: 1}, SessionAMBR: gbps},
	}}, nil
}

type vsmfReleased struct {
	psi uint8
	h   smf.HomeSession
	how smf.HomeRelease
}

type vsmfControl struct {
	releases   chan vsmfReleased
	reconciles chan etsi.SUPI
	found      bool
}

func (f *vsmfControl) ReleaseHomeSession(_ context.Context, _ etsi.SUPI, psi uint8, h smf.HomeSession, how smf.HomeRelease) bool {
	f.releases <- vsmfReleased{psi: psi, h: h, how: how}
	return f.found
}

func (f *vsmfControl) ReconcileHomeSessions(_ context.Context, supi etsi.SUPI) {
	f.reconciles <- supi
}

type vsmfFallback struct{}

func (vsmfFallback) SessionManagement(context.Context, string) (*udm.SessionManagementSubscription, error) {
	return nil, ErrNoHomeSubscription
}

func (vsmfFallback) FramedRoutes(context.Context, string, string) ([]netip.Prefix, error) {
	return nil, nil
}

func (vsmfFallback) StaticIP(context.Context, string, string, bool) (netip.Addr, bool, error) {
	return netip.Addr{}, false, nil
}

func (vsmfFallback) N26Allowed(context.Context, string) (bool, error) {
	return false, nil
}

func vsmfSelection(lbo bool) *wire.SmfSelectionSubscriptionData {
	return &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
		"1-010203": {DnnInfos: []wire.DnnInfo{
			{Dnn: "internet", DefaultDnnIndicator: true, LboRoamingAllowed: lbo},
			{Dnn: "corporate", LboRoamingAllowed: true},
			{Dnn: "ims", LboRoamingAllowed: true},
		}},
	}}
}

type vsmfFixture struct {
	v       *VSMF
	home    *vsmfFakeUDM
	n32     *vsmfN32
	control *vsmfControl
	visitor SessionVisitor
}

func newVSMFFixture(t *testing.T) *vsmfFixture {
	t.Helper()

	home := &vsmfFakeUDM{}
	n32 := &vsmfN32{fakeN32: &fakeN32{handler: home}}
	control := &vsmfControl{releases: make(chan vsmfReleased, 4), reconciles: make(chan etsi.SUPI, 4), found: true}
	visitor := SessionVisitor{PartnerID: "p", HomePLMN: vsmfHome, ServingPLMN: vsmfOP, RoutingIndicator: "0000", SMFSelection: vsmfSelection(true)}

	v := NewVSMF(vsmfLocal{}, zap.NewNop())
	v.registerBackoff = time.Millisecond
	v.SetN32(n32)
	v.SetVisitors(vsmfVisitors{vsmfIMSI: visitor})
	v.SetSessions(control)

	t.Cleanup(v.Settle)

	return &vsmfFixture{v: v, home: home, n32: n32, control: control, visitor: visitor}
}

func vsmfSUPI(t *testing.T) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(vsmfIMSI)
	if err != nil {
		t.Fatal(err)
	}

	return supi
}

func (f *vsmfFixture) open(t *testing.T, psi uint8, dnn string) (smf.HomeSession, error) {
	t.Helper()

	return f.v.OpenSession(context.Background(), smf.HomeSessionRequest{Supi: vsmfSUPI(t), PDUSessionID: psi, Snssai: vsmfSnssai, Dnn: dnn})
}

func (f *vsmfFixture) mustOpen(t *testing.T, psi uint8) smf.HomeSession {
	t.Helper()

	h, err := f.open(t, psi, "internet")
	if err != nil || h == nil {
		t.Fatalf("OpenSession = %v, %v", h, err)
	}

	return h
}

func vsmfJSON(t *testing.T, s string) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}

	return out
}

func TestVSMFOpenGetsSubscribesAndMergesTheHomeSubscription(t *testing.T) {
	f := newVSMFFixture(t)

	h := f.mustOpen(t, 5)

	rules := h.Rules()
	if rules.HomeSnssai != vsmfSnssai || len(rules.PDUSessionTypes) != 2 || rules.PDUSessionTypes[0] != fgs.PDUSessionTypeIPv4 || rules.PDUSessionTypes[1] != fgs.PDUSessionTypeIPv4v6 {
		t.Fatalf("rules = %+v", rules)
	}

	gets := f.home.find(http.MethodGet, "/nudm-sdm/v2/"+vsmfSupi+"/sm-data")
	if len(gets) != 1 {
		t.Fatalf("sm-data GETs = %d", len(gets))
	}

	q := gets[0].query
	if q.Get("single-nssai") != `{"sst":1,"sd":"010203"}` || q.Get("dnn") != "internet" || q.Get("plmn-id") != `{"mcc":"999","mnc":"70"}` {
		t.Fatalf("sm-data query = %v", q)
	}

	if gets[0].header.Get("User-Agent") != "SMF" {
		t.Fatalf("User-Agent = %q", gets[0].header.Get("User-Agent"))
	}

	nrf := f.home.find(http.MethodGet, "/nnrf-disc/v1/nf-instances")
	if len(nrf) == 0 || nrf[0].query.Get("requester-nf-type") != "SMF" || nrf[0].query.Get("target-nf-type") != "UDM" || nrf[0].query.Get("service-names") != "nudm-sdm" {
		t.Fatalf("discovery = %+v", nrf)
	}

	f.v.Settle()

	subs := f.home.find(http.MethodPost, "/nudm-sdm/v2/"+vsmfSupi+"/sdm-subscriptions")
	if len(subs) != 1 {
		t.Fatalf("SDM subscriptions = %d", len(subs))
	}

	sub := vsmfJSON(t, subs[0].body)
	if sub["callbackReference"] != "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+vsmfSupi+"/sdm-change" ||
		sub["dnn"] != "internet" || sub["implicitUnsubscribe"] != true ||
		sub["monitoredResourceUris"].([]any)[0] != "/nudm-sdm/v2/"+vsmfSupi+"/sm-data" ||
		sub["nfInstanceId"] != f.v.instanceID(vsmfOP, 1) {
		t.Fatalf("SDM subscription = %s", subs[0].body)
	}

	subs2 := f.v.Subscriptions(vsmfFallback{})

	sm, err := subs2.SessionManagement(context.Background(), vsmfIMSI)
	if err != nil {
		t.Fatal(err)
	}

	if !sm.Visitor || sm.HomePLMN != vsmfHome.Model() || len(sm.DNNs) != 1 {
		t.Fatalf("merged subscription = %+v", sm)
	}

	c := sm.DNNs[0]
	if c.DNN != "internet" || c.Var5qi != 9 || c.Arp.PriorityLevel != 1 || c.SessionAMBR.Uplink.Bps() != 50_000_000 || c.SessionAMBR.Downlink.Bps() != 150_000_000 {
		t.Fatalf("merged DNN = %+v, want local 5QI/ARP and min(home, local) Session-AMBR", c)
	}

	f.mustOpen(t, 6)

	if n := len(f.home.find(http.MethodGet, "/sm-data")); n != 1 {
		t.Fatalf("sm-data GETs after a second session on the same DNN = %d, want the cached data", n)
	}
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

	puts := f.home.find(http.MethodPut, "/nudm-uecm/v1/"+vsmfSupi+"/registrations/smf-registrations/5")
	if len(puts) != 1 {
		t.Fatalf("UECM registrations = %d, want 1", len(puts))
	}

	reg := vsmfJSON(t, puts[0].body)
	if reg["smfInstanceId"] != f.v.instanceID(vsmfOP, 1) || reg["pduSessionId"] != float64(5) || reg["dnn"] != "internet" ||
		reg["deregCallbackUri"] != "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+vsmfSupi+"/uecm-dereg" {
		t.Fatalf("SMF registration = %s", puts[0].body)
	}

	if p := reg["plmnId"].(map[string]any); p["mcc"] != "999" || p["mnc"] != "70" {
		t.Fatalf("plmnId = %v, want the serving PLMN", p)
	}

	if s := reg["singleNssai"].(map[string]any); s["sst"] != float64(1) || s["sd"] != "010203" {
		t.Fatalf("singleNssai = %v", s)
	}

	first.Close()
	first.Close()
	f.v.Settle()

	if n := f.home.count(http.MethodDelete, "/smf-registrations/5"); n != 1 {
		t.Fatalf("UECM deregistrations = %d, want 1", n)
	}

	if n := f.home.count(http.MethodDelete, "/sdm-subscriptions/"); n != 0 {
		t.Fatal("vsmfFallback while another session uses the data")
	}

	second.Close()
	f.v.Settle()

	if n := f.home.count(http.MethodDelete, "/smf-registrations/6"); n != 0 {
		t.Fatal("deregistered a session that never registered")
	}

	if del := f.home.find(http.MethodDelete, "/sdm-subscriptions/sub-1"); len(del) != 1 {
		t.Fatalf("unsubscriptions = %d, want 1", len(del))
	}

	if _, _, ok := f.v.homeEntries(vsmfIMSI); ok {
		t.Fatal("the visitor's home data outlived its sessions")
	}
}

func TestVSMFRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		dnn     string
		setup   func(f *vsmfFixture)
		want    fgs.GSMCause
		wantErr error
	}{
		"IMS data network":         {dnn: "ims", want: fgs.GSMCauseMissingOrUnknownDNN, wantErr: ErrSessionNotLocalDN},
		"not a partner DN":         {dnn: "other", want: fgs.GSMCauseMissingOrUnknownDNNInASlice, wantErr: ErrSessionNotLocalDN},
		"home routed only":         {dnn: "internet", setup: func(f *vsmfFixture) { f.withSelection(vsmfSelection(false)) }, want: fgs.GSMCauseRequestedServiceOptionNotSubscribed, wantErr: ErrSessionHomeRouted},
		"no SMF selection data":    {dnn: "internet", setup: func(f *vsmfFixture) { f.withSelection(nil) }, want: fgs.GSMCauseRequestedServiceOptionNotSubscribed, wantErr: ErrSessionHomeRouted},
		"user unknown":             {dnn: "internet", setup: vsmfSMData404(`{"cause":"USER_NOT_FOUND"}`), want: fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, wantErr: ErrSessionHomeRefused},
		"data not found":           {dnn: "internet", setup: vsmfSMData404(`{"cause":"DATA_NOT_FOUND"}`), want: fgs.GSMCauseRequestedServiceOptionNotSubscribed, wantErr: ErrSessionNotSubscribed},
		"DNN not in sm-data":       {dnn: "internet", setup: vsmfSMData(http.StatusOK, vsmfSMDataOther), want: fgs.GSMCauseRequestedServiceOptionNotSubscribed, wantErr: ErrSessionNotSubscribed},
		"home UDM overloaded":      {dnn: "internet", setup: vsmfSMData(http.StatusServiceUnavailable, `{"cause":"NF_CONGESTION"}`), want: fgs.GSMCauseNetworkFailure, wantErr: ErrSessionHomeUnreachable},
		"N32 down":                 {dnn: "internet", setup: func(f *vsmfFixture) { f.n32.down = true }, want: fgs.GSMCauseNetworkFailure, wantErr: ErrSessionHomeUnreachable},
		"SSC mode 1 not allowed":   {dnn: "internet", setup: vsmfSMData(http.StatusOK, strings.Replace(vsmfSMDataBody, "SSC_MODE_1", "SSC_MODE_2", 1)), want: fgs.GSMCauseNotSupportedSSCMode, wantErr: ErrSessionSSCMode},
		"no IP PDU session type":   {dnn: "internet", setup: vsmfSMData(http.StatusOK, strings.NewReplacer(`"IPV4"`, `"ETHERNET"`, `["IPV4V6"]`, `["UNSTRUCTURED"]`).Replace(vsmfSMDataBody)), want: fgs.GSMCauseUnknownPDUSessionType, wantErr: ErrSessionNotIP},
		"shared data not resolved": {dnn: "internet", setup: vsmfSMData(http.StatusOK, `{"sharedSmSubsDataIds": ["001-01-1"]}`), want: fgs.GSMCauseRequestRejectedUnspecified, wantErr: ErrSessionHomeData},
	} {
		t.Run(name, func(t *testing.T) {
			f := newVSMFFixture(t)
			if tc.setup != nil {
				tc.setup(f)
			}

			h, err := f.open(t, 5, tc.dnn)

			var se *SessionError
			if h != nil || !errors.As(err, &se) || se.GSMCause() != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("OpenSession = %v, %v; want cause %d and %v", h, err, tc.want, tc.wantErr)
			}

			if n := f.home.count(http.MethodPost, "/sdm-subscriptions"); n != 0 {
				t.Fatal("subscribed for a refused session")
			}

			if _, _, ok := f.v.homeEntries(vsmfIMSI); ok {
				t.Fatal("kept home data for a refused session")
			}
		})
	}
}

func (f *vsmfFixture) withSelection(sel *wire.SmfSelectionSubscriptionData) {
	f.visitor.SMFSelection = sel
	f.v.SetVisitors(vsmfVisitors{vsmfIMSI: f.visitor})
}

func vsmfSMData(status int, body string) func(f *vsmfFixture) {
	return func(f *vsmfFixture) { f.home.smData = []vsmfFakeReply{{status, body}} }
}

func vsmfSMData404(body string) func(f *vsmfFixture) {
	return vsmfSMData(http.StatusNotFound, body)
}

func TestVSMFIgnoresHomeSubscribersAndUnknownVisitors(t *testing.T) {
	f := newVSMFFixture(t)
	f.v.SetVisitors(vsmfVisitors{})

	if h, err := f.open(t, 5, "internet"); h != nil || err != nil {
		t.Fatalf("OpenSession for a UE that is not a 5G visitor = %v, %v", h, err)
	}

	f.v.SetVisitors(nil)

	if h, err := f.open(t, 5, "internet"); h != nil || err != nil {
		t.Fatalf("OpenSession without the AMF's visitor contexts = %v, %v", h, err)
	}

	if len(f.home.find(http.MethodGet, "")) != 0 {
		t.Fatal("contacted the home network for a UE that is not a 5G visitor")
	}
}

func TestVSMFRegistrationErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		replies []vsmfFakeReply
		cause   fgs.GSMCause
		puts    int
	}{
		"roaming not allowed": {[]vsmfFakeReply{{http.StatusForbidden, `{"cause":"ROAMING_NOT_ALLOWED"}`}}, fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, 1},
		"DNN not allowed":     {[]vsmfFakeReply{{http.StatusForbidden, `{"cause":"DNN_NOT_ALLOWED"}`}}, fgs.GSMCauseMissingOrUnknownDNN, 1},
		"user not found":      {[]vsmfFakeReply{{http.StatusNotFound, `{"cause":"USER_NOT_FOUND"}`}}, fgs.GSMCauseUserAuthenticationOrAuthorizationFailed, 1},
		"home UDM failing":    {[]vsmfFakeReply{{http.StatusInternalServerError, `{}`}}, 0, vsmfRegisterAttempts},
		"recovers":            {[]vsmfFakeReply{{http.StatusServiceUnavailable, `{}`}, {http.StatusCreated, `{}`}}, 0, 2},
	} {
		t.Run(name, func(t *testing.T) {
			f := newVSMFFixture(t)
			f.home.uecm = tc.replies

			h := f.mustOpen(t, 5)
			err := h.Register(context.Background())

			var se *SessionError

			switch {
			case tc.cause != 0 && (!errors.As(err, &se) || se.GSMCause() != tc.cause):
				t.Fatalf("Register = %v, want cause %d", err, tc.cause)
			case tc.cause == 0 && errors.As(err, &se):
				t.Fatalf("Register = %v, want no 5GSM cause for a transient failure", err)
			case name == "recovers" && err != nil:
				t.Fatalf("Register = %v", err)
			}

			if n := f.home.count(http.MethodPut, "/smf-registrations/5"); n != tc.puts {
				t.Fatalf("UECM PUTs = %d, want %d", n, tc.puts)
			}
		})
	}
}

func vsmfCallback(t *testing.T, v *VSMF, partnerID string, kind sepp.CallbackKind, body string) *httptest.ResponseRecorder {
	t.Helper()

	peer := sepp.Peer{PartnerID: partnerID, PLMNs: []wire.PlmnID{vsmfHome}, Callback: sepp.Callback{NF: sepp.CallbackSMF, SEPPID: 1, Supi: vsmfSupi, Kind: kind}}
	v.peer = func(context.Context) (sepp.Peer, bool) { return peer, true }

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "https://smf1.5gc.mnc070.mcc999.3gppnetwork.org"+peer.Callback.Path(), bytes.NewReader([]byte(body)))
	rec := httptest.NewRecorder()
	v.ServeHTTP(rec, req)

	return rec
}

func TestVSMFDataChangeRefreshesAndReconciles(t *testing.T) {
	f := newVSMFFixture(t)
	f.mustOpen(t, 5)
	f.v.Settle()

	change := `{"notifyItems": [{"resourceId": "` + vsmfUDMRoot + `/nudm-sdm/v2/` + vsmfSupi + `/sm-data", "changes": [{"op": "REPLACE", "path": "/0/dnnConfigurations/internet/sessionAmbr"}]}]}`

	if rec := vsmfCallback(t, f.v, "other", sepp.CallbackSDMChange, change); rec.Code != http.StatusNotFound || wire.ParseProblem(rec.Code, rec.Body.Bytes()).Cause != wire.CauseSubscriptionNotFound {
		t.Fatalf("notification from another partner: %d %s", rec.Code, rec.Body)
	}

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, `{"notifyItems": [], "subscriptionId": "sub-9"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty notification: %d %s", rec.Code, rec.Body)
	}

	f.home.mu.Lock()
	f.home.smData = []vsmfFakeReply{{http.StatusOK, vsmfSMDataLower}}
	f.home.mu.Unlock()

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

	sm, err := f.v.Subscriptions(vsmfFallback{}).SessionManagement(context.Background(), vsmfIMSI)
	if err != nil || len(sm.DNNs) != 1 || sm.DNNs[0].SessionAMBR.Uplink.Bps() != 10_000_000 {
		t.Fatalf("merged subscription after the change = %+v, %v", sm, err)
	}

	f.home.mu.Lock()
	f.home.smData = []vsmfFakeReply{{http.StatusOK, vsmfSMDataOther}}
	f.home.mu.Unlock()

	relative := strings.Replace(change, vsmfUDMRoot+"/nudm-sdm/v2/", "", 1)

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, relative); rec.Code != http.StatusNoContent {
		t.Fatalf("notification: %d %s", rec.Code, rec.Body)
	}

	<-f.control.reconciles

	sm, err = f.v.Subscriptions(vsmfFallback{}).SessionManagement(context.Background(), vsmfIMSI)
	if err != nil || len(sm.DNNs) != 0 {
		t.Fatalf("merged subscription after a withdrawal = %+v, %v; want no DNN", sm, err)
	}

	if rec := vsmfCallback(t, f.v, "p", sepp.CallbackSDMChange, strings.Replace(change, `"notifyItems"`, `"subscriptionId": "sub-7", "notifyItems"`, 1)); rec.Code != http.StatusNotFound {
		t.Fatalf("notification for another subscription: %d %s", rec.Code, rec.Body)
	}
}

func TestVSMFDeregistrationNotification(t *testing.T) {
	for name, tc := range map[string]struct {
		reason     wire.DeregistrationReason
		how        smf.HomeRelease
		deregister int
	}{
		"duplicate":    {wire.DeregDuplicatePDUSession, smf.HomeReleaseDuplicate, 0},
		"transferred":  {wire.DeregSMFContextTransferred, smf.HomeReleaseDuplicate, 0},
		"ePDG":         {wire.DeregDuplicatePDUSessionEPDG, smf.HomeReleaseDuplicateNotify, 0},
		"reactivation": {wire.DeregPDUSessionReactivationRequired, smf.HomeReleaseReactivate, 1},
		"other":        {wire.DeregSubscriptionWithdrawn, smf.HomeReleaseDeactivate, 0},
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

			h.Close()
			f.v.Settle()

			if n := f.home.count(http.MethodDelete, "/smf-registrations/5"); n != tc.deregister {
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

	deadline := time.Now().Add(2 * time.Second)

	for {
		f.v.Settle()

		if _, _, ok := f.v.homeEntries(vsmfIMSI); !ok {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("the home session was kept after the SMF reported no such session")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if n := f.home.count(http.MethodDelete, "/smf-registrations/5"); n != 0 {
		t.Fatal("deregistered a registration the home UDM moved to another SMF")
	}
}

func TestVSMFRenewsAnExpiringSDMSubscription(t *testing.T) {
	f := newVSMFFixture(t)
	f.home.expires = time.Now().Add(300 * time.Millisecond).UTC().Format(time.RFC3339Nano)

	h := f.mustOpen(t, 5)

	deadline := time.Now().Add(3 * time.Second)
	for f.home.count(http.MethodPatch, "/sdm-subscriptions/sub-1") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the SDM subscription was not renewed before it expired")
		}

		time.Sleep(20 * time.Millisecond)
	}

	patch := f.home.find(http.MethodPatch, "/sdm-subscriptions/sub-1")[0]
	if patch.header.Get("Content-Type") != wire.ContentTypeMergePatch || !strings.Contains(patch.body, `"expires"`) {
		t.Fatalf("renewal = %+v", patch)
	}

	h.Close()
	f.v.Settle()
}

func TestVSMFCallbacksForAnotherNF(t *testing.T) {
	f := newVSMFFixture(t)
	f.v.peer = func(context.Context) (sepp.Peer, bool) {
		return sepp.Peer{PartnerID: "p", Callback: sepp.Callback{NF: sepp.CallbackAMF, SEPPID: 1, Supi: vsmfSupi, Kind: sepp.CallbackUECMDereg}}, true
	}

	rec := httptest.NewRecorder()
	f.v.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("AMF vsmfCallback served by the V-SMF: %d", rec.Code)
	}
}

func TestVSMFThroughTheSEPP(t *testing.T) {
	home := &vsmfFakeUDM{}
	partner := sepptest.NewPartner(t, vsmfHome, home)
	ella := sepptest.NewElla(t, vsmfOP, 1, partner.Config("p"))
	control := &vsmfControl{releases: make(chan vsmfReleased, 4), reconciles: make(chan etsi.SUPI, 4), found: true}

	v := NewVSMF(vsmfLocal{}, zap.NewNop())
	v.SetN32(ella.Manager)
	v.SetVisitors(vsmfVisitors{vsmfIMSI: {PartnerID: "p", HomePLMN: vsmfHome, ServingPLMN: vsmfOP, SMFSelection: vsmfSelection(true)}})
	v.SetSessions(control)
	ella.Manager.SetCallbacks(v)

	t.Cleanup(v.Settle)

	ctx := context.Background()

	h, err := v.OpenSession(ctx, smf.HomeSessionRequest{Supi: vsmfSUPI(t), PDUSessionID: 5, Snssai: vsmfSnssai, Dnn: "internet"})
	if err != nil || h == nil {
		t.Fatalf("OpenSession over N32 = %v, %v", h, err)
	}

	if err := h.Register(ctx); err != nil {
		t.Fatalf("Register over N32: %v", err)
	}

	v.Settle()

	for _, path := range []string{"/sm-data", "/sdm-subscriptions"} {
		if len(home.find(http.MethodGet, path))+len(home.find(http.MethodPost, path)) != 1 {
			t.Fatalf("the home UDM did not receive %s over N32", path)
		}
	}

	puts := home.find(http.MethodPut, "/smf-registrations/5")
	if len(puts) != 1 || puts[0].header.Get(sepp.HeaderOriginatingNetworkID) != "999-70" {
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

	if r := <-control.releases; r.psi != 5 || r.how != smf.HomeReleaseDuplicate {
		t.Fatalf("release = %+v", r)
	}

	h.Close()
	v.Settle()

	if home.count(http.MethodDelete, "/smf-registrations/5") != 0 || home.count(http.MethodDelete, "/sdm-subscriptions/") != 1 {
		t.Fatal("after a duplicate the V-SMF must keep the new SMF's registration and drop only its own subscription")
	}
}
