// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const testCustomDomain = "ims.example.com"

func registerWithIMPI(t *testing.T, store *fakeIMSStore, impi, impu string) *cx.IMSSubscription {
	t.Helper()

	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impi, PublicIdentity: impu, ServerName: testSCSCF})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	ans = serverAssignment(t, store, cx.ServerAssignmentRequest{PrivateIdentity: impi, PublicIdentities: []string{impu}, Type: cx.AssignmentRegistration})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		t.Fatalf("parse SAA: %v", err)
	}

	if sa.PrivateIdentity != impi {
		t.Fatalf("SAA User-Name = %q, want %q", sa.PrivateIdentity, impi)
	}

	profile, err := cx.ParseUserData(sa.UserData)
	if err != nil {
		t.Fatalf("parse User-Data: %v", err)
	}

	return &profile
}

func requirePublicIdentities(t *testing.T, profile *cx.IMSSubscription, want []cx.ProfileIdentity) {
	t.Helper()

	if len(profile.ServiceProfiles) != 1 || !slices.Equal(profile.ServiceProfiles[0].PublicIdentities, want) {
		t.Fatalf("public identities = %+v, want %+v", profile.ServiceProfiles, want)
	}
}

func TestConfiguredHomeDomainRegistersAnISIMIdentity(t *testing.T) {
	store := newFakeIMSStore()
	store.homeDomain = testCustomDomain

	impi := testIMSI + "@" + testCustomDomain
	profile := registerWithIMPI(t, store, impi, "sip:+"+testMSISDN+"@"+testCustomDomain)

	if profile.PrivateIdentity != impi {
		t.Fatalf("User-Data private identity = %q, want %q", profile.PrivateIdentity, impi)
	}

	requirePublicIdentities(t, profile, []cx.ProfileIdentity{
		{Identity: "sip:+" + testMSISDN + "@" + testCustomDomain},
		{Identity: "tel:+" + testMSISDN},
		{Identity: "sip:" + testIMSI + "@" + testIMSDomain, Barred: true},
	})
}

func TestConfiguredHomeDomainStillAcceptsTheDerivedDomain(t *testing.T) {
	store := newFakeIMSStore()
	store.homeDomain = testCustomDomain

	impi := impiOf(testIMSI)
	profile := registerWithIMPI(t, store, impi, "sip:"+impi)

	if profile.PrivateIdentity != impi {
		t.Fatalf("User-Data private identity = %q, want %q", profile.PrivateIdentity, impi)
	}

	requirePublicIdentities(t, profile, []cx.ProfileIdentity{
		{Identity: "sip:+" + testMSISDN + "@" + testCustomDomain},
		{Identity: "tel:+" + testMSISDN},
		{Identity: "sip:" + testIMSI + "@" + testIMSDomain, Barred: true},
	})
}

func TestConfiguredHomeDomainKeepsEachIdentityInItsDomain(t *testing.T) {
	store := newFakeIMSStore()
	store.homeDomain = testCustomDomain

	for impu, want := range map[string]tgpp.Result{
		"sip:+" + testMSISDN + "@" + testCustomDomain:   tgpp.Experimental(tgpp.ResultFirstRegistration),
		"sip:" + testIMSI + "@" + testIMSDomain:         tgpp.Experimental(tgpp.ResultFirstRegistration),
		"sip:+" + testMSISDN + "@" + testIMSDomain:      tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		"sip:mcptt-" + testMSISDN + "@" + testIMSDomain: tgpp.Experimental(tgpp.ResultErrorUserUnknown),
		"sip:" + testIMSI + "@" + testCustomDomain:      tgpp.Experimental(tgpp.ResultErrorUserUnknown),
	} {
		ans := userAuthorization(t, store, cx.UserAuthorizationRequest{PrivateIdentity: impiOf(testIMSI), PublicIdentity: impu})
		requireResult(t, ans, want)
	}
}

func TestServerAssignmentWithoutUserNameKeepsTheStoredPrivateIdentity(t *testing.T) {
	store := newFakeIMSStore()
	store.homeDomain = testCustomDomain

	impu := "sip:+" + testMSISDN + "@" + testCustomDomain
	impi := impiOf(testIMSI)
	registerWithIMPI(t, store, impi, impu)

	ans := serverAssignment(t, store, cx.ServerAssignmentRequest{PublicIdentities: []string{impu}, Type: cx.AssignmentNoAssignment})
	requireResult(t, ans, tgpp.Result{Code: diameter.ResultSuccess})

	sa, err := cx.ParseServerAssignmentAnswer(ans)
	if err != nil {
		t.Fatalf("parse SAA: %v", err)
	}

	profile, err := cx.ParseUserData(sa.UserData)
	if err != nil {
		t.Fatalf("parse User-Data: %v", err)
	}

	if sa.PrivateIdentity != impi || profile.PrivateIdentity != impi {
		t.Fatalf("SAA User-Name %q, User-Data private identity %q, want %q", sa.PrivateIdentity, profile.PrivateIdentity, impi)
	}

	reg, err := store.Registration(context.Background(), testIMSI)
	if err != nil || reg == nil || reg.PrivateIdentity != impi {
		t.Fatalf("stored registration = %+v, %v; want private identity %q", reg, err, impi)
	}
}

func TestUnknownHomeDomainIsRejected(t *testing.T) {
	store := newFakeIMSStore()
	store.homeDomain = testCustomDomain

	impi := testIMSI + "@ims.other.example"
	ans := multimediaAuth(t, store, cx.MultimediaAuthRequest{PrivateIdentity: impi, PublicIdentity: "sip:" + impi, ServerName: testSCSCF})
	requireResult(t, ans, tgpp.Experimental(tgpp.ResultErrorUserUnknown))
}
