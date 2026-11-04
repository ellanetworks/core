// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/internal/roaming/fakehplmn"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas/fgs"
)

const (
	roamerIMSI = "001010000000001"
	roamerSUCI = "suci-0-001-01-0000-0-0-0000000001"
	roamerK    = "465b5ce8b199b49faa5f0a2ee238a6bc"
	roamerOPc  = "cd63cb71954a9f4e48a5994e37a02baf"
)

type roamingTestDB struct {
	mu        sync.Mutex
	partners  []db.RoamingPartner
	visitors  map[string]*db.Subscriber
	allow5G   bool
	count     int
	sliceSD   *string
	profileUp string
}

func newRoamingTestDB() *roamingTestDB {
	seppFQDN := "sepp1.5gc.mnc001.mcc001.3gppnetwork.org"

	return &roamingTestDB{
		partners: []db.RoamingPartner{{
			ID: "partner-1", Name: "home", ProfileID: "roamers", SEPPFQDN: &seppFQDN,
			PLMNs: []db.RoamingPLMN{{Mcc: "001", Mnc: "01"}},
		}},
		visitors:  map[string]*db.Subscriber{},
		allow5G:   true,
		profileUp: "1 Gbps",
	}
}

func (d *roamingTestDB) GetOperator(context.Context) (*db.Operator, error) {
	return &db.Operator{Mcc: "999", Mnc: "70", SupportedTACs: `["000001"]`, AmfRegionID: 1, AmfSetID: 1}, nil
}

func (d *roamingTestDB) GetSubscriberOrVisitor(_ context.Context, imsi string) (*db.Subscriber, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if s, ok := d.visitors[imsi]; ok {
		return s, nil
	}

	return nil, db.ErrNotFound
}

func (d *roamingTestDB) GetProfileByID(_ context.Context, id string) (*db.Profile, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return &db.Profile{ID: id, UeAmbrUplink: d.profileUp, UeAmbrDownlink: d.profileUp, Allow5G: d.allow5G, Allow4G: true}, nil
}

func (d *roamingTestDB) ListPoliciesByProfile(context.Context, string) ([]db.Policy, error) {
	return []db.Policy{{ID: "pol", ProfileID: "roamers", SliceID: "slice", DataNetworkID: "dn", Var5qi: 9, Arp: 1, SessionAmbrUplink: "100 Mbps", SessionAmbrDownlink: "100 Mbps", IsDefault: true}}, nil
}

func (d *roamingTestDB) ListNetworkSlicesByIDs(context.Context, []string) ([]db.NetworkSlice, error) {
	return []db.NetworkSlice{{ID: "slice", Sst: 1, Sd: d.sliceSD}}, nil
}

func (d *roamingTestDB) GetDataNetworkByID(context.Context, string) (*db.DataNetwork, error) {
	return &db.DataNetwork{ID: "dn", Name: "internet"}, nil
}

func (d *roamingTestDB) GetNetworkSliceByID(context.Context, string) (*db.NetworkSlice, error) {
	return &db.NetworkSlice{ID: "slice", Sst: 1}, nil
}

func (d *roamingTestDB) ListAllNetworkSlices(context.Context) ([]db.NetworkSlice, error) {
	return []db.NetworkSlice{{ID: "slice", Sst: 1}}, nil
}

func (d *roamingTestDB) AMFPointer() int { return 1 }

func (d *roamingTestDB) ListRoamingPartners(context.Context) ([]db.RoamingPartner, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]db.RoamingPartner(nil), d.partners...), nil
}

func (d *roamingTestDB) SetVisitorMSISDN(_ context.Context, imsi, msisdn string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	s, ok := d.visitors[imsi]
	if !ok {
		return db.ErrNotFound
	}

	s.Msisdn = msisdn

	return nil
}

func (d *roamingTestDB) CountSubscribersAndVisitors(context.Context) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.count, nil
}

type directN32 struct {
	h http.Handler
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func (d directN32) Client(string) *http.Client {
	return &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		d.h.ServeHTTP(rec, r)

		return rec.Result(), nil
	})}
}

