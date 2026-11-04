// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehss"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/ueregistration"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

const (
	roamerIMSI       = "999700000000001"
	roamerPartnerID  = "partner-1"
	roamerProfileID  = "partner-profile"
	roamerHSSHost    = "hss.epc.mnc070.mcc999.3gppnetwork.org"
	roamerHPLMNRealm = "epc.mnc070.mcc999.3gppnetwork.org"
	roamerIMEISV     = "3506382143658701"
	roamerTACAndSNR  = "35063821436587"
	roamerSoftwareSV = "01"
)

type roamingPartners struct {
	partners []db.RoamingPartner
	count    int

	mu      sync.Mutex
	msisdns map[string][]string
}

func (r *roamingPartners) SetVisitorMSISDN(_ context.Context, imsi, msisdn string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.msisdns == nil {
		r.msisdns = map[string][]string{}
	}

	r.msisdns[imsi] = append(r.msisdns[imsi], msisdn)

	return nil
}

func (r *roamingPartners) storedMSISDNs(imsi string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.msisdns[imsi]...)
}

func (r *roamingPartners) ListRoamingPartners(context.Context) ([]db.RoamingPartner, error) {
	return r.partners, nil
}

func (r *roamingPartners) CountSubscribersAndVisitors(context.Context) (int, error) {
	return r.count, nil
}

type visitorBearerStore struct {
	fakeBearerStore
	allow4G bool

	mu       sync.Mutex
	visitors map[string]string
}

func (s *visitorBearerStore) GetSubscriberOrVisitor(ctx context.Context, imsi string) (*db.Subscriber, error) {
	s.mu.Lock()
	partner, ok := s.visitors[imsi]
	s.mu.Unlock()

	switch {
	case ok:
		return &db.Subscriber{Imsi: imsi, ProfileID: roamerProfileID, RoamingPartnerID: &partner}, nil
	case strings.HasPrefix(imsi, "00101"):
		return s.fakeBearerStore.GetSubscriberOrVisitor(ctx, imsi)
	default:
		return nil, db.ErrNotFound
	}
}

func (s *visitorBearerStore) GetProfileByID(ctx context.Context, id string) (*db.Profile, error) {
	if id == roamerProfileID {
		return &db.Profile{ID: id, UeAmbrDownlink: "1 Gbps", UeAmbrUplink: "1 Gbps", Allow4G: s.allow4G, Allow5G: true}, nil
	}

	return s.fakeBearerStore.GetProfileByID(ctx, id)
}

func (s *visitorBearerStore) visitor(imsi string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.visitors[imsi]

	return p, ok
}

type visitorRegistrar struct {
	store *visitorBearerStore

	mu               sync.Mutex
	ownRegistrations []string
	purged           []string
	withdraw         *ueregistration.Withdrawal
}

func (r *visitorRegistrar) Register(_ context.Context, imsi string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ownRegistrations = append(r.ownRegistrations, imsi)

	return 1, nil
}

func (r *visitorRegistrar) RegisterVisitor(_ context.Context, imsi, partnerID string) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	r.store.visitors[imsi] = partnerID

	return 1, nil
}

func (r *visitorRegistrar) Confirmed(context.Context, string, int64) bool { return true }

func (r *visitorRegistrar) Purge(imsi string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.purged = append(r.purged, imsi)
}

func (r *visitorRegistrar) Reconcile(ctx context.Context, _ string, _ func() int64, _ func(context.Context), withdraw func(context.Context, ueregistration.Withdrawal)) {
	r.mu.Lock()
	why := r.withdraw
	r.mu.Unlock()

	if why != nil {
		withdraw(ctx, *why)
	}
}

func (r *visitorRegistrar) purges() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.purged...)
}

type roamingMME struct {
	m        *mme.MME
	store    *visitorBearerStore
	partners *roamingPartners
	hss      *fakehss.Node
	reg      *visitorRegistrar
	sessions *fakeSessionManager
}

func roamerPartner() db.RoamingPartner {
	peer := "dea"

	return db.RoamingPartner{ID: roamerPartnerID, Name: "partner", ProfileID: roamerProfileID, DiameterPeerID: &peer, PLMNs: []db.RoamingPLMN{{Mcc: "999", Mnc: "70"}}}
}

