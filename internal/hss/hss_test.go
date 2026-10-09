// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
	"github.com/ellanetworks/core/internal/milenage"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/core/internal/udm"
	"go.uber.org/zap"
)

const (
	testIMSDomain = "ims.mnc001.mcc001.3gppnetwork.org"
	testIMSI      = "001010000000001"
	testOtherIMSI = "001010000000002"
	testMSISDN    = "15551230001"
	testOtherTel  = "15551230002"
	testNoMSISDN  = "001010000000003"
	testNoIMS     = "001010000000004"
	testNoIMSTel  = "15551230004"
	testK         = "465b5ce8b199b49faa5f0a2ee238a6bc"
	testOPc       = "cd63cb71954a9f4e48a5994e37a02baf"
	testSCSCF     = "sip:scscf.ims.mnc001.mcc001.3gppnetwork.org:5080"
)

var (
	hssIdentity  = diameter.Identity{OriginHost: "mmec01.mmegi0001.mme.epc.mnc001.mcc001.3gppnetwork.org", OriginRealm: "epc.mnc001.mcc001.3gppnetwork.org"}
	cscfIdentity = diameter.Identity{OriginHost: "ims.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testIMSDomain}
	errStoreDown = errors.New("store unavailable")
)

type fakeIMSSubscriber struct {
	hss.Subscriber
	sqn string
}

type fakeIMSStore struct {
	subscribers   map[string]*fakeIMSSubscriber
	registrations map[string]hss.Registration
	noPCSCF       bool
	fail          bool
}

func newFakeIMSStore() *fakeIMSStore {
	return &fakeIMSStore{
		subscribers: map[string]*fakeIMSSubscriber{
			testIMSI:      {Subscriber: hss.Subscriber{IMSI: testIMSI, MSISDN: testMSISDN, IMSDataNetwork: true}, sqn: "000000000020"},
			testOtherIMSI: {Subscriber: hss.Subscriber{IMSI: testOtherIMSI, MSISDN: testOtherTel, IMSDataNetwork: true}, sqn: "000000000020"},
			testNoMSISDN:  {Subscriber: hss.Subscriber{IMSI: testNoMSISDN, IMSDataNetwork: true}, sqn: "000000000020"},
			testNoIMS:     {Subscriber: hss.Subscriber{IMSI: testNoIMS, MSISDN: testNoIMSTel}, sqn: "000000000020"},
		},
		registrations: map[string]hss.Registration{},
	}
}

func (f *fakeIMSStore) Registration(_ context.Context, imsi string) (*hss.Registration, error) {
	if f.fail {
		return nil, errStoreDown
	}

	reg, ok := f.registrations[imsi]
	if !ok {
		return nil, nil
	}

	return &reg, nil
}

func (f *fakeIMSStore) CompareAndSwapRegistration(_ context.Context, imsi string, expected, next *hss.Registration) (*hss.Registration, bool, error) {
	current, ok := f.registrations[imsi]

	switch {
	case !ok && expected != nil:
		return nil, false, nil
	case ok && (expected == nil || current != *expected):
		return &current, false, nil
	case next == nil:
		delete(f.registrations, imsi)
	default:
		f.registrations[imsi] = *next
	}

	return next, true, nil
}

func (f *fakeIMSStore) IMSDomain(context.Context) (string, error) {
	if f.fail {
		return "", errStoreDown
	}

	return testIMSDomain, nil
}

func (f *fakeIMSStore) PCSCFConfigured(context.Context) (bool, error) {
	if f.fail {
		return false, errStoreDown
	}

	return !f.noPCSCF, nil
}

func (f *fakeIMSStore) Subscriber(_ context.Context, imsi string) (*hss.Subscriber, error) {
	if f.fail {
		return nil, errStoreDown
	}

	s, ok := f.subscribers[imsi]
	if !ok {
		return nil, hss.ErrSubscriberUnknown
	}

	sub := s.Subscriber

	return &sub, nil
}

func (f *fakeIMSStore) CountRegistered(context.Context) (int, error) {
	n := 0

	for _, reg := range f.registrations {
		if reg.State == hss.Registered {
			n++
		}
	}

	return n, nil
}

func (f *fakeIMSStore) SubscriberByMSISDN(_ context.Context, msisdn string) (*hss.Subscriber, error) {
	for _, s := range f.subscribers {
		if s.MSISDN == msisdn {
			sub := s.Subscriber
			return &sub, nil
		}
	}

	return nil, hss.ErrSubscriberUnknown
}

func (f *fakeIMSStore) AdvanceIMSSequenceNumber(_ context.Context, imsi, resyncAuts, resyncRand string) (*udm.AdvancedCredentials, error) {
	s, ok := f.subscribers[imsi]
	if !ok {
		return nil, udm.ErrSubscriberUnknown
	}

	next, err := sqn.NextIMS(s.sqn)
	if resyncAuts != "" {
		next, err = sqn.NextIMSResync(s.sqn, testOPc, testK, resyncAuts, resyncRand)
	}

	if err != nil {
		return nil, err
	}

	s.sqn = next

	return &udm.AdvancedCredentials{PermanentKey: testK, Opc: testOPc, SequenceNumber: next}, nil
}

func newHSS(store *fakeIMSStore) *hss.HSS {
	return hss.New(store, udm.NewIMSCredentials(store), nil, zap.NewNop())
}

func impiOf(imsi string) string {
	return imsi + "@" + testIMSDomain
}

func envelope() tgpp.Envelope {
	return tgpp.Envelope{
		SessionID:        cscfIdentity.OriginHost + ";1;1",
		Origin:           cscfIdentity,
		DestinationHost:  hssIdentity.OriginHost,
		DestinationRealm: testIMSDomain,
	}
}

func userAuthorization(t *testing.T, store *fakeIMSStore, r cx.UserAuthorizationRequest) *diameter.Message {
	t.Helper()

	if r.VisitedNetwork == "" {
		r.VisitedNetwork = testIMSDomain
	}

	req, err := cx.NewUserAuthorizationRequest(envelope(), r)
	if err != nil {
		t.Fatalf("build UAR: %v", err)
	}

	return newHSS(store).UserAuthorization(context.Background(), hssIdentity, req)
}

func multimediaAuth(t *testing.T, store *fakeIMSStore, r cx.MultimediaAuthRequest) *diameter.Message {
	t.Helper()

	if r.Scheme == "" {
		r.Scheme = cx.SchemeDigestAKAv1MD5
	}

	if r.ServerName == "" {
		r.ServerName = testSCSCF
	}

	req, err := cx.NewMultimediaAuthRequest(envelope(), r)
	if err != nil {
		t.Fatalf("build MAR: %v", err)
	}

	return newHSS(store).MultimediaAuth(context.Background(), hssIdentity, req)
}

func requireResult(t *testing.T, ans *diameter.Message, want tgpp.Result) {
	t.Helper()

	got, err := tgpp.ParseResult(ans)
	if err != nil {
		t.Fatalf("parse result: %v", err)
	}

	if got.Code != want.Code || got.Experimental != want.Experimental {
		t.Fatalf("result = %s, want %s", got, want)
	}
}

func TestUserAuthorization(t *testing.T) {
	cases := []struct {
		name string
		req  cx.UserAuthorizationRequest
		want tgpp.Result
	}{
		{
			name: "registration with the temporary public identity",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI)},
			want: tgpp.Experimental(tgpp.ResultFirstRegistration),
		},
		{
			name: "registration with the MSISDN public identity",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN},
			want: tgpp.Experimental(tgpp.ResultFirstRegistration),
		},
		{
			name: "registration and capabilities",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI), AuthorizationType: cx.AuthorizationRegistrationAndCapabilities},
			want: tgpp.Result{Code: diameter.ResultSuccess},
		},
		{
			name: "deregistration of an unregistered user",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI), AuthorizationType: cx.AuthorizationDeregistration},
			want: tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered),
		},
		{
			name: "unknown subscriber",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf("001010000000099"), PublicIdentity: "sip:" + impiOf("001010000000099")},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "private identity in another domain",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: testIMSI + "@ims.example.org", PublicIdentity: "sip:" + impiOf(testIMSI)},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "unknown public identity",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:alice@" + testIMSDomain},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "public identity of another subscriber",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testOtherIMSI)},
			want: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch),
		},
		{
			name: "registration of a subscriber without the ims data network",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testNoIMS), PublicIdentity: "tel:+" + testNoIMSTel},
			want: tgpp.Result{Code: diameter.ResultAuthorizationRejected},
		},
		{
			name: "registration and capabilities of a subscriber without the ims data network",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testNoIMS), PublicIdentity: "tel:+" + testNoIMSTel, AuthorizationType: cx.AuthorizationRegistrationAndCapabilities},
			want: tgpp.Result{Code: diameter.ResultAuthorizationRejected},
		},
		{
			name: "deregistration of a subscriber without the ims data network",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testNoIMS), PublicIdentity: "tel:+" + testNoIMSTel, AuthorizationType: cx.AuthorizationDeregistration},
			want: tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered),
		},
		{
			name: "MSISDN of another subscriber",
			req:  cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testOtherTel},
			want: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ans := userAuthorization(t, newFakeIMSStore(), tc.req)
			requireResult(t, ans, tc.want)

			if !tc.want.Success() {
				return
			}

			ua, err := cx.ParseUserAuthorizationAnswer(ans)
			if err != nil {
				t.Fatalf("parse UAA: %v", err)
			}

			if ua.ServerName != "" || ua.Capabilities == nil {
				t.Fatalf("UAA server name %q, capabilities %+v; want empty capabilities", ua.ServerName, ua.Capabilities)
			}
		})
	}
}