func (d directN32) HPLMNNRF(_ string, home wire.PlmnID) (string, error) {
	return "http://" + home.NFFQDN(wire.FQDNLabelNRF), nil
}

type openN32 struct{ err error }

func (o openN32) WaitEstablished(context.Context, string, time.Duration) error { return o.err }

type roamingFixture struct {
	amf  *AMF
	db   *roamingTestDB
	home *fakehplmn.HPLMN
	reg  *fakeRegistrar
}

func newRoamingFixture(t *testing.T) *roamingFixture {
	t.Helper()

	f := &roamingFixture{db: newRoamingTestDB(), home: fakehplmn.New(wire.PlmnID{Mcc: "001", Mnc: "01"})}
	f.home.Provision(roamerIMSI, fakehplmn.Subscriber{
		K: roamerK, OPc: roamerOPc, SQN: "000000000020",
		AM: wire.AccessAndMobilitySubscriptionData{
			SubscribedUeAmbr: &wire.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"},
			Nssai:            &wire.Nssai{DefaultSingleNssais: []wire.Snssai{wire.NewSnssai(1, "")}},
		},
		SMFSel: &wire.SmfSelectionSubscriptionData{SubscribedSnssaiInfos: map[string]wire.SnssaiInfo{
			"1": {DnnInfos: []wire.DnnInfo{{Dnn: "internet", DefaultDnnIndicator: true, LboRoamingAllowed: true}}},
		}},
	})

	f.amf = New(f.db, nil, nil)
	f.amf.Roaming = f.db
	f.amf.N32 = openN32{}
	f.amf.HPLMN = roaming.NewAMFHPLMN(roaming.NewHPLMNSBI(directN32{h: f.home}, roaming.NewDiscovery(directN32{h: f.home})))
	f.reg = newFakeRegistrar()
	f.amf.Registrations = f.reg

	return f
}

func (f *roamingFixture) visitorUE(t *testing.T) (*UeContext, *UeConn) {
	t.Helper()

	ue := NewUeContext()
	ue.mu.Lock()
	ue.suci = roamerSUCI
	ue.mu.Unlock()

	conn := connectTestUE(t, f.amf, ue)

	if err := f.amf.AdmitVisitor(t.Context(), ue); err != nil {
		t.Fatalf("AdmitVisitor: %v", err)
	}

	return ue, conn
}

func waitDone[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	select {
	case v := <-ch:
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("the home network answer never came")
	}

	var zero T

	return zero
}

func hplmnRESStar(t *testing.T, result *ausf.AuthResult) [16]byte {
	t.Helper()

	k, _ := hex.DecodeString(roamerK)
	opc, _ := hex.DecodeString(roamerOPc)
	rand, _ := hex.DecodeString(result.Rand)
	res, ck, ik, ak, akstar := make([]byte, 8), make([]byte, 16), make([]byte, 16), make([]byte, 6), make([]byte, 6)

	if err := udm.F2345(opc, k, rand, res, ck, ik, ak, akstar); err != nil {
		t.Fatal(err)
	}

	out, err := udm.DeriveXresStar(ck, ik, "5G:mnc070.mcc999.3gppnetwork.org", rand, res)
	if err != nil {
		t.Fatal(err)
	}

	return [16]byte(out)
}

type confirmation struct {
	supi  etsi.SUPI
	kseaf []byte
	err   error
}

func (f *roamingFixture) authenticate(t *testing.T, ue *UeContext, conn *UeConn) {
	t.Helper()

	challenges := make(chan *ausf.AuthResult, 1)

	f.amf.RequestAuthentication(t.Context(), ue, conn, nil, func(_ context.Context, r *ausf.AuthResult, err error) {
		if err != nil {
			t.Errorf("RequestAuthentication: %v", err)
		}

		challenges <- r
	})

	challenge := waitDone(t, challenges)
	conn.AuthenticationCtx = challenge

	confirmed := make(chan confirmation, 1)

	f.amf.RequestConfirmation(t.Context(), ue, conn, hplmnRESStar(t, challenge), func(_ context.Context, supi etsi.SUPI, kseaf []byte, err error) {
		confirmed <- confirmation{supi, kseaf, err}
	})

	c := waitDone(t, confirmed)
	if c.err != nil || c.supi.IMSI() != roamerIMSI || len(c.kseaf) != 32 {
		t.Fatalf("confirmation %+v", c)
	}

	ue.SetSupi(c.supi)

	if err := f.amf.CommitUEIdentity(t.Context(), ue, MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatal(err)
	}
}