func roamerSubscription() s6a.SubscriptionData {
	return fakehss.Subscription(1, fakehss.APN(1, "internet"), fakehss.APN(2, "ims"))
}

func newRoamingMME(t *testing.T) *roamingMME {
	t.Helper()

	store := &visitorBearerStore{allow4G: true, visitors: map[string]string{}}
	sessions := &fakeSessionManager{}
	hss := fakehss.New(
		diameter.Identity{OriginHost: "mmec01.mmegi8001.mme.epc.mnc001.mcc001.3gppnetwork.org", OriginRealm: "epc.mnc001.mcc001.3gppnetwork.org"},
		diameter.Identity{OriginHost: roamerHSSHost, OriginRealm: roamerHPLMNRealm},
	)
	hss.Provision(roamerIMSI, fakehss.Subscriber{
		K:            "465b5ce8b199b49faa5f0a2ee238a6bc",
		OPc:          "cd63cb71954a9f4e48a5994e37a02baf",
		SQN:          "000000000001",
		Subscription: roamerSubscription(),
	})

	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), store, sessions)
	m.NAS = &nasHandler{m: m}
	partners := &roamingPartners{partners: []db.RoamingPartner{roamerPartner()}}
	m.Roaming = partners
	m.HPLMN = roaming.NewS6a(func() roaming.DiameterNode { return hss })
	m.IMSVoice = fakeIMSVoice(true)
	m.SMS = &fakeSMSHandler{allowed: true}

	reg := &visitorRegistrar{store: store}
	m.Registrations = reg

	return &roamingMME{m: m, store: store, partners: partners, hss: hss, reg: reg, sessions: sessions}
}

type roamerAttach struct {
	ue      *mme.UeContext
	cc      *captureConn
	knasInt [16]byte
	knasEnc [16]byte
}

func (a *roamerAttach) last(t *testing.T) []byte {
	t.Helper()

	a.cc.mu.Lock()
	defer a.cc.mu.Unlock()

	if len(a.cc.sent) == 0 {
		t.Fatal("no downlink sent")
	}

	return a.cc.sent[len(a.cc.sent)-1]
}

func (a *roamerAttach) protected(t *testing.T, wire []byte) []byte {
	t.Helper()

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink,
		mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, a.knasInt, a.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	return plain
}

func attachRequestFor(t *testing.T, imsi string, attachType eps.AttachType, apn string) []byte {
	t.Helper()

	pdn := &eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}
	if apn != "" {
		pdn.AccessPointName = new(eps.APN(apn))
	}

	esm, err := pdn.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	attach, err := (&eps.AttachRequest{
		EPSAttachType:       attachType,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(imsi)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return attach
}

func (f *roamingMME) handle(conn *mme.UeConn, pdu []byte) {
	HandleNAS(context.Background(), f.m, conn, pdu)
	f.settle(conn)
}

func (f *roamingMME) settle(conn *mme.UeConn) {
	conn.SettleForTest()
	f.m.SettleHPLMNForTest()
}

func (f *roamingMME) startAttach(t *testing.T, imsi string, attachType eps.AttachType, apn string) *roamerAttach {
	t.Helper()

	cc := &captureConn{}
	ue := newAttachUe(f.m, cc, 7)

	f.handle(ue.Conn(), attachRequestFor(t, imsi, attachType, apn))

	return &roamerAttach{ue: ue, cc: cc}
}

func (f *roamingMME) respondToAuthentication(t *testing.T, a *roamerAttach) {
	t.Helper()

	authReq, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, a.last(t)))
	if err != nil {
		t.Fatalf("expected an Authentication Request: %v", err)
	}

	res := make([]byte, 8)
	if err := udm.F2345(testSubscriber.OPc[:], testSubscriber.K[:], authReq.RAND[:], res, make([]byte, 16), make([]byte, 16), make([]byte, 6), nil); err != nil {
		t.Fatal(err)
	}

	kasme := a.ue.Conn().AuthVector.KASME

	if a.knasEnc, err = epskeys.DeriveKNASEnc(kasme, 2); err != nil {
		t.Fatal(err)
	}

	if a.knasInt, err = epskeys.DeriveKNASInt(kasme, 2); err != nil {
		t.Fatal(err)
	}

	resp, err := (&eps.AuthenticationResponse{RES: res}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	f.handle(a.ue.Conn(), resp)
}

