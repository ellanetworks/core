// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/hss"
)

var (
	mmtelIdentity = "sip:+" + testMSISDN + "@" + testIMSDomain
	mcpttIdentity = "sip:mcptt-" + testMSISDN + "@" + testIMSDomain
)

func challengeIdentity(t *testing.T, store *fakeIMSStore, impu string) {
	t.Helper()

	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: impu, ServerName: testSCSCF})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})
}

func assignIdentity(t *testing.T, store *fakeIMSStore, impu string, typ cx.AssignmentType, want tgpp.Result) []byte {
	t.Helper()

	ans := serverAssignment(t, store, cx.ServerAssignmentRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentities: []string{impu}, Type: typ})
	requireResult(t, ans, want)

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		t.Fatalf("parse SAA: %v", err)
	}

	return sa.UserData
}

func registerBoth(t *testing.T, store *fakeIMSStore) {
	t.Helper()

	for _, impu := range []string{mmtelIdentity, mcpttIdentity} {
		challengeIdentity(t, store, impu)
		assignIdentity(t, store, impu, cx.AssignmentRegistration, tgpp.Result{Code: diameter.ResultSuccess})
	}
}

func authorizeIdentity(t *testing.T, store *fakeIMSStore, impu string, typ cx.AuthorizationType) *diameter.Message {
	t.Helper()

	return userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: impu, AuthorizationType: typ})
}

func TestRegistrationSetsAreIndependent_TS29228_6_5_1(t *testing.T) {
	store := newFakeIMSStore()
	store.mcpttFilterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}
	success := tgpp.Result{Code: diameter.ResultSuccess}

	requireResult(t, authorizeIdentity(t, store, mcpttIdentity, cx.AuthorizationRegistration), tgpp.Experimental(tgpp.ResultFirstRegistration))

	challengeIdentity(t, store, mmtelIdentity)
	assignIdentity(t, store, mmtelIdentity, cx.AssignmentRegistration, success)

	mmtel, _ := registration(t, store, testIMSI)

	ans := authorizeIdentity(t, store, mcpttIdentity, cx.AuthorizationRegistration)
	requireResult(t, ans, tgpp.Experimental(tgpp.ResultSubsequentRegistration))

	if name := uarServerName(t, ans); name != testSCSCF {
		t.Fatalf("UAA server name for the MCPTT identity = %q, want the S-CSCF of the subscription", name)
	}

	requireResult(t, authorizeIdentity(t, store, mcpttIdentity, cx.AuthorizationDeregistration), tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered))

	challengeIdentity(t, store, mcpttIdentity)
	requireRegistration(t, store, &hss.Registration{ServerName: testSCSCF, Sets: sets{
		hss.MMTel: {State: hss.Registered},
		hss.MCPTT: {State: hss.NotRegistered, AuthPending: true},
	}})
	requireResult(t, authorizeIdentity(t, store, mcpttIdentity, cx.AuthorizationDeregistration), success)

	userData := assignIdentity(t, store, mcpttIdentity, cx.AssignmentRegistration, success)

	profile, err := cx.ParseUserData(userData)
	if err != nil {
		t.Fatalf("parse User-Data: %v", err)
	}

	if len(profile.ServiceProfiles) != 1 || len(profile.ServiceProfiles[0].PublicIdentities) != 1 ||
		profile.ServiceProfiles[0].PublicIdentities[0].Identity != mcpttIdentity || len(profile.ServiceProfiles[0].InitialFilterCriteria) != 1 {
		t.Fatalf("MCPTT User-Data = %+v, want the MCPTT identity alone with its iFC", profile)
	}

	both, _ := registration(t, store, testIMSI)
	if both.Sets[hss.MMTel] != mmtel.Sets[hss.MMTel] || both.ServerName != mmtel.ServerName {
		t.Fatalf("registering the MCPTT set changed the MMTel one: %+v, was %+v", both, mmtel)
	}

	if s := both.Sets[hss.MCPTT]; s.State != hss.Registered || s.DefaultIdentity != mcpttIdentity || s.ProfileDigest == "" {
		t.Fatalf("MCPTT set = %+v, want registered with what the SAA delivered", s)
	}

	assignIdentity(t, store, mcpttIdentity, cx.AssignmentUserDeregistration, success)

	if after, _ := registration(t, store, testIMSI); !sameRegistration(after, mmtel) {
		t.Fatalf("de-registering the MCPTT set left %+v, want the MMTel registration %+v", after, mmtel)
	}

	requireResult(t, authorizeIdentity(t, store, mcpttIdentity, cx.AuthorizationDeregistration), tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered))
	requireResult(t, authorizeIdentity(t, store, mmtelIdentity, cx.AuthorizationDeregistration), success)

	challengeIdentity(t, store, mcpttIdentity)
	assignIdentity(t, store, mcpttIdentity, cx.AssignmentRegistration, success)
	assignIdentity(t, store, mmtelIdentity, cx.AssignmentUserDeregistrationStoreServer, success)
	requireRegistration(t, store, &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MCPTT: {State: hss.Registered}}})

	requireResult(t, serverAssignment(t, store, cx.ServerAssignmentRequest{PrivateIdentity: impiOf(testIMSI), Type: cx.AssignmentUserDeregistrationStoreServer}),
		tgpp.Experimental(tgpp.ResultSuccessServerNameNotStored))
	requireRegistration(t, store, nil)
}

