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

const otherSCSCF = "sip:scscf2.ims.mnc001.mcc001.3gppnetwork.org:5080"

func serverAssignment(t *testing.T, store *fakeIMSStore, r cx.ServerAssignmentRequest) *diameter.Message {
	t.Helper()

	if r.ServerName == "" {
		r.ServerName = testSCSCF
	}

	req, err := cx.NewServerAssignmentRequest(envelope(), r)
	if err != nil {
		t.Fatalf("build SAR: %v", err)
	}

	return newHSS(store).ServerAssignment(context.Background(), hssIdentity, req)
}

func register(imsi string, t cx.AssignmentType) cx.ServerAssignmentRequest {
	return cx.ServerAssignmentRequest{PrivateIdentity: impiOf(imsi), PublicIdentities: []string{"sip:" + impiOf(imsi)}, Type: t}
}

func challenge(t *testing.T, store *fakeIMSStore, server string) {
	t.Helper()

	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI), ServerName: server})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})
}

func requireRegistration(t *testing.T, store *fakeIMSStore, want *hss.Registration) {
	t.Helper()

	got, ok := store.registrations[testIMSI]

	switch {
	case want == nil && ok:
		t.Fatalf("registration = %+v, want none", got)
	case want == nil:
		return
	case !ok:
		t.Fatalf("no registration, want %+v", *want)
	case got.State != want.State || got.ServerName != want.ServerName || got.AuthPending != want.AuthPending:
		t.Fatalf("registration = %+v, want %+v", got, *want)
	}
}

func uarServerName(t *testing.T, ans *diameter.Message) string {
	t.Helper()

	ua, err := cx.ParseUserAuthorizationAnswer(ans)
	if err != nil {
		t.Fatalf("parse UAA: %v", err)
	}

	return ua.ServerName
}

func TestRegistrationAndDeregistration(t *testing.T) {
	store := newFakeIMSStore()
	uar := cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "sip:" + impiOf(testIMSI)}

	requireResult(t, userAuthorization(t, store, uar), tgpp.Experimental(tgpp.ResultFirstRegistration))

	challenge(t, store, testSCSCF)
	requireRegistration(t, store, &hss.Registration{State: hss.NotRegistered, ServerName: testSCSCF, AuthPending: true})

	if got := store.registrations[testIMSI]; got.OriginHost != cscfIdentity.OriginHost || got.OriginRealm != cscfIdentity.OriginRealm {
		t.Fatalf("stored origin %q %q", got.OriginHost, got.OriginRealm)
	}

	ans := userAuthorization(t, store, uar)
	requireResult(t, ans, tgpp.Experimental(tgpp.ResultSubsequentRegistration))

	if name := uarServerName(t, ans); name != testSCSCF {
		t.Fatalf("UAA server name = %q after the challenge", name)
	}

	ans = serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration))
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})
	requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: testSCSCF})

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		t.Fatalf("parse SAA: %v", err)
	}

	if sa.PrivateIdentity != impiOf(testIMSI) {
		t.Fatalf("SAA User-Name = %q", sa.PrivateIdentity)
	}

	profile, err := cx.ParseUserData(sa.UserData)
	if err != nil {
		t.Fatalf("parse User-Data: %v", err)
	}

	want := []cx.ProfileIdentity{
		{Identity: "tel:+" + testMSISDN},
		{Identity: "sip:" + impiOf(testIMSI), Barred: true},
	}

	if profile.PrivateIdentity != impiOf(testIMSI) || len(profile.ServiceProfiles) != 1 || len(profile.ServiceProfiles[0].PublicIdentities) != len(want) {
		t.Fatalf("User-Data = %+v", profile)
	}

	for i, identity := range profile.ServiceProfiles[0].PublicIdentities {
		if identity.Identity != want[i].Identity || identity.Barred != want[i].Barred {
			t.Fatalf("public identity %d = %+v, want %+v", i, identity, want[i])
		}
	}

	reRegister := register(testIMSI, cx.AssignmentReRegistration)
	reRegister.UserDataAlreadyAvailable = true

	ans = serverAssignment(t, store, reRegister)
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	if sa, err := cx.ParseServerAssignmentAnswer(ans); err != nil || len(sa.UserData) != 0 {
		t.Fatalf("re-registration SAA user data %d octets, err %v", len(sa.UserData), err)
	}

	dereg := uar
	dereg.AuthorizationType = cx.AuthorizationDeregistration

	ans = userAuthorization(t, store, dereg)
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	if name := uarServerName(t, ans); name != testSCSCF {
		t.Fatalf("deregistration UAA server name = %q", name)
	}

	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentUserDeregistration)), tgpp.Result{Code: diameter.ResultSuccess})
	requireRegistration(t, store, nil)

	requireResult(t, userAuthorization(t, store, dereg), tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered))
	requireResult(t, userAuthorization(t, store, uar), tgpp.Experimental(tgpp.ResultFirstRegistration))
}

func TestServerAssignmentFromAnotherSCSCFIsRefused(t *testing.T) {
	store := newFakeIMSStore()

	challenge(t, store, testSCSCF)
	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultSuccess})

	for _, typ := range []cx.AssignmentType{cx.AssignmentRegistration, cx.AssignmentTimeoutDeregistration, cx.AssignmentAuthenticationFailure} {
		r := register(testIMSI, typ)
		r.ServerName = otherSCSCF

		ans := serverAssignment(t, store, r)
		requireResult(t, ans, tgpp.Experimental(tgpp.ResultErrorIdentityAlreadyRegistered))

		if name, ok := ans.Find(cx.AVPServerName, tgpp.VendorID); !ok || name.UTF8String() != testSCSCF {
			t.Fatalf("%s: 5005 without the assigned Server-Name", typ)
		}
	}

	requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: testSCSCF})
}