func (f *roamingMME) authenticate(t *testing.T, a *roamerAttach) {
	t.Helper()

	f.respondToAuthentication(t, a)

	if _, err := eps.ParseSecurityModeCommand(a.protected(t, decodeDownlinkNAS(t, a.last(t)))); err != nil {
		t.Fatalf("expected a Security Mode Command: %v", err)
	}

	imeisv := eps.MobileIMEISV(roamerIMEISV)

	complete, err := (&eps.SecurityModeComplete{IMEISV: &imeisv}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(complete, eps.SHTIntegrityProtectedCipheredNewContext, nas.MakeCount(0, 0), nas.DirectionUplink,
		mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, a.knasInt, a.knasEnc))
	if err != nil {
		t.Fatal(err)
	}

	f.handle(a.ue.Conn(), wire)
}

func (f *roamingMME) attach(t *testing.T, attachType eps.AttachType, apn string) *roamerAttach {
	t.Helper()

	a := f.startAttach(t, roamerIMSI, attachType, apn)
	f.authenticate(t, a)

	return a
}

func (a *roamerAttach) accept(t *testing.T) *eps.AttachAccept {
	t.Helper()

	ics := parseInitialContextSetup(t, a.last(t))

	accept, err := eps.ParseAttachAccept(a.protected(t, []byte(ics.ERABToBeSetup[0].NASPDU)))
	if err != nil {
		t.Fatal(err)
	}

	return accept
}

func (a *roamerAttach) reject(t *testing.T, secured bool) *eps.AttachReject {
	t.Helper()

	a.cc.mu.Lock()
	sent := append([][]byte(nil), a.cc.sent...)
	a.cc.mu.Unlock()

	if len(sent) < 2 {
		t.Fatalf("expected an Attach Reject and a UE Context Release Command, got %d downlinks", len(sent))
	}

	parseUEContextReleaseCommand(t, sent[len(sent)-1])

	nasPDU := decodeDownlinkNAS(t, sent[len(sent)-2])
	if secured {
		nasPDU = a.protected(t, nasPDU)
	}

	reject, err := eps.ParseAttachReject(nasPDU)
	if err != nil {
		t.Fatalf("expected an Attach Reject: %v", err)
	}

	return reject
}

func s6aRequests(t *testing.T, hss *fakehss.Node, command uint32) []*diameter.Message {
	t.Helper()

	var out []*diameter.Message

	for _, r := range hss.Requests() {
		if r.CommandCode == command {
			out = append(out, r)
		}
	}

	return out
}

