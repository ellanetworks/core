// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/sh"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
	"github.com/ellanetworks/core/internal/udm"
	"go.uber.org/zap"
)

const testIPSMGWNumber = "15550000100"

var ipSMGWIdentity = diameter.Identity{OriginHost: "ipsmgw.ims.mnc001.mcc001.3gppnetwork.org", OriginRealm: testIMSDomain}

type fakeSMS struct {
	registrations map[string]hss.IPSMGWRegistration
	calls         int
	err           error
}

func newFakeSMS() *fakeSMS {
	return &fakeSMS{registrations: map[string]hss.IPSMGWRegistration{}}
}

func (f *fakeSMS) RegisterIPSMGW(_ context.Context, imsi string, r hss.IPSMGWRegistration) error {
	f.calls++
	if f.err != nil {
		return f.err
	}

	f.registrations[imsi] = r

	return nil
}

func (f *fakeSMS) DeregisterIPSMGW(_ context.Context, imsi string) error {
	f.calls++
	if f.err != nil {
		return f.err
	}

	delete(f.registrations, imsi)

	return nil
}

func smsRegistration(t *testing.T, number string) []byte {
	t.Helper()

	b, err := sh.MarshalShData(sh.ShData{SMSRegistration: &sh.SMSRegistrationInfo{IPSMGWNumber: number}})
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func profileUpdate(t *testing.T, store *fakeIMSStore, sms hss.SMS, r sh.ProfileUpdateRequest) *diameter.Message {
	t.Helper()

	if r.DataReferences == nil {
		r.DataReferences = []sh.DataReference{sh.DataSMSRegistrationInfo}
	}

	req, err := sh.NewProfileUpdateRequest(tgpp.Envelope{
		SessionID: ipSMGWIdentity.OriginHost + ";1;1", Origin: ipSMGWIdentity,
		DestinationHost: hssIdentity.OriginHost, DestinationRealm: testIMSDomain,
	}, r)
	if err != nil {
		t.Fatalf("build PUR: %v", err)
	}

	return hss.New(store, udm.NewIMSCredentials(store), nil, sms, zap.NewNop()).ProfileUpdate(context.Background(), hssIdentity, req)
}

func TestProfileUpdateRegistersTheIPSMGW(t *testing.T) {
	for name, user := range map[string]sh.UserIdentity{
		"MSISDN":     {MSISDN: testMSISDN},
		"tel URI":    {PublicIdentity: "tel:+" + testMSISDN},
		"SIP of tel": {PublicIdentity: "sip:+" + testMSISDN + "@" + testIMSDomain},
	} {
		t.Run(name, func(t *testing.T) {
			store := newFakeIMSStore()
			sms := newFakeSMS()

			requireResult(t, profileUpdate(t, store, sms, sh.ProfileUpdateRequest{User: user, UserData: smsRegistration(t, testIPSMGWNumber)}),
				tgpp.Result{Code: diameter.ResultSuccess})

			want := hss.IPSMGWRegistration{Number: testIPSMGWNumber, DiameterHost: ipSMGWIdentity.OriginHost, DiameterRealm: ipSMGWIdentity.OriginRealm}
			if got := sms.registrations[testIMSI]; got != want {
				t.Fatalf("registration = %+v, want %+v", got, want)
			}

			requireResult(t, profileUpdate(t, store, sms, sh.ProfileUpdateRequest{User: user, UserData: smsRegistration(t, "")}),
				tgpp.Result{Code: diameter.ResultSuccess})

			if _, ok := sms.registrations[testIMSI]; ok {
				t.Fatalf("after deregistration: %+v", sms.registrations)
			}
		})
	}
}

func TestProfileUpdateErrors(t *testing.T) {
	cases := map[string]struct {
		req   sh.ProfileUpdateRequest
		setup func(*fakeIMSStore, *fakeSMS)
		want  tgpp.Result
	}{
		"unknown MSISDN": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{MSISDN: "15559999999"}},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		"unknown public identity": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{PublicIdentity: "tel:+15559999999"}},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		"external identifier": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{ExternalIdentifier: "user@example.org"}},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		"private identity of another user": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{MSISDN: testMSISDN}, PrivateIdentity: impiOf(testOtherIMSI)},
			want: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch),
		},
		"public service identity": {
			req: sh.ProfileUpdateRequest{User: sh.UserIdentity{PublicIdentity: "sip:conference@" + testIMSDomain}},
			setup: func(s *fakeIMSStore, _ *fakeSMS) {
				s.serviceIDs = map[string]string{"sip:conference@" + testIMSDomain: "sip:as.example.org"}
			},
			want: tgpp.Experimental(tgpp.ResultErrorOperationNotAllowed),
		},
		"other data": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{MSISDN: testMSISDN}, DataReferences: []sh.DataReference{sh.DataRepositoryData}},
			want: tgpp.Experimental(tgpp.ResultErrorUserDataCannotBeModified),
		},
		"unrecognised user data": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{MSISDN: testMSISDN}, UserData: []byte("<Sh-Data/>")},
			want: tgpp.Experimental(tgpp.ResultErrorUserDataNotRecognized),
		},
		"wildcarded PSI": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{PublicIdentity: "sip:conf1@" + testIMSDomain}, WildcardedPublicIdentity: "sip:conf!.*!@" + testIMSDomain},
			want: tgpp.Experimental(tgpp.ResultErrorOperationNotAllowed),
		},
		"wildcarded IMPU": {
			req:  sh.ProfileUpdateRequest{User: sh.UserIdentity{PublicIdentity: "tel:+" + testMSISDN}, WildcardedIMPU: "sip:user!.*!@" + testIMSDomain},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		"store unavailable": {
			req:   sh.ProfileUpdateRequest{User: sh.UserIdentity{MSISDN: testMSISDN}},
			setup: func(_ *fakeIMSStore, sms *fakeSMS) { sms.err = hss.ErrUnavailable },
			want:  tgpp.Result{Code: diameter.ResultTooBusy},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store, sms := newFakeIMSStore(), newFakeSMS()
			if tc.setup != nil {
				tc.setup(store, sms)
			}

			if tc.req.UserData == nil {
				tc.req.UserData = smsRegistration(t, testIPSMGWNumber)
			}

			requireResult(t, profileUpdate(t, store, sms, tc.req), tc.want)

			if len(sms.registrations) != 0 {
				t.Fatalf("registered %+v", sms.registrations)
			}
		})
	}
}
