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
	registered := hss.Registration{State: hss.Registered, ServerName: testSCSCF}
	unregistered := hss.Registration{State: hss.Unregistered, ServerName: testSCSCF}
	authenticating := hss.Registration{State: hss.NotRegistered, ServerName: testSCSCF, AuthPending: true}

	cases := []struct {
		name         string
		registration *hss.Registration
		req          cx.LocationInfoRequest
		want         tgpp.Result
		serverName   string
		capabilities bool
	}{
		{
			name:         "registered MSISDN public identity",
			registration: &registered,
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN},
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
			name:         "originating request for a user that is not registered",
			req:          cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN, Originating: true},
			want:         tgpp.Experimental(tgpp.ResultUnregisteredService),
			capabilities: true,
		},
		{
			name: "unknown MSISDN",
			req:  cx.LocationInfoRequest{PublicIdentity: "tel:+15559999999"},
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

			ans := locationInfo(t, store, tc.req)
			requireResult(t, ans, tc.want)

			if tc.want.Failure() {
				return
			}

			li, err := cx.ParseLocationInfoAnswer(ans)
			if err != nil {
				t.Fatalf("parse LIA: %v", err)
			}

			if li.ServerName != tc.serverName || (li.Capabilities != nil) != tc.capabilities {
				t.Fatalf("LIA server name %q, capabilities %+v; want %q, capabilities %t", li.ServerName, li.Capabilities, tc.serverName, tc.capabilities)
			}
		})
	}
}

func TestLocationInfoStoreFailureIsUnableToComply(t *testing.T) {
	store := newFakeIMSStore()
	store.fail = true

	ans := locationInfo(t, store, cx.LocationInfoRequest{PublicIdentity: "sip:" + impiOf(testIMSI)})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultUnableToComply})
}