func TestVisitorAttachBreaksOutLocally(t *testing.T) {
	f := newRoamingMME(t)
	a := f.attach(t, eps.AttachTypeEPS, "")

	accept := a.accept(t)

	if f.sessions.lastRequest.APN != "internet" || f.sessions.lastRequest.IMSI != roamerIMSI {
		t.Fatalf("session request %+v, want the home default APN internet", f.sessions.lastRequest)
	}

	if a.ue.Ambr == nil || a.ue.Ambr.Uplink.Bps() != 80_000_000 || a.ue.Ambr.Downlink.Bps() != 300_000_000 {
		t.Fatalf("UE-AMBR = %+v, want min(home 80/300 Mbps, partner profile 1/1 Gbps)", a.ue.Ambr)
	}

	if accept.NetworkFeatureSupport == nil || accept.NetworkFeatureSupport.IMSVoPS {
		t.Fatalf("Attach Accept network feature support %+v, want IMS VoPS not supported for a roamer", accept.NetworkFeatureSupport)
	}

	if partner, ok := f.store.visitor(roamerIMSI); !ok || partner != roamerPartnerID {
		t.Fatalf("visitor row = %q, %v, want one for %s", partner, ok, roamerPartnerID)
	}

	if len(f.reg.ownRegistrations) != 0 {
		t.Fatalf("a roamer was registered as a home subscriber: %v", f.reg.ownRegistrations)
	}

	if a.ue.FiveGSInterworkingAllowed() {
		t.Fatal("a roamer was offered 5GS interworking")
	}

	if sub, ok := f.m.HPLMNSubscription(roamerIMSI); !ok || sub.APNConfigurationProfile == nil {
		t.Fatal("the home subscription is not cached on the UE context")
	}

	airs := s6aRequests(t, f.hss, s6a.CommandAuthenticationInformation)
	if len(airs) != 1 {
		t.Fatalf("AIRs = %d, want 1", len(airs))
	}

	if env := tgpp.ParseEnvelope(airs[0]); env.DestinationRealm != roamerHPLMNRealm || env.DestinationHost != "" {
		t.Fatalf("AIR envelope %+v, want the home realm and no Destination-Host", env)
	}

	ulrs := s6aRequests(t, f.hss, s6a.CommandUpdateLocation)
	if len(ulrs) != 1 {
		t.Fatalf("ULRs = %d, want 1", len(ulrs))
	}

	if env := tgpp.ParseEnvelope(ulrs[0]); env.DestinationRealm != roamerHPLMNRealm || env.DestinationHost != roamerHSSHost {
		t.Fatalf("ULR envelope %+v, want the HSS learned from the AIA", env)
	}

	ulr, err := s6a.ParseUpdateLocationRequest(ulrs[0])
	if err != nil {
		t.Fatal(err)
	}

	switch {
	case ulr.Flags != s6a.ULRFlagS6aS6dIndicator|s6a.ULRFlagInitialAttachIndicator:
		t.Fatalf("ULR-Flags = %s, want S6a/S6d-Indicator and Initial-Attach-Indicator only", ulr.Flags)
	case ulr.RATType != tgpp.RATEUTRAN || ulr.VisitedPLMN != (tgpp.PLMN{MCC: "001", MNC: "01"}):
		t.Fatalf("ULR RAT %s, visited PLMN %+v", ulr.RATType, ulr.VisitedPLMN)
	case ulr.Terminal == nil || ulr.Terminal.IMEI != roamerTACAndSNR || ulr.Terminal.SoftwareVersion != roamerSoftwareSV:
		t.Fatalf("Terminal-Information = %+v, want IMEI %s and SV %s", ulr.Terminal, roamerTACAndSNR, roamerSoftwareSV)
	case ulr.Features != roaming.ULRFeatures:
		t.Fatalf("Supported-Features = %+v, want the ODB APN features", ulr.Features)
	case ulr.HomogeneousIMSVoiceOverPS == nil || *ulr.HomogeneousIMSVoiceOverPS != s6a.IMSVoiceOverPSNotSupported:
		t.Fatal("Homogeneous-Support-of-IMS-Voice-Over-PS-Sessions is not NOT_SUPPORTED")
	case ulr.MMENumberForMTSMS != "" || ulr.SMSRegisterRequest != nil:
		t.Fatal("the ULR requests SMS in the MME")
	}
}

func TestVisitorCombinedAttachIsEPSOnly(t *testing.T) {
	f := newRoamingMME(t)
	a := f.attach(t, eps.AttachTypeCombined, "")

	accept := a.accept(t)

	if accept.EPSAttachResult != eps.AttachResultEPS || accept.Cause == nil || *accept.Cause != eps.EMMCauseCSDomainNotAvailable {
		t.Fatalf("combined attach result %d cause %v, want EPS only with #18", accept.EPSAttachResult, accept.Cause)
	}
}

func TestVisitorAttachRequestsTheSubscribedAPN(t *testing.T) {
	f := newRoamingMME(t)
	a := f.attach(t, eps.AttachTypeEPS, "internet")

	a.accept(t)

	if f.sessions.lastRequest.APN != "internet" {
		t.Fatalf("APN = %q, want internet", f.sessions.lastRequest.APN)
	}
}