func TestDeregistrationOfThePrivateIdentityEndsEverySet_TS29228_6_5_1_2(t *testing.T) {
	store := newFakeIMSStore()
	registerBoth(t, store)

	requireResult(t, serverAssignment(t, store, cx.ServerAssignmentRequest{PrivateIdentity: impiOf(testIMSI), Type: cx.AssignmentTimeoutDeregistration}),
		tgpp.Result{Code: diameter.ResultSuccess})
	requireRegistration(t, store, nil)
}

func TestAuthenticationOfOneSetLeavesTheOther_TS29228_6_5_1_3(t *testing.T) {
	store := newFakeIMSStore()

	challengeIdentity(t, store, mmtelIdentity)
	assignIdentity(t, store, mmtelIdentity, cx.AssignmentRegistration, tgpp.Result{Code: diameter.ResultSuccess})

	challengeIdentity(t, store, mcpttIdentity)
	challengeIdentity(t, store, mmtelIdentity)
	requireRegistration(t, store, &hss.Registration{ServerName: testSCSCF, Sets: sets{
		hss.MMTel: {State: hss.Registered},
		hss.MCPTT: {State: hss.NotRegistered, AuthPending: true},
	}})

	assignIdentity(t, store, mcpttIdentity, cx.AssignmentAuthenticationFailure, tgpp.Result{Code: diameter.ResultSuccess})
	requireRegistration(t, store, &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Registered}}})
}

func TestLocationInfoOfTheMCPTTIdentity_TS29228_6_2_1(t *testing.T) {
	store := newFakeIMSStore()
	terminating := cx.LocationInfoRequest{PublicIdentity: mcpttIdentity}

	challengeIdentity(t, store, mmtelIdentity)
	assignIdentity(t, store, mmtelIdentity, cx.AssignmentRegistration, tgpp.Result{Code: diameter.ResultSuccess})

	requireResult(t, locationInfo(t, store, cx.LocationInfoRequest{PublicIdentity: mmtelIdentity}), tgpp.Result{Code: diameter.ResultSuccess})
	requireResult(t, locationInfo(t, store, terminating), tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered))

	challengeIdentity(t, store, mcpttIdentity)
	assignIdentity(t, store, mcpttIdentity, cx.AssignmentRegistration, tgpp.Result{Code: diameter.ResultSuccess})

	ans := locationInfo(t, store, terminating)
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	if l, err := cx.ParseLocationInfoAnswer(ans); err != nil || l.ServerName != testSCSCF {
		t.Fatalf("LIA = %+v, %v; want the S-CSCF of the MCPTT registration", l, err)
	}

	for name, impu := range map[string]string{
		"unknown MSISDN": "sip:mcptt-15559990000@" + testIMSDomain,
		"another domain": "sip:mcptt-" + testMSISDN + "@ims.example.org",
		"no MSISDN":      "sip:mcptt-@" + testIMSDomain,
	} {
		t.Run(name, func(t *testing.T) {
			requireResult(t, locationInfo(t, newFakeIMSStore(), cx.LocationInfoRequest{PublicIdentity: impu}), tgpp.Experimental(tgpp.ResultErrorUserUnknown))
		})
	}

	ans = userAuthorization(t, newFakeIMSStore(), cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testOtherIMSI), PublicIdentity: mcpttIdentity})
	requireResult(t, ans, tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch))
}