func (f *roamingFixture) registerAtHPLMN(t *testing.T, ue *UeContext, conn *UeConn) {
	t.Helper()

	done := make(chan error, 1)

	f.amf.RequestHPLMNRegistration(t.Context(), ue, conn, true, func(_ context.Context, err error) { done <- err })

	if err := waitDone(t, done); err != nil {
		t.Fatalf("RequestHPLMNRegistration: %v", err)
	}

	partner := "partner-1"

	f.db.mu.Lock()
	f.db.visitors[roamerIMSI] = &db.Subscriber{Imsi: roamerIMSI, ProfileID: "roamers", RoamingPartnerID: &partner}
	f.db.mu.Unlock()
}

func TestAdmitVisitorGatesOnThePartner(t *testing.T) {
	cases := []struct {
		name    string
		suci    string
		setup   func(d *roamingTestDB)
		cause   fgs.GMMCause
		backoff time.Duration
		visitor bool
	}{
		{name: "home subscriber", suci: "suci-0-999-70-0000-0-0-0000000001"},
		{name: "partner", suci: roamerSUCI, visitor: true},
		{name: "no partner", suci: "suci-0-002-02-0000-0-0-0000000001", cause: fgs.GMMCausePLMNNotAllowed},
		{name: "no 5G agreement", suci: roamerSUCI, setup: func(d *roamingTestDB) { d.partners[0].SEPPFQDN = nil }, cause: fgs.GMMCauseN1ModeNotAllowed},
		{name: "5G not allowed by the partner profile", suci: roamerSUCI, setup: func(d *roamingTestDB) { d.allow5G = false }, cause: fgs.GMMCauseN1ModeNotAllowed},
		{name: "agreement withdrawn", suci: roamerSUCI, setup: func(d *roamingTestDB) {
			d.partners[0].Withdrawals = []db.RoamingWithdrawal{{PartnerID: "partner-1", ExpiresAt: time.Now().Add(time.Hour).Unix()}}
		}, cause: fgs.GMMCausePLMNNotAllowed},
		{name: "5G agreement withdrawn", suci: roamerSUCI, setup: func(d *roamingTestDB) {
			d.partners[0].Withdrawals = []db.RoamingWithdrawal{{PartnerID: "partner-1", Type: db.UERegistrationTypeAMF3GPPAccess, ExpiresAt: time.Now().Add(time.Hour).Unix()}}
		}, cause: fgs.GMMCauseN1ModeNotAllowed},
		{name: "subscriber cap", suci: "", setup: func(d *roamingTestDB) { d.count = db.MaxNumSubscribers }, cause: fgs.GMMCauseCongestion, backoff: T3346CapacityBackoff},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRoamingFixture(t)
			if tc.setup != nil {
				tc.setup(f.db)
			}

			ue := NewUeContext()
			ue.suci = tc.suci

			if tc.suci == "" {
				supi, _ := etsi.NewSUPIFromIMSI(roamerIMSI)
				ue.supi = supi
			}

			err := f.amf.AdmitVisitor(t.Context(), ue)

			var r *Rejection

			switch {
			case tc.cause == 0 && err != nil:
				t.Fatalf("AdmitVisitor: %v", err)
			case tc.cause != 0 && (!errors.As(err, &r) || r.Cause != tc.cause || r.BackOff != tc.backoff):
				t.Fatalf("AdmitVisitor = %v, want cause %s with back-off %s", err, tc.cause, tc.backoff)
			}

			if got := ue.IsVisitor(); got != tc.visitor {
				t.Fatalf("visitor = %v, want %v", got, tc.visitor)
			}
		})
	}
}

