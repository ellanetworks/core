// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
)

func locationInfo(t *testing.T, store *fakeIMSStore, r cx.LocationInfoRequest) *diameter.Message {
	t.Helper()

	req, err := cx.NewLocationInfoRequest(envelope(), r)
	if err != nil {
		t.Fatalf("build LIR: %v", err)
	}

	return newHSS(store).LocationInfo(context.Background(), hssIdentity, req)
}

func TestLocationInfo(t *testing.T) {
	registered := hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Registered}}}
	unregistered := hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Unregistered}}}
	authenticating := hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.NotRegistered, AuthPending: true}}}
	emergency := hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Unregistered, Emergency: true}}}

	cases := []struct {
		name                 string
		registration         *hss.Registration
		unentitled           bool
		unregisteredServices bool
		req                  cx.LocationInfoRequest
		want                 tgpp.Result
		serverName           string
	}{
		{
			name:         "registered MSISDN public identity",
			registration: &registered,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name:         "registered SIP URI of the MSISDN",
			registration: &registered,
			req:          cx.LocationInfoRequest{PublicIdentity: "sip:+" + testMSISDN + "@" + testIMSDomain},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name:         "registered temporary public identity",
			registration: &registered,
			req:          cx.LocationInfoRequest{PublicIdentity: "sip:" + impiOf(testIMSI)},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name:         "unregistered user with an assigned S-CSCF",
			registration: &unregistered,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name:         "registered user queried with registration and capabilities",
			registration: &registered,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN, AuthorizationType: cx.AuthorizationRegistrationAndCapabilities},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name: "not registered",
			req:  cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want: tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered),
		},
		{
			name:         "authentication pending",
			registration: &authenticating,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:         tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered),
		},
		{
			name:                 "authentication pending, with unregistered services",
			registration:         &authenticating,
			unregisteredServices: true,
			req:                  cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:                 tgpp.Result{Code: diameter.ResultSuccess},
			serverName:           testSCSCF,
		},
		{
			name:                 "not registered, with unregistered services",
			unregisteredServices: true,
			req:                  cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:                 tgpp.Experimental(tgpp.ResultUnregisteredService),
		},
		{
			name: "originating request for a user that is not registered",
			req:  cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN, Originating: true},
			want: tgpp.Experimental(tgpp.ResultUnregisteredService),
		},
		{
			name:         "originating request for a user authenticating",
			registration: &authenticating,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN, Originating: true},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name:                 "user not entitled to IMS, with unregistered services",
			unentitled:           true,
			unregisteredServices: true,
			req:                  cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:                 tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered),
		},
		{
			name:       "originating request for a user not entitled to IMS",
			unentitled: true,
			req:        cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN, Originating: true},
			want:       tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered),
		},
		{
			name:         "emergency registration of a user not entitled to IMS",
			registration: &emergency,
			unentitled:   true,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
			want:         tgpp.Result{Code: diameter.ResultSuccess},
			serverName:   testSCSCF,
		},
		{
			name: "unknown MSISDN",
			req:  cx.LocationInfoRequest{PublicIdentity: "tel:+15559999999"},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "unknown SIP URI of an MSISDN",
			req:  cx.LocationInfoRequest{PublicIdentity: "sip:+15559999999@" + testIMSDomain},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "SIP URI of an MSISDN in another domain",
			req:  cx.LocationInfoRequest{PublicIdentity: "sip:+" + testMSISDN + "@ims.example.org"},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "unknown SIP public identity",
			req:  cx.LocationInfoRequest{PublicIdentity: "sip:alice@" + testIMSDomain},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "public identity in another domain",
			req:  cx.LocationInfoRequest{PublicIdentity: "sip:" + testIMSI + "@ims.example.org"},
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()
			if tc.registration != nil {
				store.registrations[testIMSI] = *tc.registration
			}

			store.subscribers[testIMSI].IMSDataNetwork = !tc.unentitled

			if tc.unregisteredServices {
				store.filterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: {{ApplicationServer: cx.ApplicationServer{ServerName: "sip:voicemail.example.org"}}}}
			}

			ans := locationInfo(t, store, tc.req)
			requireResult(t, ans, tc.want)

			if tc.want.Failure() {
				return
			}

			li, err := cx.ParseLocationInfoAnswer(ans)
			if err != nil {
				t.Fatalf("parse LIA: %v", err)
			}

			if li.ServerName != tc.serverName || li.Capabilities != nil {
				t.Fatalf("LIA server name %q, capabilities %+v; want %q and no capabilities", li.ServerName, li.Capabilities, tc.serverName)
			}
		})
	}
}