func TestVisitorDefaultPDNRules(t *testing.T) {
	for _, tc := range []struct {
		name   string
		apn    string
		modify func(*s6a.SubscriptionData)
		want   eps.ESMCause
	}{
		{name: "APN not subscribed", apn: "corporate", want: eps.ESMCauseServiceOptionNotSubscribed},
		{name: "IMS APN", apn: "ims", want: eps.ESMCauseServiceOptionNotSubscribed},
		{name: "VPLMN dynamic address not allowed", modify: func(s *s6a.SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].VPLMNDynamicAddressAllowed = false
		}, want: eps.ESMCauseServiceOptionNotSubscribed},
		{name: "static PGW", modify: func(s *s6a.SubscriptionData) {
			c := &s.APNConfigurationProfile.Configurations[0]
			c.PDNGateway = &s6a.MIP6AgentInfo{HomeAgentHost: "pgw.epc.mnc070.mcc999.3gppnetwork.org", HomeAgentRealm: roamerHPLMNRealm}
		}, want: eps.ESMCauseServiceOptionNotSubscribed},
		{name: "home APN that is not a local data network", modify: func(s *s6a.SubscriptionData) {
			s.APNConfigurationProfile.Configurations[0].ServiceSelection = "corporate"
		}, want: eps.ESMCauseMissingOrUnknownAPN},
		{name: "all packet oriented services barred", modify: func(s *s6a.SubscriptionData) {
			s.SubscriberStatus = new(s6a.SubscriberStatusOperatorDeterminedBarring)
			s.OperatorDeterminedBarring = new(s6a.ODBAllPacketOrientedServices)
		}, want: eps.ESMCauseOperatorDeterminedBarring},
		{name: "roamer access to VPLMN-AP barred", modify: func(s *s6a.SubscriptionData) {
			s.SubscriberStatus = new(s6a.SubscriberStatusOperatorDeterminedBarring)
			s.OperatorDeterminedBarring = new(s6a.ODBRoamerAccessVPLMNAP)
		}, want: eps.ESMCauseOperatorDeterminedBarring},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)

			if tc.modify != nil {
				f.hss.RewriteULA(func(u s6a.UpdateLocation) s6a.UpdateLocation {
					tc.modify(u.Subscription)
					return u
				})
			}

			a := f.attach(t, eps.AttachTypeEPS, tc.apn)

			reject := a.reject(t, true)
			if reject.Cause != eps.EMMCauseESMFailure {
				t.Fatalf("Attach Reject cause %s, want #19", reject.Cause)
			}

			pdn, err := eps.ParsePDNConnectivityReject(reject.ESMMessageContainer)
			if err != nil {
				t.Fatalf("expected a PDN Connectivity Reject inside the Attach Reject: %v", err)
			}

			if pdn.Cause != tc.want {
				t.Fatalf("ESM cause %s, want %s", pdn.Cause, tc.want)
			}

			if f.sessions.lastRequest.IMSI != "" {
				t.Fatal("a session was requested for an APN that cannot be broken out")
			}
		})
	}
}

func TestVisitorAttachIgnoresBarringThatSparesLocalBreakout(t *testing.T) {
	for name, odb := range map[string]s6a.OperatorDeterminedBarring{
		"roamer access to HPLMN-AP barred": s6a.ODBRoamerAccessHPLMNAP,
		"outgoing calls barred":            s6a.ODBAllOutgoingCalls,
	} {
		t.Run(name, func(t *testing.T) {
			f := newRoamingMME(t)
			f.hss.RewriteULA(func(u s6a.UpdateLocation) s6a.UpdateLocation {
				u.Subscription.SubscriberStatus = new(s6a.SubscriberStatusOperatorDeterminedBarring)
				u.Subscription.OperatorDeterminedBarring = &odb

				return u
			})

			f.attach(t, eps.AttachTypeEPS, "").accept(t)
		})
	}
}

func TestVisitorAdditionalPDNForTheIMSAPNIsRefused(t *testing.T) {
	f := newRoamingMME(t)
	a := f.attach(t, eps.AttachTypeEPS, "")
	a.accept(t)

	a.ue.ForceStateForTest(mme.EMMRegistered)

	req := &eps.PDNConnectivityRequest{PTI: 2, RequestType: 1, PDNType: 1, AccessPointName: new(eps.APN("ims"))}

	handlePDNConnectivityRequest(context.Background(), f.m, a.ue, a.ue.Conn(), req)

	reject, err := eps.ParsePDNConnectivityReject(a.protected(t, decodeDownlinkNAS(t, a.last(t))))
	if err != nil {
		t.Fatalf("expected a PDN Connectivity Reject: %v", err)
	}

	if reject.Cause != eps.ESMCauseServiceOptionNotSubscribed {
		t.Fatalf("ESM cause %s, want #33", reject.Cause)
	}
}