func TestChallengeFromAnotherSCSCFMovesTheAssignment(t *testing.T) {
	store := newFakeIMSStore()

	challenge(t, store, testSCSCF)
	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultSuccess})

	challenge(t, store, otherSCSCF)
	requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: otherSCSCF, AuthPending: true})

	stale := register(testIMSI, cx.AssignmentTimeoutDeregistration)
	requireResult(t, serverAssignment(t, store, stale), tgpp.Experimental(tgpp.ResultErrorIdentityAlreadyRegistered))

	moved := register(testIMSI, cx.AssignmentRegistration)
	moved.ServerName = otherSCSCF

	requireResult(t, serverAssignment(t, store, moved), tgpp.Result{Code: diameter.ResultSuccess})
	requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: otherSCSCF})
}

func TestChallengeOfARegisteredUserKeepsTheRegistration(t *testing.T) {
	store := newFakeIMSStore()

	challenge(t, store, testSCSCF)
	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultSuccess})

	challenge(t, store, testSCSCF)
	requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: testSCSCF})
}

func TestAuthenticationFailure(t *testing.T) {
	for _, typ := range []cx.AssignmentType{cx.AssignmentAuthenticationFailure, cx.AssignmentAuthenticationTimeout} {
		t.Run(typ.String(), func(t *testing.T) {
			store := newFakeIMSStore()

			challenge(t, store, testSCSCF)
			requireResult(t, serverAssignment(t, store, register(testIMSI, typ)), tgpp.Result{Code: diameter.ResultSuccess})
			requireRegistration(t, store, nil)

			challenge(t, store, testSCSCF)
			requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultSuccess})
			challenge(t, store, otherSCSCF)

			r := register(testIMSI, typ)
			r.ServerName = otherSCSCF

			requireResult(t, serverAssignment(t, store, r), tgpp.Result{Code: diameter.ResultSuccess})
			requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: otherSCSCF})
		})
	}
}

func TestDeregistrationKeepingTheServerNameIsNotStored(t *testing.T) {
	store := newFakeIMSStore()

	challenge(t, store, testSCSCF)
	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultSuccess})

	r := cx.ServerAssignmentRequest{PrivateIdentity: impiOf(testIMSI), Type: cx.AssignmentUserDeregistrationStoreServer}

	requireResult(t, serverAssignment(t, store, r), tgpp.Experimental(tgpp.ResultSuccessServerNameNotStored))
	requireRegistration(t, store, nil)
}

func TestDeregistrationOfAnUnregisteredUserSucceeds(t *testing.T) {
	store := newFakeIMSStore()

	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentTimeoutDeregistration)), tgpp.Result{Code: diameter.ResultSuccess})
	requireRegistration(t, store, nil)
}

func TestNoAssignment(t *testing.T) {
	store := newFakeIMSStore()
	r := cx.ServerAssignmentRequest{PublicIdentities: []string{"tel:+" + testMSISDN}, Type: cx.AssignmentNoAssignment}

	requireResult(t, serverAssignment(t, store, r), tgpp.Result{Code: diameter.ResultUnableToComply})

	challenge(t, store, testSCSCF)
	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultSuccess})

	ans := serverAssignment(t, store, r)
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	if sa, err := cx.ParseServerAssignmentAnswer(ans); err != nil || len(sa.UserData) == 0 {
		t.Fatalf("NO_ASSIGNMENT answer without User-Data: %v", err)
	}

	requireRegistration(t, store, &hss.Registration{State: hss.Registered, ServerName: testSCSCF})
}

func TestUnregisteredUser(t *testing.T) {
	store := newFakeIMSStore()
	r := cx.ServerAssignmentRequest{PublicIdentities: []string{"tel:+" + testMSISDN}, Type: cx.AssignmentUnregisteredUser}

	ans := serverAssignment(t, store, r)
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	if sa, err := cx.ParseServerAssignmentAnswer(ans); err != nil || sa.PrivateIdentity != impiOf(testIMSI) || len(sa.UserData) == 0 {
		t.Fatalf("UNREGISTERED_USER answer %+v, %v", sa, err)
	}

	requireRegistration(t, store, &hss.Registration{State: hss.Unregistered, ServerName: testSCSCF})

	ans = userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: "tel:+" + testMSISDN})
	requireResult(t, ans, tgpp.Experimental(tgpp.ResultSubsequentRegistration))
}

func TestServerAssignmentIdentityErrors(t *testing.T) {
	cases := []struct {
		name string
		req  cx.ServerAssignmentRequest
		want tgpp.Result
	}{
		{
			name: "unknown subscriber",
			req:  register("001010000000099", cx.AssignmentRegistration),
			want: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		},
		{
			name: "public identity of another subscriber",
			req:  cx.ServerAssignmentRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentities: []string{"tel:+" + testOtherTel}, Type: cx.AssignmentRegistration},
			want: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeIMSStore()

			requireResult(t, serverAssignment(t, store, tc.req), tc.want)

			if len(store.registrations) != 0 {
				t.Fatalf("registrations changed: %+v", store.registrations)
			}
		})
	}
}

func TestServerAssignmentStoreFailureIsUnableToComply(t *testing.T) {
	store := newFakeIMSStore()
	store.fail = true

	requireResult(t, serverAssignment(t, store, register(testIMSI, cx.AssignmentRegistration)), tgpp.Result{Code: diameter.ResultUnableToComply})
}

func TestSubscriberWithoutMSISDN(t *testing.T) {
	store := newFakeIMSStore()

	ans := userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testNoMSISDN), PublicIdentity: "sip:" + impiOf(testNoMSISDN)})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultAuthorizationRejected})
}
