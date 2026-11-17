// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func fields(t *testing.T, s string) map[string]json.RawMessage {
	t.Helper()

	var f map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &f); err != nil {
		t.Fatal(err)
	}

	return f
}

func TestClaims(t *testing.T) {
	c := &Client{settings: valid()}

	for name, tc := range map[string]struct {
		in       string
		verified Verified
		groups   []string
		overage  bool
	}{
		"verified":               {`{"sub": "s", "email_verified": true}`, VerifiedTrue, nil, false},
		"verified as a string":   {`{"sub": "s", "email_verified": "true"}`, VerifiedTrue, nil, false},
		"not verified":           {`{"sub": "s", "email_verified": false}`, VerifiedFalse, nil, false},
		"not verified as string": {`{"sub": "s", "email_verified": "false"}`, VerifiedFalse, nil, false},
		"verified as a number":   {`{"sub": "s", "email_verified": 1}`, VerifiedFalse, nil, false},
		"no email_verified":      {`{"sub": "s"}`, VerifiedUnknown, nil, false},
		"email_verified null":    {`{"sub": "s", "email_verified": null}`, VerifiedUnknown, nil, false},
		"groups null":            {`{"sub": "s", "groups": null}`, VerifiedUnknown, nil, false},
		"one group":              {`{"sub": "s", "groups": "a"}`, VerifiedUnknown, []string{"a"}, false},
		"groups":                 {`{"sub": "s", "groups": ["a", "b"]}`, VerifiedUnknown, []string{"a", "b"}, false},
		"overage": {
			`{"sub": "s", "_claim_names": {"groups": "src1"}, "_claim_sources": {"src1": {"endpoint": "https://x"}}}`,
			VerifiedUnknown, nil, true,
		},
	} {
		cl, err := c.claims(fields(t, tc.in), ErrToken)
		if err != nil || cl.Subject != "s" || cl.EmailVerified != tc.verified || !slices.Equal(cl.Groups, tc.groups) ||
			cl.GroupsOverage != tc.overage {
			t.Errorf("%s: %+v, %v", name, cl, err)
		}
	}

	for name, in := range map[string]string{
		"groups as a number": `{"sub": "s", "groups": 1}`,
		"email as a number":  `{"sub": "s", "email": 1}`,
	} {
		if _, err := c.claims(fields(t, in), ErrToken); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestTrustsEmail(t *testing.T) {
	for name, tc := range map[string]struct {
		email    string
		verified Verified
		trust    bool
		want     bool
	}{
		"verified":                      {"a@example.com", VerifiedTrue, false, true},
		"not verified":                  {"a@example.com", VerifiedFalse, false, false},
		"not verified, trusting":        {"a@example.com", VerifiedFalse, true, false},
		"unknown":                       {"a@example.com", VerifiedUnknown, false, false},
		"unknown, trusting":             {"a@example.com", VerifiedUnknown, true, true},
		"no email, verified":            {"", VerifiedTrue, true, false},
		"a blank email, trusting":       {" ", VerifiedUnknown, true, false},
		"verified, trusting":            {"a@example.com", VerifiedTrue, true, true},
		"not verified as a string":      {"a@example.com", VerifiedFalse, true, false},
		"unknown, trusting, no address": {"", VerifiedUnknown, true, false},
	} {
		o := valid()
		o.TrustUnverifiedEmail = tc.trust

		c := &Client{settings: o}
		if got := c.TrustsEmail(Claims{Email: tc.email, EmailVerified: tc.verified}); got != tc.want {
			t.Errorf("%s: %t, want %t", name, got, tc.want)
		}
	}
}

func TestMergeUserInfo(t *testing.T) {
	c := &Client{settings: valid()}

	for name, tc := range map[string]struct {
		idToken, userInfo string
		want              Claims
	}{
		"everything from UserInfo": {
			`{"sub": "s"}`,
			`{"sub": "s", "email": "a@example.com", "email_verified": true, "groups": ["g"]}`,
			Claims{Subject: "s", Email: "a@example.com", EmailVerified: VerifiedTrue, Groups: []string{"g"}},
		},
		"the ID token first": {
			`{"sub": "s", "email": "a@example.com", "email_verified": false, "groups": ["g"]}`,
			`{"sub": "s", "email": "b@example.com", "email_verified": true, "groups": ["h"]}`,
			Claims{Subject: "s", Email: "a@example.com", EmailVerified: VerifiedFalse, Groups: []string{"g"}},
		},
		"email_verified for the same email": {
			`{"sub": "s", "email": "a@example.com"}`,
			`{"sub": "s", "email": "a@example.com", "email_verified": true}`,
			Claims{Subject: "s", Email: "a@example.com", EmailVerified: VerifiedTrue},
		},
		"email_verified for another email": {
			`{"sub": "s", "email": "a@example.com"}`,
			`{"sub": "s", "email": "b@example.com", "email_verified": true}`,
			Claims{Subject: "s", Email: "a@example.com", EmailVerified: VerifiedUnknown},
		},
		"no groups in the ID token": {
			`{"sub": "s", "email": "a@example.com", "email_verified": true, "groups": null}`,
			`{"sub": "s", "groups": ["g"]}`,
			Claims{Subject: "s", Email: "a@example.com", EmailVerified: VerifiedTrue, Groups: []string{"g"}},
		},
		"an overage in the ID token": {
			`{"sub": "s", "_claim_names": {"groups": "src1"}}`,
			`{"sub": "s", "groups": ["g"]}`,
			Claims{Subject: "s", GroupsOverage: true},
		},
	} {
		idToken := fields(t, tc.idToken)

		cl, err := c.claims(idToken, ErrToken)
		if err != nil {
			t.Fatal(err)
		}

		if !c.incomplete(cl, idToken) && name != "the ID token first" {
			t.Errorf("%s: the ID token is complete", name)
		}

		got, err := c.merge(cl, idToken, fields(t, tc.userInfo))
		if err != nil || got.Subject != tc.want.Subject || got.Email != tc.want.Email ||
			got.EmailVerified != tc.want.EmailVerified || !slices.Equal(got.Groups, tc.want.Groups) ||
			got.GroupsOverage != tc.want.GroupsOverage {
			t.Errorf("%s: %+v, %v, want %+v", name, got, err, tc.want)
		}
	}
}

func TestRoleForGivesTheHighestRole(t *testing.T) {
	c := &Client{settings: db.OIDC{GroupRoles: []db.GroupRole{
		{Group: "viewers", RoleID: db.RoleReadOnly},
		{Group: "noc", RoleID: db.RoleNetworkManager},
		{Group: "admins", RoleID: db.RoleAdmin},
	}}}

	tests := []struct {
		groups []string
		want   db.RoleID
		ok     bool
	}{
		{[]string{"viewers", "admins", "noc"}, db.RoleAdmin, true},
		{[]string{"viewers", "noc"}, db.RoleNetworkManager, true},
		{[]string{"viewers"}, db.RoleReadOnly, true},
		{[]string{"guests"}, 0, false},
		{nil, 0, false},
	}

	for _, tc := range tests {
		if got, ok := c.RoleFor(tc.groups); got != tc.want || ok != tc.ok {
			t.Errorf("RoleFor(%v) = %d, %t; want %d, %t", tc.groups, got, ok, tc.want, tc.ok)
		}
	}
}

func TestLogoutURL(t *testing.T) {
	c := &Client{
		settings:  db.OIDC{ExternalURL: "https://core.example.com", ClientID: "ella-core"},
		discovery: Discovery{EndSessionEndpoint: "https://idp.example.com/logout?ui=1"},
	}

	if got, want := c.LogoutURL(), "https://idp.example.com/logout?client_id=ella-core&post_logout_redirect_uri=https%3A%2F%2Fcore.example.com%2Flogin&ui=1"; got != want {
		t.Fatalf("LogoutURL() = %q, want %q", got, want)
	}

	c.discovery.EndSessionEndpoint = ""

	if got := c.LogoutURL(); got != "" {
		t.Fatalf("LogoutURL() without an end session endpoint = %q", got)
	}
}