func TestVisitorUpdateLocationResultMapping(t *testing.T) {
	diag := func(d s6a.ErrorDiagnostic) *s6a.ErrorDiagnostic { return &d }
	experimental := func(code uint32, d *s6a.ErrorDiagnostic) error {
		return &s6a.ResultError{Result: tgpp.Experimental(code), ErrorDiagnostic: d}
	}
	base := func(code uint32) error { return &s6a.ResultError{Result: tgpp.Result{Code: code}} }

	for _, tc := range []struct {
		name     string
		err      error
		modify   func(s6a.UpdateLocation) s6a.UpdateLocation
		down     bool
		want     eps.EMMCause
		extended bool
	}{
		{name: "USER_UNKNOWN", err: experimental(tgpp.ResultErrorUserUnknown, nil), want: eps.EMMCauseEPSAndNonEPSServicesNotAllowed},
		{name: "ROAMING_NOT_ALLOWED", err: experimental(tgpp.ResultErrorRoamingNotAllowed, nil), want: eps.EMMCausePLMNNotAllowed},
		{name: "ROAMING_NOT_ALLOWED ODB_HPLMN_APN", err: experimental(tgpp.ResultErrorRoamingNotAllowed, diag(s6a.ErrorDiagnosticODBHPLMNAPN)), want: eps.EMMCauseEPSServicesNotAllowedInThisPLMN},
		{name: "ROAMING_NOT_ALLOWED ODB_VPLMN_APN", err: experimental(tgpp.ResultErrorRoamingNotAllowed, diag(s6a.ErrorDiagnosticODBVPLMNAPN)), want: eps.EMMCauseEPSServicesNotAllowedInThisPLMN},
		{name: "ROAMING_NOT_ALLOWED ODB_ALL_APN", err: experimental(tgpp.ResultErrorRoamingNotAllowed, diag(s6a.ErrorDiagnosticODBAllAPN)), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "UNKNOWN_EPS_SUBSCRIPTION", err: experimental(tgpp.ResultErrorUnknownEPSSubscription, nil), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "UNKNOWN_EPS_SUBSCRIPTION GPRS_DATA_SUBSCRIBED", err: experimental(tgpp.ResultErrorUnknownEPSSubscription, diag(s6a.ErrorDiagnosticGPRSDataSubscribed)), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "UNKNOWN_EPS_SUBSCRIPTION NO_GPRS_DATA_SUBSCRIBED", err: experimental(tgpp.ResultErrorUnknownEPSSubscription, diag(s6a.ErrorDiagnosticNoGPRSDataSubscribed)), want: eps.EMMCauseEPSServicesNotAllowed},
		{name: "RAT_NOT_ALLOWED", err: experimental(tgpp.ResultErrorRATNotAllowed, nil), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "UNABLE_TO_DELIVER", err: base(diameter.ResultUnableToDeliver), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "REALM_NOT_SERVED", err: base(diameter.ResultRealmNotServed), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "AUTHORIZATION_REJECTED", err: base(diameter.ResultAuthorizationRejected), want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "UNABLE_TO_COMPLY", err: base(diameter.ResultUnableToComply), want: eps.EMMCauseNetworkFailure},
		{name: "MISSING_AVP", err: base(diameter.ResultMissingAVP), want: eps.EMMCauseNetworkFailure},
		{name: "AUTHENTICATION_DATA_UNAVAILABLE", err: experimental(tgpp.ResultAuthenticationDataUnavailable, nil), want: eps.EMMCauseNetworkFailure},
		{name: "HSS unreachable", down: true, want: eps.EMMCauseProtocolErrorUnspecified},
		{name: "no subscription data", modify: func(u s6a.UpdateLocation) s6a.UpdateLocation {
			u.Subscription = nil
			return u
		}, want: eps.EMMCauseNetworkFailure},
		{name: "roaming restricted due to unsupported feature", modify: func(u s6a.UpdateLocation) s6a.UpdateLocation {
			u.Subscription.RoamingRestrictedDueToUnsupportedFeature = true
			return u
		}, want: eps.EMMCauseEPSServicesNotAllowedInThisPLMN},
		{name: "WB-E-UTRAN not allowed", modify: func(u s6a.UpdateLocation) s6a.UpdateLocation {
			u.Subscription.AccessRestrictionData = new(s6a.ARDWBEUTRANNotAllowed)
			return u
		}, want: eps.EMMCauseNoSuitableCellsInTrackingArea, extended: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			f.hss.FailULR(tc.err)

			if tc.modify != nil {
				f.hss.RewriteULA(tc.modify)
			}

			a := f.startAttach(t, roamerIMSI, eps.AttachTypeEPS, "")

			if tc.down {
				f.respondToAuthentication(t, a)
				f.hss.SetDown(true)
				f.completeSecurityMode(t, a)
			} else {
				f.authenticate(t, a)
			}

			reject := a.reject(t, true)
			if reject.Cause != tc.want {
				t.Fatalf("Attach Reject cause %s, want %s", reject.Cause, tc.want)
			}

			if got := reject.ExtendedEMMCause != nil && reject.ExtendedEMMCause.EUTRANNotAllowed; got != tc.extended {
				t.Fatalf("Extended EMM cause E-UTRAN not allowed = %v, want %v", got, tc.extended)
			}

			if _, ok := f.store.visitor(roamerIMSI); ok {
				t.Fatal("a rejected roamer got a visitor row")
			}
		})
	}
}