func TestUserAuthorizationStoreFailureIsUnableToComply(t *testing.T) {
	store := newFakeIMSStore()
	store.fail = true

	ans := userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI)})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultUnableToComply})
}

func TestMultimediaAuthReturnsAnIMSAKAVector(t *testing.T) {
	store := newFakeIMSStore()

	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI)})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	ma, err := cx.ParseMultimediaAuthAnswer(ans)
	if err != nil {
		t.Fatalf("parse MAA: %v", err)
	}

	if ma.PrivateIdentity != impiOf(testIMSI) || ma.PublicIdentity != "sip:"+impiOf(testIMSI) {
		t.Fatalf("MAA identities %q %q", ma.PrivateIdentity, ma.PublicIdentity)
	}

	if len(ma.Items) != 1 {
		t.Fatalf("MAA items = %d, want 1", len(ma.Items))
	}

	item := ma.Items[0]
	if item.ItemNumber != 1 || item.Scheme != cx.SchemeDigestAKAv1MD5 || item.AKA == nil {
		t.Fatalf("MAA item = %+v", item)
	}

	gotSQN := verifyAsUE(t, item.AKA)

	if want := store.subscribers[testIMSI].sqn; hex.EncodeToString(gotSQN) != want {
		t.Fatalf("SQN = %x, want %s", gotSQN, want)
	}

	if ind := gotSQN[5] & 0x1f; ind != byte(sqn.IMSInd) {
		t.Fatalf("IND = %d, want %d", ind, sqn.IMSInd)
	}
}

