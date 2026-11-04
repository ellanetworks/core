// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/fgs"
)

type causeError struct {
	cause fgs.GSMCause
}

func (e causeError) Error() string { return "home network refusal" }

func (e causeError) GSMCause() fgs.GSMCause { return e.cause }

type fakeHPLMNSubscription struct {
	mu          sync.Mutex
	rules       smf.HPLMNRules
	registerErr error
	registered  chan struct{}
	closed      int
}

func (h *fakeHPLMNSubscription) Rules() smf.HPLMNRules {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.rules
}

func (h *fakeHPLMNSubscription) Register(context.Context) error {
	defer close(h.registered)

	return h.registerErr
}

func (h *fakeHPLMNSubscription) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.closed++
}

func (h *fakeHPLMNSubscription) closes() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.closed
}

type fakeHPLMNClient struct {
	mu          sync.Mutex
	err         error
	rules       smf.HPLMNRules
	registerErr error
	requests    []smf.HPLMNSubscriptionRequest
	sessions    []*fakeHPLMNSubscription
}

func (f *fakeHPLMNClient) OpenSubscription(_ context.Context, req smf.HPLMNSubscriptionRequest) (smf.HPLMNSubscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.requests = append(f.requests, req)

	if f.err != nil {
		return nil, f.err
	}

	h := &fakeHPLMNSubscription{rules: f.rules, registerErr: f.registerErr, registered: make(chan struct{})}
	f.sessions = append(f.sessions, h)

	return h, nil
}

func (f *fakeHPLMNClient) session(t *testing.T) *fakeHPLMNSubscription {
	t.Helper()

	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.sessions) != 1 {
		t.Fatalf("home sessions opened = %d, want 1", len(f.sessions))
	}

	return f.sessions[0]
}

var visitorHPLMN = models.PlmnID{Mcc: "001", Mnc: "01"}

type roamingSubscriptions struct {
	*fakeStore
}

func (r roamingSubscriptions) SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error) {
	sm, err := r.fakeStore.SessionManagement(ctx, imsi)
	if err != nil {
		return nil, err
	}

	sm.Visitor, sm.HomePLMN = true, visitorHPLMN

	return sm, nil
}

type hplmnFixture struct {
	s     *smf.SMF
	pcf   *fakePCF
	store *fakeStore
	upf   *fakeUPF
	amf   *fakeAMF
}

func hplmnSMF(home *fakeHPLMNClient) hplmnFixture {
	pcf, store, upf, amfCb := defaultFakes()
	store.pcf = pcf
	s := smf.New(pcf, store, upf, amfCb, smf.WithSubscriptions(roamingSubscriptions{store}), smf.WithHPLMNClient(home))

	return hplmnFixture{s: s, pcf: pcf, store: store, upf: upf, amf: amfCb}
}

func hplmnRules() smf.HPLMNRules {
	return smf.HPLMNRules{HPLMNSnssai: testSnssai, PDUSessionTypes: []fgs.PDUSessionType{fgs.PDUSessionTypeIPv4, fgs.PDUSessionTypeIPv4v6}}
}

func testTarget() *smf.HPLMNTarget {
	return &smf.HPLMNTarget{PartnerID: "p", HPLMN: visitorHPLMN, ServingNetwork: models.PlmnID{Mcc: "999", Mnc: "70"}, HPLMNSnssai: *testSnssai, LBO: true}
}

func waitRegistered(t *testing.T, h *fakeHPLMNSubscription) {
	t.Helper()

	select {
	case <-h.registered:
	case <-time.After(2 * time.Second):
		t.Fatal("the SMF did not register the session with the home network")
	}
}

