// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	chatServer  = "sip:chat.example.com"
	mcpttServer = "sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org"
)

func psiStore() *fakeIMSStore {
	store := newFakeIMSStore()
	store.serviceIDs = map[string]string{
		"sip:chatlist!.*!@example.com":                     chatServer,
		"sip:chatlist1@example.com":                        "sip:distinct.example.com",
		"sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org": mcpttServer,
		"tel:+" + testMSISDN:                               "sip:shadowed.example.com",
	}

	return store
}

func TestLocationInfoRoutesPublicServiceIdentitiesDirectly(t *testing.T) {
	for _, tc := range []struct {
		identity   string
		serverName string
		wildcard   string
	}{
		{identity: "sip:chatlist1@example.com", serverName: "sip:distinct.example.com"},
		{identity: "sip:chatlist2@example.com", serverName: chatServer, wildcard: "sip:chatlist!.*!@example.com"},
		{identity: "sip:chatlist42@example.com", serverName: chatServer, wildcard: "sip:chatlist!.*!@example.com"},
		{identity: "sip:chatlistAbC@example.com", serverName: chatServer, wildcard: "sip:chatlist!.*!@example.com"},
		{identity: "sip:chatlist!1@example.com", serverName: chatServer, wildcard: "sip:chatlist!.*!@example.com"},
		{identity: "sip:group-1@mcptt.ims.mnc001.mcc001.3gppnetwork.org", serverName: mcpttServer, wildcard: "sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org"},
	} {
		t.Run(tc.identity, func(t *testing.T) {
			ans := locationInfo(t, psiStore(), cx.LocationInfoRequest{PublicIdentity: tc.identity})
			requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

			li, err := cx.ParseLocationInfoAnswer(ans)
			if err != nil {
				t.Fatalf("parse LIA: %v", err)
			}

			if li.ServerName != tc.serverName || li.WildcardedPublicIdentity != tc.wildcard || !li.PSIDirectRouting {
				t.Fatalf("LIA server %q, wildcard %q, direct routing %t; want %q, %q, true", li.ServerName, li.WildcardedPublicIdentity, li.PSIDirectRouting, tc.serverName, tc.wildcard)
			}
		})
	}
}

func TestLocationInfoForPublicServiceIdentitiesFallsBack(t *testing.T) {
	cases := []struct {
		name string
		req  cx.LocationInfoRequest
		want tgpp.Result
	}{
		{name: "originating request", req: cx.LocationInfoRequest{PublicIdentity: "sip:chatlist2@example.com", Originating: true}, want: tgpp.Experimental(tgpp.ResultErrorUserUnknown)},
		{name: "no matching PSI", req: cx.LocationInfoRequest{PublicIdentity: "sip:chatlist2@example.org"}, want: tgpp.Experimental(tgpp.ResultErrorUserUnknown)},
		{name: "subscriber identities take precedence", req: cx.LocationInfoRequest{PublicIdentity: "tel:+" + testMSISDN}, want: tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requireResult(t, locationInfo(t, psiStore(), tc.req), tc.want)
		})
	}
}

func TestPublicServiceIdentitiesAreNotRegistrable(t *testing.T) {
	store := psiStore()

	requireResult(t, userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:chatlist2@example.com"}),
		tgpp.Experimental(tgpp.ResultErrorUserUnknown))

	requireResult(t, serverAssignment(t, store, cx.ServerAssignmentRequest{PublicIdentities: []string{"sip:chatlist2@example.com"}, ServerName: testSCSCF, Type: cx.AssignmentUnregisteredUser}),
		tgpp.Experimental(tgpp.ResultErrorUserUnknown))
}