func TestMultimediaAuthAdvancesTheSequenceNumber(t *testing.T) {
	store := newFakeIMSStore()
	req := cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN}

	var previous []byte

	for range 3 {
		ma, err := cx.ParseMultimediaAuthAnswer(multimediaAuth(t, store, req))
		if err != nil {
			t.Fatalf("parse MAA: %v", err)
		}

		got := verifyAsUE(t, ma.Items[0].AKA)
		if previous != nil && bytes.Compare(got, previous) <= 0 {
			t.Fatalf("SQN %x does not follow %x", got, previous)
		}

		previous = got
	}
}

func TestMultimediaAuthErrors(t *testing.T) {
	resync := &cx.Resync{RAND: make([]byte, 16), AUTS: make([]byte, 14)}

	cases := []struct {
		name string
		req  cx.MultimediaAuthRequest
		want tgpp.Result
	}{
		{
			name: "unsupported scheme",
			req:  cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI), Scheme: cx.SchemeSIPDigest},
			want: tgpp.Experimental(tgpp.ResultErrorAuthSchemeNotSupported),
		},
		{
			name: "unknown scheme",
			req:  cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI), Scheme: cx.SchemeUnknown},
			want: tgpp.Experimental(tgpp.ResultErrorAuthSchemeNotSupported),
		},
		{
			name: "resynchronisation without a challenge",
			req:  cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI), Resync: resync},
			want: tgpp.Result{Code: diameter.ResultUnableToComply},
		},
		{
			name: "unknown subscriber",
			req:  cx.MultimediaAuthRequest{PrivateIdentity: impiOf("001010000000099"), PublicIdentity: "sip:" + impiOf("001010000000099")},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "public identity of another subscriber",
			req:  cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testOtherTel},
			want: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()

			requireResult(t, multimediaAuth(t, store, tc.req), tc.want)

			if got := store.subscribers[testIMSI].sqn; got != "000000000020" {
				t.Fatalf("SQN advanced to %s on a rejected request", got)
			}
		})
	}
}