func TestVisitorSessionIsOpenedRegisteredAndClosedAtTheHomeNetwork(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	f := hplmnSMF(home)
	s, amfCb := f.s, f.amf
	ctx := context.Background()

	ref, rsp, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget())
	if err != nil || rsp != nil {
		t.Fatalf("CreateSmContext = %v, %d-byte reject", err, len(rsp))
	}

	home.mu.Lock()
	req := home.requests[0]
	home.mu.Unlock()

	if req.Supi != testSUPI() || req.PDUSessionID != 1 || req.Dnn != testDNN || req.Target != *testTarget() {
		t.Fatalf("home session request = %+v", req)
	}

	amfCb.mu.Lock()
	n1 := amfCb.n1n2Calls[0].n1Msg
	amfCb.mu.Unlock()

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(n1)
	if err != nil {
		t.Fatal(err)
	}

	if acc.SNSSAI == nil || acc.SNSSAI.MappedHPLMNSST == nil || *acc.SNSSAI.MappedHPLMNSST != 1 || acc.SNSSAI.MappedHPLMNSD == nil || *acc.SNSSAI.MappedHPLMNSD != [3]byte{1, 2, 3} {
		t.Fatalf("accept S-NSSAI = %+v, want the mapped HPLMN S-NSSAI 1/010203", acc.SNSSAI)
	}

	h := home.session(t)

	select {
	case <-h.registered:
		t.Fatal("the session was registered before the access network accepted it")
	default:
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatal(err)
	}

	waitRegistered(t, h)

	if err := s.ReleaseSmContext(ctx, ref, models.ReleaseFor(models.ReleaseUERequested)); err != nil {
		t.Fatal(err)
	}

	if h.closes() != 1 {
		t.Fatalf("home session closed %d times, want 1", h.closes())
	}
}

func TestHPLMNRefusalIsTheEstablishmentRejectCause(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want fgs.GSMCause
	}{
		"home routed only":    {causeError{fgs.GSMCauseRequestedServiceOptionNotSubscribed}, fgs.GSMCauseRequestedServiceOptionNotSubscribed},
		"user unknown":        {causeError{fgs.GSMCauseUserAuthenticationOrAuthorizationFailed}, fgs.GSMCauseUserAuthenticationOrAuthorizationFailed},
		"home unreachable":    {causeError{fgs.GSMCauseNetworkFailure}, fgs.GSMCauseNetworkFailure},
		"SSC mode":            {causeError{fgs.GSMCauseNotSupportedSSCMode}, fgs.GSMCauseNotSupportedSSCMode},
		"without a 5GSM code": {errors.New("boom"), fgs.GSMCauseRequestRejectedUnspecified},
	} {
		t.Run(name, func(t *testing.T) {
			f := hplmnSMF(&fakeHPLMNClient{err: tc.err})
			s, store, upf := f.s, f.store, f.upf

			_, rsp, err := s.CreateSmContext(context.Background(), testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget())
			if err == nil || rsp == nil {
				t.Fatalf("CreateSmContext = %v, reject %v", err, rsp)
			}

			if got := fgs.GSMCause(rejectCauseCode(t, rsp)); got != tc.want {
				t.Fatalf("reject cause = %d, want %d", got, tc.want)
			}

			upf.mu.Lock()
			established := upf.lastEstablish != nil
			upf.mu.Unlock()

			store.mu.Lock()
			allocated := len(store.allocSessionLog)
			store.mu.Unlock()

			if established || allocated != 0 || s.SessionCount() != 0 {
				t.Fatal("the SMF allocated resources for a session the home network refused")
			}
		})
	}
}

func TestHPLMNPDUSessionTypesNarrowTheSession(t *testing.T) {
	home := &fakeHPLMNClient{rules: smf.HPLMNRules{HPLMNSnssai: testSnssai, PDUSessionTypes: []fgs.PDUSessionType{fgs.PDUSessionTypeIPv4}}}
	f := hplmnSMF(home)
	s, pcf, store, amfCb := f.s, f.pcf, f.store, f.amf
	store.allocatedIPv6 = netip.MustParseAddr("2001:db8::")
	pcf.policy.IPv6Pool = "2001:db8::/48"

	dual := fgs.PDUSessionTypeIPv4v6
	req := &fgs.PDUSessionEstablishmentRequest{PDUSessionID: 1, PTI: 10, IntegrityProtMaxDataRate: [2]byte{0xff, 0xff}, PDUSessionType: &dual}

	n1, err := req.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ref, rsp, err := s.CreateSmContext(context.Background(), testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, n1, 0, testTarget())
	if err != nil || rsp != nil {
		t.Fatalf("CreateSmContext = %v, %d-byte reject", err, len(rsp))
	}

	if got := s.GetSession(ref).PDUSessionType; got != uint8(fgs.PDUSessionTypeIPv4) {
		t.Fatalf("PDU session type = %d, want IPv4 only (home allows IPV4)", got)
	}

	amfCb.mu.Lock()
	n1Accept := amfCb.n1n2Calls[0].n1Msg
	amfCb.mu.Unlock()

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(n1Accept)
	if err != nil {
		t.Fatal(err)
	}

	if acc.Cause == nil || *acc.Cause != fgs.GSMCausePDUSessionTypeIPv4OnlyAllowed {
		t.Fatalf("accept cause = %v, want #50", acc.Cause)
	}

	if s.GetSession(ref).PolicyData.IPv6Pool == "" {
		t.Fatal("the session policy lost its IPv6 pool, so a reconcile would see a pool change")
	}
}