func (f *roamingMME) completeSecurityMode(t *testing.T, a *roamerAttach) {
	t.Helper()

	imeisv := eps.MobileIMEISV(roamerIMEISV)

	complete, err := (&eps.SecurityModeComplete{IMEISV: &imeisv}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(complete, eps.SHTIntegrityProtectedCipheredNewContext, nas.MakeCount(0, 0), nas.DirectionUplink,
		mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, a.knasInt, a.knasEnc))
	if err != nil {
		t.Fatal(err)
	}

	f.handle(a.ue.Conn(), wire)
}

func TestVisitorAuthenticationInformationResultMapping(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		down bool
		want eps.EMMCause
	}{
		{name: "USER_UNKNOWN", err: &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUserUnknown)}, want: eps.EMMCauseEPSAndNonEPSServicesNotAllowed},
		{name: "UNKNOWN_EPS_SUBSCRIPTION", err: &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorUnknownEPSSubscription)}, want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "AUTHENTICATION_DATA_UNAVAILABLE", err: &s6a.ResultError{Result: tgpp.Experimental(tgpp.ResultAuthenticationDataUnavailable)}, want: eps.EMMCauseNetworkFailure},
		{name: "UNABLE_TO_DELIVER", err: &s6a.ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToDeliver}}, want: eps.EMMCauseNoSuitableCellsInTrackingArea},
		{name: "HSS unreachable", down: true, want: eps.EMMCauseProtocolErrorUnspecified},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			f.hss.FailAIR(tc.err)
			f.hss.SetDown(tc.down)

			a := f.startAttach(t, roamerIMSI, eps.AttachTypeEPS, "")

			if reject := a.reject(t, false); reject.Cause != tc.want {
				t.Fatalf("Attach Reject cause %s, want %s", reject.Cause, tc.want)
			}
		})
	}
}

func TestVisitorResynchronisationGoesToTheHomeHSS(t *testing.T) {
	f := newRoamingMME(t)
	a := f.startAttach(t, roamerIMSI, eps.AttachTypeEPS, "")

	first := a.ue.Conn().AuthVector.RAND
	auts := autsFor(t, a.ue, []byte{0, 0, 0, 0, 0x10, 0})

	f.handle(a.ue.Conn(), authFailureWire(t, eps.EMMCauseSynchFailure, auts))

	airs := s6aRequests(t, f.hss, s6a.CommandAuthenticationInformation)
	if len(airs) != 2 {
		t.Fatalf("AIRs = %d, want a second one for the resynchronisation", len(airs))
	}

	air, err := s6a.ParseAuthenticationInformationRequest(airs[1])
	if err != nil {
		t.Fatal(err)
	}

	if r := air.EUTRAN.Resync; r == nil || string(r.RAND) != string(first[:]) || string(r.AUTS) != string(auts) {
		t.Fatalf("Re-Synchronization-Info = %+v, want the first RAND and the UE's AUTS", r)
	}

	if env := tgpp.ParseEnvelope(airs[1]); env.DestinationHost != roamerHSSHost {
		t.Fatalf("resynchronisation AIR Destination-Host = %q, want the HSS learned from the first AIA", env.DestinationHost)
	}

	f.authenticate(t, a)
	a.accept(t)
}