func autsAsUE(t *testing.T, rand []byte, sqnMS []byte) []byte {
	t.Helper()

	k, _ := hex.DecodeString(testK)
	opc, _ := hex.DecodeString(testOPc)

	akStar := make([]byte, 6)
	if err := milenage.F2345(opc, k, rand, nil, nil, nil, nil, akStar); err != nil {
		t.Fatalf("f5*: %v", err)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	if err := milenage.F1(opc, k, rand, sqnMS, []byte{0, 0}, macA, macS); err != nil {
		t.Fatalf("f1*: %v", err)
	}

	auts := make([]byte, 0, 14)
	for i := range sqnMS {
		auts = append(auts, sqnMS[i]^akStar[i])
	}

	return append(auts, macS...)
}

func firstVector(t *testing.T, store *fakeIMSStore) *cx.AKAVector {
	t.Helper()

	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN})

	ma, err := cx.ParseMultimediaAuthAnswer(ans)
	if err != nil {
		t.Fatalf("parse MAA: %v", err)
	}

	return ma.Items[0].AKA
}

func TestMultimediaAuthResynchronises(t *testing.T) {
	store := newFakeIMSStore()
	first := firstVector(t, store)

	sqnMS, _ := hex.DecodeString("00000000105e")
	resync := &cx.Resync{RAND: first.RAND, AUTS: autsAsUE(t, first.RAND, sqnMS)}

	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN, Resync: resync})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	ma, err := cx.ParseMultimediaAuthAnswer(ans)
	if err != nil {
		t.Fatalf("parse MAA: %v", err)
	}

	got := verifyAsUE(t, ma.Items[0].AKA)
	if hex.EncodeToString(got) != "00000000107f" {
		t.Fatalf("SQN after resynchronisation = %x, want 00000000107f", got)
	}

	if store.subscribers[testIMSI].sqn != "00000000107f" {
		t.Fatalf("stored SQN = %s, want 00000000107f", store.subscribers[testIMSI].sqn)
	}
}