func TestVisitorAuthenticatesRegistersAndIsPurgedAtItsHome(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)

	f.authenticate(t, ue, conn)
	f.registerAtHPLMN(t, ue, conn)

	supi := "imsi-" + roamerIMSI

	reg, ok := f.home.Registration(supi)
	if !ok {
		t.Fatal("no UECM registration at the home UDM")
	}

	if reg.DeregCallbackURI != "https://amf1.5gc.mnc070.mcc999.3gppnetwork.org/roaming-callbacks/v1/"+supi+"/uecm-dereg" ||
		reg.ImsVoPs != wire.ImsVoPsHomogeneousNonSupport || reg.RatType != wire.RatNR || reg.InitialRegistrationInd == nil || !*reg.InitialRegistrationInd {
		t.Fatalf("UECM registration %+v", reg)
	}

	if len(f.home.Subscriptions()) != 1 {
		t.Fatalf("SDM subscriptions %+v", f.home.Subscriptions())
	}

	if err := f.amf.RegisterUE(t.Context(), ue); err != nil {
		t.Fatal(err)
	}

	profile, err := f.amf.SubscriberProfile(t.Context(), ue.Supi())
	if err != nil {
		t.Fatalf("SubscriberProfile: %v", err)
	}

	want := models.Ambr{Uplink: models.BitRateFromBps(100_000_000), Downlink: models.BitRateFromBps(200_000_000)}
	if *profile.Ambr != want || profile.EPSInterworking || len(profile.AllowedNssai) != 1 {
		t.Fatalf("merged profile %+v (AMBR %+v)", profile, *profile.Ambr)
	}

	if dnn, err := f.amf.SubscriberDnn(t.Context(), ue.Supi(), &models.Snssai{Sst: 1}); err != nil || dnn != "internet" {
		t.Fatalf("default DNN %q, %v", dnn, err)
	}

	f.amf.DeregisterAndRemoveUeContext(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))
	f.amf.SettleHPLMN()

	if !f.home.Purged(supi) {
		t.Fatal("the visitor was not purged at the home UDM")
	}

	if len(f.home.Subscriptions()) != 0 {
		t.Fatal("the SDM subscription was not removed")
	}

	if len(f.reg.purged) != 1 || f.reg.purged[0] != roamerIMSI {
		t.Fatalf("local registration purges %v", f.reg.purged)
	}
}

func TestVisitorFailedAuthenticationSendsANullRESStar(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)

	challenges := make(chan *ausf.AuthResult, 1)

	f.amf.RequestAuthentication(t.Context(), ue, conn, nil, func(_ context.Context, r *ausf.AuthResult, _ error) { challenges <- r })
	waitDone(t, challenges)

	f.amf.AuthenticationFailed(t.Context(), ue)
	f.amf.SettleHPLMN()

	got := f.home.Confirmations()
	if len(got) != 1 || got[0] != nil {
		t.Fatalf("confirmations %v, want one null RES*", got)
	}
}

func TestVisitorWrongRESStarIsIllegalUE(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)

	challenges := make(chan *ausf.AuthResult, 1)

	f.amf.RequestAuthentication(t.Context(), ue, conn, nil, func(_ context.Context, r *ausf.AuthResult, _ error) { challenges <- r })
	waitDone(t, challenges)

	confirmed := make(chan error, 1)

	f.amf.RequestConfirmation(t.Context(), ue, conn, [16]byte{1}, func(_ context.Context, _ etsi.SUPI, _ []byte, err error) { confirmed <- err })

	r, ok := ConfirmationRejection(waitDone(t, confirmed))
	if !ok || r.Cause != fgs.GMMCauseIllegalUE {
		t.Fatalf("rejection %+v, want #3", r)
	}
}

func TestVisitorWithoutN32IsTransient(t *testing.T) {
	f := newRoamingFixture(t)
	f.amf.N32 = openN32{err: errors.New("no N32 context")}
	ue, conn := f.visitorUE(t)

	done := make(chan error, 1)

	f.amf.RequestAuthentication(t.Context(), ue, conn, nil, func(_ context.Context, _ *ausf.AuthResult, err error) { done <- err })

	err := waitDone(t, done)
	if !errors.Is(err, roaming.ErrHPLMNUnreachable) {
		t.Fatalf("error %v, want ErrHPLMNUnreachable", err)
	}

	if r, ok := AuthenticationRejection(err); ok {
		t.Fatalf("rejection %+v, want a transient release", r)
	}
}