func TestVisitorPartnerGate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		imsi     string
		setup    func(*roamingMME)
		want     eps.EMMCause
		extended bool
		backOff  bool
	}{
		{name: "no roaming partner", imsi: "998010000000001", want: eps.EMMCausePLMNNotAllowed},
		{name: "partner without a 4G agreement", imsi: roamerIMSI, setup: func(f *roamingMME) {
			f.partners.partners[0].DiameterPeerID = nil
		}, want: eps.EMMCauseNoSuitableCellsInTrackingArea, extended: true},
		{name: "partner profile without 4G", imsi: roamerIMSI, setup: func(f *roamingMME) {
			f.store.allow4G = false
		}, want: eps.EMMCauseNoSuitableCellsInTrackingArea, extended: true},
		{name: "maximum number of subscribers", imsi: roamerIMSI, setup: func(f *roamingMME) {
			f.partners.count = db.MaxNumSubscribers
		}, want: eps.EMMCauseCongestion, backOff: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingMME(t)
			if tc.setup != nil {
				tc.setup(f)
			}

			a := f.startAttach(t, tc.imsi, eps.AttachTypeEPS, "")

			reject := a.reject(t, false)
			if reject.Cause != tc.want {
				t.Fatalf("Attach Reject cause %s, want %s", reject.Cause, tc.want)
			}

			if got := reject.ExtendedEMMCause != nil && reject.ExtendedEMMCause.EUTRANNotAllowed; got != tc.extended {
				t.Fatalf("Extended EMM cause E-UTRAN not allowed = %v, want %v", got, tc.extended)
			}

			if tc.backOff {
				if reject.T3346 == nil {
					t.Fatal("Attach Reject #22 without T3346")
				}

				if d, ok := reject.T3346.Duration(); !ok || d != mme.T3346CapacityBackoff {
					t.Fatalf("T3346 = %v, want %v", d, mme.T3346CapacityBackoff)
				}
			}

			if len(f.hss.Requests()) != 0 {
				t.Fatal("a refused roamer reached the home HSS")
			}
		})
	}
}

func TestVisitorAlreadyHoldingARowIsNotCountedAgain(t *testing.T) {
	f := newRoamingMME(t)
	f.partners.count = db.MaxNumSubscribers
	f.store.visitors[roamerIMSI] = roamerPartnerID

	a := f.attach(t, eps.AttachTypeEPS, "")
	a.accept(t)
}

func TestOwnSubscriberAttachIgnoresRoaming(t *testing.T) {
	f := newRoamingMME(t)
	a := f.startAttach(t, testSubscriber.IMSI, eps.AttachTypeEPS, "")

	if _, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, a.last(t))); err != nil {
		t.Fatalf("expected an Authentication Request from the local HSS: %v", err)
	}

	if len(f.hss.Requests()) != 0 {
		t.Fatal("a home subscriber was sent to a roaming partner's HSS")
	}

	if a.ue.IsVisitor() {
		t.Fatal("a home subscriber was marked as a visitor")
	}
}

func TestVisitorRemovalPurgesTheRegistration(t *testing.T) {
	f := newRoamingMME(t)
	a := f.attach(t, eps.AttachTypeEPS, "")
	a.accept(t)

	f.m.RemoveUe(context.Background(), a.ue)
	f.m.SettleHPLMNForTest()

	f.reg.mu.Lock()
	defer f.reg.mu.Unlock()

	if len(f.reg.purged) != 1 || f.reg.purged[0] != roamerIMSI {
		t.Fatalf("purged = %v, want the roamer's registration purged when its context goes", f.reg.purged)
	}
}