func TestMultimediaAuthResyncFailures(t *testing.T) {
	cases := []struct {
		name       string
		serverName string
		corrupt    bool
	}{
		{name: "bad MAC-S", corrupt: true},
		{name: "another S-CSCF", serverName: "sip:scscf2.ims.mnc001.mcc001.3gppnetwork.org:5080"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			first := firstVector(t, store)
			stored := store.subscribers[testIMSI].sqn

			sqnMS, _ := hex.DecodeString("00000000105e")
			auts := autsAsUE(t, first.RAND, sqnMS)

			if tc.corrupt {
				auts[13] ^= 0x01
			}

			req := cx.MultimediaAuthRequest{
				PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN, ServerName: tc.serverName,
				Resync: &cx.Resync{RAND: first.RAND, AUTS: auts},
			}

			requireResult(t, multimediaAuth(t, store, req), tgpp.Result{Code: diameter.ResultUnableToComply})

			if got := store.subscribers[testIMSI].sqn; got != stored {
				t.Fatalf("SQN moved from %s to %s on a refused resynchronisation", stored, got)
			}
		})
	}
}

func verifyAsUE(t *testing.T, v *cx.AKAVector) []byte {
	t.Helper()

	k, _ := hex.DecodeString(testK)
	opc, _ := hex.DecodeString(testOPc)

	res, ck, ik := make([]byte, 8), make([]byte, 16), make([]byte, 16)
	ak := make([]byte, 6)

	if err := milenage.F2345(opc, k, v.RAND, res, ck, ik, ak, nil); err != nil {
		t.Fatalf("f2345: %v", err)
	}

	sqnUE := make([]byte, 6)
	for i := range sqnUE {
		sqnUE[i] = v.AUTN[i] ^ ak[i]
	}

	amf := v.AUTN[6:8]
	if !bytes.Equal(amf, []byte{0x00, 0x00}) {
		t.Fatalf("AMF = %x, want 0000", amf)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	if err := milenage.F1(opc, k, v.RAND, sqnUE, amf, macA, macS); err != nil {
		t.Fatalf("f1: %v", err)
	}

	switch {
	case !bytes.Equal(macA, v.AUTN[8:]):
		t.Fatalf("MAC-A mismatch")
	case !bytes.Equal(res, v.XRES):
		t.Fatalf("RES %x != XRES %x", res, v.XRES)
	case !bytes.Equal(ck, v.CK) || !bytes.Equal(ik, v.IK):
		t.Fatalf("CK/IK mismatch")
	}

	return sqnUE
}

func TestTermination(t *testing.T) {
	cases := []struct {
		name string
		reg  *hss.Registration
		want bool
	}{
		{name: "no registration"},
		{name: "authentication pending", reg: &hss.Registration{State: hss.NotRegistered, ServerName: testSCSCF, AuthPending: true, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm}},
		{name: "registered", reg: &hss.Registration{State: hss.Registered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm}, want: true},
		{name: "unregistered", reg: &hss.Registration{State: hss.Unregistered, ServerName: testSCSCF, OriginHost: cscfIdentity.OriginHost, OriginRealm: cscfIdentity.OriginRealm}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			if tc.reg != nil {
				store.registrations[testIMSI] = *tc.reg
			}

			got, err := newHSS(store).Termination(context.Background(), testIMSI)
			if err != nil {
				t.Fatalf("Termination: %v", err)
			}

			if !tc.want {
				if got != nil {
					t.Fatalf("Termination = %+v, want none", got)
				}

				return
			}

			want := hss.Termination{IMSI: testIMSI, PrivateIdentity: impiOf(testIMSI), ServerHost: cscfIdentity.OriginHost, ServerRealm: cscfIdentity.OriginRealm}
			if got == nil || *got != want {
				t.Fatalf("Termination = %+v, want %+v", got, want)
			}
		})
	}
}

func TestTerminateWithoutDiameter(t *testing.T) {
	err := newHSS(newFakeIMSStore()).Terminate(context.Background(), &hss.Termination{IMSI: testIMSI, PrivateIdentity: impiOf(testIMSI), ServerHost: cscfIdentity.OriginHost})
	if !errors.Is(err, hss.ErrDiameterUnavailable) {
		t.Fatalf("Terminate error = %v, want %v", err, hss.ErrDiameterUnavailable)
	}
}