func TestHPLMNRegistrationRejectionsFollowTS29524(t *testing.T) {
	cases := []struct {
		status int
		cause  wire.Cause
		want   fgs.GMMCause
	}{
		{http.StatusForbidden, wire.CauseUnknown5GSSubscription, fgs.GMMCauseN1ModeNotAllowed},
		{http.StatusForbidden, wire.CauseRATNotAllowed, fgs.GMMCauseNoSuitableCellsInTrackingArea},
		{http.StatusForbidden, wire.CauseAccessNotAllowed, fgs.GMMCauseNoSuitableCellsInTrackingArea},
		{http.StatusForbidden, wire.CauseRoamingNotAllowed, fgs.GMMCausePLMNNotAllowed},
		{http.StatusForbidden, wire.CauseNoPSSubscription, fgs.GMMCauseServicesNotAllowed},
		{http.StatusNotFound, wire.CauseUserNotFound, fgs.GMMCauseIllegalUE},
		{http.StatusNotFound, wire.CauseContextNotFound, fgs.GMMCauseUEIdentityCannotBeDerived},
		{http.StatusUnprocessableEntity, wire.CauseUnprocessableRequest, fgs.GMMCauseProtocolErrorUnspecified},
	}

	for _, tc := range cases {
		r, ok := HPLMNRegistrationRejection(&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(tc.status, tc.cause, "")})
		if !ok || r.Cause != tc.want {
			t.Errorf("%d %s: %+v, want %s", tc.status, tc.cause, r, tc.want)
		}
	}

	for _, tc := range []struct {
		cause wire.Cause
		want  fgs.GMMCause
	}{
		{wire.CauseUserNotFound, fgs.GMMCauseIllegalUE},
		{wire.CauseDataNotFound, fgs.GMMCauseN1ModeNotAllowed},
	} {
		r, ok := HPLMNRegistrationRejection(&HPLMNFailure{Op: HPLMNSubscriptionData, Err: wire.NewProblem(http.StatusNotFound, tc.cause, "")})
		if !ok || r.Cause != tc.want {
			t.Errorf("SDM 404 %s: %+v, want %s", tc.cause, r, tc.want)
		}
	}

	for _, err := range []error{
		wire.NewProblem(http.StatusGatewayTimeout, wire.CauseNetworkFailure, ""),
		wire.NewProblem(http.StatusForbidden, wire.CauseReauthenticationRequired, ""),
		&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(http.StatusForbidden, "", "")},
		&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(http.StatusForbidden, wire.CauseOriginatingNetworkIDMismatch, "")},
		&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(http.StatusNotFound, "", "")},
		&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(http.StatusNotFound, wire.CauseDataNotFound, "")},
		&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(http.StatusUnprocessableEntity, "", "")},
		&HPLMNFailure{Op: HPLMNRegister, Err: wire.NewProblem(http.StatusUnauthorized, "", "")},
		&HPLMNFailure{Op: HPLMNSubscriptionData, Err: wire.NewProblem(http.StatusForbidden, wire.CauseUnknown5GSSubscription, "")},
		roaming.ErrHPLMNUnreachable,
		roaming.ErrHPLMNUnauthorized,
	} {
		if r, ok := HPLMNRegistrationRejection(err); ok {
			t.Errorf("%v: %+v, want a transient release", err, r)
		}
	}
}