func TestReconcileEachSet_TS29228_6_5_2_1(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{}
	h := connectedHSS(t, store, scscf)

	registerBoth(t, store)

	before, _ := registration(t, store, testIMSI)
	store.mcpttFilterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	pushes := scscf.Pushes()
	if len(pushes) != 1 {
		t.Fatalf("profile pushes = %d, want one for the MCPTT set", len(pushes))
	}

	ppr, err := cx.ParsePushProfileRequest(pushes[0])
	if err != nil {
		t.Fatalf("parse PPR: %v", err)
	}

	pushed, err := cx.ParseUserData(ppr.UserData)
	if err != nil || len(pushed.ServiceProfiles) != 1 || pushed.ServiceProfiles[0].PublicIdentities[0].Identity != mcpttIdentity {
		t.Fatalf("PPR user data = %+v, %v; want the MCPTT set alone", pushed, err)
	}

	after, _ := registration(t, store, testIMSI)
	if after.Sets[hss.MMTel] != before.Sets[hss.MMTel] || after.Sets[hss.MCPTT].ProfileDigest == before.Sets[hss.MCPTT].ProfileDigest {
		t.Fatalf("registration after the push = %+v, want the MCPTT digest alone changed from %+v", after, before)
	}

	store.subscribers[testIMSI].MSISDN = "15559990000"

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile after the MSISDN change: %v", err)
	}

	if got := scscf.Terminations(); !slices.Equal(got, []cx.ReasonCode{cx.ReasonServerChange, cx.ReasonServerChange}) {
		t.Fatalf("registration terminations = %v, want a server change per set", got)
	}

	if got, want := scscf.TerminatedIdentities(), [][]string{{mcpttIdentity}, {mmtelIdentity}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("terminated identities = %v, want %v", got, want)
	}

	requireRegistration(t, store, nil)
}

func TestTerminationOfOneSetKeepsTheOther_TS29228_6_1_3(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{pushResult: tgpp.Experimental(tgpp.ResultErrorNotSupportedUserData)}
	h := connectedHSS(t, store, scscf)

	registerBoth(t, store)
	store.mcpttFilterCriteria = map[string][]cx.InitialFilterCriteria{testIMSI: mcpttCriteria()}

	if err := h.Reconcile(context.Background(), testIMSI); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	if got, want := scscf.TerminatedIdentities(), [][]string{{mcpttIdentity}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("terminated identities = %v, want %v", got, want)
	}

	requireRegistration(t, store, &hss.Registration{ServerName: testSCSCF, Sets: sets{hss.MMTel: {State: hss.Registered}}})
}

func TestTerminationOfThePrivateIdentityKeepsTheEmergencySet_TS29228_6_1_3_1(t *testing.T) {
	store := newFakeIMSStore()
	scscf := &fakeSCSCF{
		rtrResult:    tgpp.Result{Code: diameter.ResultUnableToComply},
		rtrEmergency: []cx.EmergencyIdentity{{PrivateIdentity: impiOf(testIMSI), PublicIdentity: mmtelIdentity}},
	}
	h := connectedHSS(t, store, scscf)

	registerBoth(t, store)

	term, err := h.Termination(context.Background(), testIMSI)
	if err != nil || term == nil || len(term.PublicIdentities) != 0 {
		t.Fatalf("Termination = %+v, %v; want the private identity alone", term, err)
	}

	if err := h.Terminate(context.Background(), term); err != nil {
		t.Fatalf("Terminate: %v", err)
	}

	reg, ok := registration(t, store, testIMSI)
	if s := reg.Sets[hss.MMTel]; !ok || len(reg.Sets) != 1 || s.State != hss.Unregistered || !s.Emergency {
		t.Fatalf("registration = %+v (kept %t), want the emergency MMTel set alone", reg, ok)
	}
}