func TestFailedEstablishmentClosesTheHPLMNSubscription(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	f := hplmnSMF(home)
	s, upf := f.s, f.upf
	upf.err = errors.New("UPF down")

	_, rsp, err := s.CreateSmContext(context.Background(), testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget())
	if err == nil || rsp == nil {
		t.Fatalf("CreateSmContext = %v, reject %v", err, rsp)
	}

	if home.session(t).closes() == 0 {
		t.Fatal("the home session of a failed establishment was not closed")
	}
}

func TestHPLMNRegistrationRefusalReleasesTheSession(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules(), registerErr: causeError{fgs.GSMCauseUserAuthenticationOrAuthorizationFailed}}
	f := hplmnSMF(home)
	s, amfCb := f.s, f.amf
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget())
	if err != nil {
		t.Fatal(err)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatal(err)
	}

	waitRegistered(t, home.session(t))

	var cmd *fgs.PDUSessionReleaseCommand

	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		amfCb.mu.Lock()
		if len(amfCb.releaseCalls) > 0 {
			cmd, err = fgs.ParsePDUSessionReleaseCommand(amfCb.releaseCalls[0].n1Msg)
		}
		amfCb.mu.Unlock()

		if cmd != nil || err != nil {
			break
		}
	}

	if err != nil || cmd == nil || cmd.Cause != fgs.GSMCauseUserAuthenticationOrAuthorizationFailed {
		t.Fatalf("release command = %+v, %v; want cause #29", cmd, err)
	}
}

func TestHPLMNDeregistrationReleasesTheSession(t *testing.T) {
	for name, tc := range map[string]struct {
		how      smf.HPLMNRelease
		signal   fgs.GSMCause
		dropped  int
		inPool   bool
		releases int
	}{
		"duplicate":           {how: smf.HPLMNReleaseDuplicate},
		"duplicate via ePDG":  {how: smf.HPLMNReleaseDuplicateNotify, dropped: 1},
		"reactivation":        {how: smf.HPLMNReleaseReactivate, signal: fgs.GSMCauseReactivationRequested, inPool: true, releases: 1},
		"other home decision": {how: smf.HPLMNReleaseDeactivate, signal: fgs.GSMCauseRegularDeactivation, inPool: true, releases: 1},
	} {
		t.Run(name, func(t *testing.T) {
			home := &fakeHPLMNClient{rules: hplmnRules()}
			f := hplmnSMF(home)
			s, amfCb := f.s, f.amf
			ctx := context.Background()

			ref, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget())
			if err != nil {
				t.Fatal(err)
			}

			h := home.session(t)

			if s.ReleaseHPLMNBoundSession(ctx, testSUPI(), 2, h, tc.how) {
				t.Fatal("released a PDU session the home session does not belong to")
			}

			if s.ReleaseHPLMNBoundSession(ctx, testSUPI(), 1, &fakeHPLMNSubscription{}, tc.how) {
				t.Fatal("released a PDU session of another home session")
			}

			if !s.ReleaseHPLMNBoundSession(ctx, testSUPI(), 1, h, tc.how) {
				t.Fatal("the session was not found")
			}

			if (s.GetSession(ref) != nil) != tc.inPool {
				t.Fatalf("session in pool = %v, want %v", s.GetSession(ref) != nil, tc.inPool)
			}

			amfCb.mu.Lock()
			releases, dropped := amfCb.releaseCalls, len(amfCb.droppedCalls)
			amfCb.mu.Unlock()

			if len(releases) != tc.releases || dropped != tc.dropped {
				t.Fatalf("release commands = %d, dropped notifications = %d; want %d, %d", len(releases), dropped, tc.releases, tc.dropped)
			}

			if tc.releases > 0 {
				cmd, err := fgs.ParsePDUSessionReleaseCommand(releases[0].n1Msg)
				if err != nil || cmd.Cause != tc.signal {
					t.Fatalf("release command = %+v, %v; want cause %d", cmd, err, tc.signal)
				}
			}

			if !tc.inPool && h.closes() != 1 {
				t.Fatalf("home session closed %d times, want 1", h.closes())
			}
		})
	}
}