func TestAuthenticationRejectionsFollowTS29524(t *testing.T) {
	cases := []struct {
		err        error
		authReject bool
		cause      fgs.GMMCause
	}{
		{wire.NewProblem(http.StatusForbidden, wire.CauseServingNetworkNotAuthorized, ""), false, fgs.GMMCausePLMNNotAllowed},
		{wire.NewProblem(http.StatusForbidden, wire.CauseAuthenticationRejected, ""), true, 0},
		{wire.NewProblem(http.StatusNotFound, wire.CauseUserNotFound, ""), true, 0},
		{wire.NewProblem(http.StatusNotImplemented, wire.CauseUnsupportedProtectionScheme, ""), true, 0},
		{wire.NewProblem(http.StatusForbidden, wire.CauseInvalidSchemeOutput, ""), true, 0},
		{wire.NewProblem(http.StatusForbidden, wire.CauseInvalidHNPublicKeyIdentifier, ""), true, 0},
		{roaming.ErrUnsupportedAuthType, false, fgs.GMMCauseN1ModeNotAllowed},
	}

	for _, tc := range cases {
		r, ok := AuthenticationRejection(tc.err)
		if !ok || r.AuthReject != tc.authReject || (!tc.authReject && r.Cause != tc.cause) {
			t.Errorf("%v: %+v", tc.err, r)
		}
	}

	for _, err := range []error{
		wire.NewProblem(http.StatusGatewayTimeout, wire.CauseUpstreamServerError, ""),
		wire.NewProblem(http.StatusInternalServerError, wire.CauseAVGenerationProblem, ""),
		wire.NewProblem(http.StatusForbidden, "", ""),
		wire.NewProblem(http.StatusForbidden, wire.CauseOriginatingNetworkIDMismatch, ""),
		wire.NewProblem(http.StatusNotFound, "", ""),
		wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, ""),
		wire.NewProblem(http.StatusNotImplemented, "", ""),
		wire.NewProblem(http.StatusBadRequest, wire.CauseInvalidSchemeOutput, ""),
		wire.NewProblem(http.StatusUnauthorized, "", ""),
		roaming.ErrHPLMNUnreachable,
		roaming.ErrHPLMNUnauthorized,
	} {
		if r, ok := AuthenticationRejection(err); ok {
			t.Errorf("%v: %+v, want a transient release", err, r)
		}
	}
}

func TestHPLMNRegistrationRefusalIsMappedAndNotPurged(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)

	f.authenticate(t, ue, conn)
	f.home.Fail(http.MethodPut, "amf-3gpp-access", wire.NewProblem(http.StatusForbidden, wire.CauseRATNotAllowed, ""))

	done := make(chan error, 1)

	f.amf.RequestHPLMNRegistration(t.Context(), ue, conn, true, func(_ context.Context, err error) { done <- err })

	r, ok := HPLMNRegistrationRejection(waitDone(t, done))
	if !ok || r.Cause != fgs.GMMCauseNoSuitableCellsInTrackingArea {
		t.Fatalf("rejection %+v", r)
	}

	if v, _ := ue.Visitor(); v.Registered {
		t.Fatal("a refused UECM registration is still held")
	}
}

func TestHPLMNRestrictionsRejectTheVisitor(t *testing.T) {
	f := newRoamingFixture(t)
	f.home.Update(roamerIMSI, func(s *fakehplmn.Subscriber) { s.AM.RatRestrictions = []wire.RatType{wire.RatNR} })

	ue, conn := f.visitorUE(t)
	f.authenticate(t, ue, conn)

	done := make(chan error, 1)

	f.amf.RequestHPLMNRegistration(t.Context(), ue, conn, true, func(_ context.Context, err error) { done <- err })

	r, ok := HPLMNRegistrationRejection(waitDone(t, done))
	if !ok || r.Cause != fgs.GMMCauseNoSuitableCellsInTrackingArea {
		t.Fatalf("rejection %+v", r)
	}
}

func TestHPLMNDeregistrationNotification(t *testing.T) {
	cases := []struct {
		reason    wire.DeregistrationReason
		wantPurge bool
	}{
		{wire.DeregUEInitialRegistration, false},
		{wire.DeregSubscriptionWithdrawn, false},
	}

	for _, tc := range cases {
		t.Run(string(tc.reason), func(t *testing.T) {
			f := newRoamingFixture(t)
			ue, conn := f.visitorUE(t)

			f.authenticate(t, ue, conn)
			f.registerAtHPLMN(t, ue, conn)

			v, _ := ue.Visitor()

			if err := f.amf.HPLMNDeregistration(t.Context(), "other-partner", roamerIMSI, wire.DeregistrationData{DeregReason: tc.reason}); !errors.Is(err, roaming.ErrNoVisitorContext) {
				t.Fatalf("notification from another partner: %v", err)
			}

			if err := f.amf.HPLMNDeregistration(t.Context(), v.PartnerID, roamerIMSI, wire.DeregistrationData{DeregReason: tc.reason, AccessType: wire.Access3GPP}); err != nil {
				t.Fatalf("HPLMNDeregistration: %v", err)
			}

			f.amf.SettleHPLMN()

			if f.amf.HoldsUE(roamerIMSI) {
				f.amf.DeregisterAndRemoveUeContext(t.Context(), ue, models.ReleaseFor(models.ReleaseDeregistration))
				f.amf.SettleHPLMN()
			}

			if f.home.Purged("imsi-"+roamerIMSI) != tc.wantPurge {
				t.Fatal("the AMF purged a registration the home UDM had already removed")
			}

			if len(f.home.Subscriptions()) != 0 {
				t.Fatal("the SDM subscription was not removed")
			}
		})
	}
}

func TestHPLMNSubscriptionChangeAppliesTheNewUEAMBR(t *testing.T) {
	f := newRoamingFixture(t)
	ue, conn := f.visitorUE(t)

	f.authenticate(t, ue, conn)
	f.registerAtHPLMN(t, ue, conn)
	ue.ForceStateForTest(Registered)
	ue.SetAllowedNssai([]models.Snssai{{Sst: 1}})

	f.home.Update(roamerIMSI, func(s *fakehplmn.Subscriber) {
		s.AM.SubscribedUeAmbr = &wire.Ambr{Uplink: "10 Mbps", Downlink: "20 Mbps"}
	})

	v, _ := ue.Visitor()

	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, wire.ModificationNotification{SubscriptionID: v.SDM.ID}); err != nil {
		t.Fatalf("HPLMNSubscriptionChanged: %v", err)
	}

	if err := f.amf.HPLMNSubscriptionChanged(t.Context(), v.PartnerID, roamerIMSI, wire.ModificationNotification{SubscriptionID: "other"}); !errors.Is(err, roaming.ErrNoVisitorContext) {
		t.Fatalf("notification for another subscription: %v", err)
	}

	f.amf.SettleHPLMN()

	want := models.Ambr{Uplink: models.BitRateFromBps(10_000_000), Downlink: models.BitRateFromBps(20_000_000)}
	if got := ue.Ambr(); got == nil || *got != want {
		t.Fatalf("UE-AMBR %+v, want %+v", got, want)
	}
}

func TestSoRIsRelayedAndAcknowledged(t *testing.T) {
	f := newRoamingFixture(t)
	ack := true

	f.home.Update(roamerIMSI, func(s *fakehplmn.Subscriber) {
		s.AM.SorInfo = &wire.SorInfo{AckInd: &ack, SorMacIausf: "00112233445566778899aabbccddeeff", Countersor: "0001", ProvisioningTime: "2026-11-03T10:00:00Z"}
	})

	ue, conn := f.visitorUE(t)
	f.authenticate(t, ue, conn)
	f.registerAtHPLMN(t, ue, conn)

	sor := f.amf.SoRContainer(ue)
	if len(sor) != 19 || sor[0] != 0x0c {
		t.Fatalf("SOR transparent container %x", sor)
	}

	ackContainer := append([]byte{0x01}, make([]byte, 16)...)
	ackContainer[1] = 0xab

	f.amf.AcknowledgeSoR(t.Context(), ue, ackContainer)
	f.amf.SettleHPLMN()

	got, ok := f.home.SoRAck("imsi-" + roamerIMSI)
	if !ok || got.SorMacIue != "ab000000000000000000000000000000" || got.ProvisioningTime != "2026-11-03T10:00:00Z" {
		t.Fatalf("SoR acknowledgement %+v", got)
	}

	if f.amf.SoRContainer(ue) != nil {
		t.Fatal("the SoR information is still pending after the acknowledgement")
	}
}