func TestVisitorFiveGUsageIsRecordedWithItsHomePLMN(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	f := hplmnSMF(home)
	s, store, upf := f.s, f.store, f.upf
	ctx := context.Background()

	ref, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget())
	if err != nil {
		t.Fatal(err)
	}

	upf.count(s.GetSession(ref).PFCPContext.SEID, 300, 400)
	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	want := models.SubscriberUsage{EpochDay: today(), IMSI: testIMSI, HomePLMN: visitorHPLMN, UplinkVolume: 300, DownlinkVolume: 400}
	if len(store.usageRows) != 1 || store.usageRows[0] != want {
		t.Fatalf("usage = %+v, want [%+v]", store.usageRows, want)
	}
}

func TestReconcileHPLMNBoundSessionsReleasesAWithdrawnSession(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	f := hplmnSMF(home)
	s, pcf, amfCb := f.s, f.pcf, f.amf
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget()); err != nil {
		t.Fatal(err)
	}

	pcf.mu.Lock()
	pcf.policy = nil
	pcf.mu.Unlock()

	s.ReconcileHPLMNBoundSessions(ctx, testSUPI())

	amfCb.mu.Lock()
	defer amfCb.mu.Unlock()

	if len(amfCb.releaseCalls) != 1 {
		t.Fatalf("release commands = %d, want 1 for a session the home network no longer subscribes", len(amfCb.releaseCalls))
	}
}

func TestRoamerSessionsNeedTheAMFHPLMNTarget(t *testing.T) {
	notLBO := testTarget()
	notLBO.LBO = false

	for name, tc := range map[string]struct {
		target *smf.HPLMNTarget
		dnn    string
		want   fgs.GSMCause
	}{
		"no home target":   {dnn: testDNN, want: fgs.GSMCauseRequestRejectedUnspecified},
		"home routed only": {target: notLBO, dnn: testDNN, want: fgs.GSMCauseRequestedServiceOptionNotSubscribed},
	} {
		t.Run(name, func(t *testing.T) {
			home := &fakeHPLMNClient{rules: hplmnRules()}
			f := hplmnSMF(home)

			_, rsp, err := f.s.CreateSmContext(context.Background(), testSUPI(), 1, tc.dnn, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, tc.target)
			if err == nil || rsp == nil {
				t.Fatalf("CreateSmContext = %v, reject %v", err, rsp)
			}

			if got := fgs.GSMCause(rejectCauseCode(t, rsp)); got != tc.want {
				t.Fatalf("reject cause = %d, want %d", got, tc.want)
			}

			home.mu.Lock()
			opened := len(home.requests)
			home.mu.Unlock()

			if opened != 0 || f.s.SessionCount() != 0 {
				t.Fatal("a roamer's session went ahead without a usable home target")
			}
		})
	}
}

func TestHPLMNSessionAMBRCapsTheSubscribedQoS(t *testing.T) {
	rules := hplmnRules()
	rules.SessionAMBR = &models.Ambr{Uplink: models.MustParseBitRate("5 Mbps"), Downlink: models.MustParseBitRate("7 Mbps")}

	f := hplmnSMF(&fakeHPLMNClient{rules: rules})

	if _, _, err := f.s.CreateSmContext(context.Background(), testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget()); err != nil {
		t.Fatal(err)
	}

	f.pcf.mu.Lock()

	var got smf.SubscribedQoS
	for _, c := range f.pcf.associations {
		got = c.Subscribed
	}
	f.pcf.mu.Unlock()

	if got.SessionAMBR.Uplink.Bps() != 5_000_000 || got.SessionAMBR.Downlink.Bps() != 7_000_000 {
		t.Fatalf("subscribed Session-AMBR = %+v, want the home 5/7 Mbps cap", got.SessionAMBR)
	}
}

func TestReconcileReleasesASessionTheHomeNetworkWithdrew(t *testing.T) {
	home := &fakeHPLMNClient{rules: hplmnRules()}
	f := hplmnSMF(home)
	ctx := context.Background()

	if _, _, err := f.s.CreateSmContext(ctx, testSUPI(), 1, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest(), 0, testTarget()); err != nil {
		t.Fatal(err)
	}

	h := home.session(t)
	h.mu.Lock()
	h.rules.Withdrawn = true
	h.mu.Unlock()

	f.s.ReconcileHPLMNBoundSessions(ctx, testSUPI())

	f.amf.mu.Lock()
	defer f.amf.mu.Unlock()

	if len(f.amf.releaseCalls) != 1 {
		t.Fatalf("release commands = %d, want 1 for a session the home network withdrew", len(f.amf.releaseCalls))
	}
}
