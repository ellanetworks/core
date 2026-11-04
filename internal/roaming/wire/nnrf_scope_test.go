// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"testing"
	"time"
)

func TestSupiRangeContains(t *testing.T) {
	r := SupiRange{Start: "001010000000100", End: "001010000000199"}

	for supi, want := range map[string]bool{
		"imsi-001010000000100": true,
		"imsi-001010000000199": true,
		"imsi-001010000000200": false,
		"imsi-00101000000150":  false,
		"nai-x@example.org":    false,
	} {
		if got := r.Contains(supi); got != want {
			t.Errorf("%s: %v, want %v", supi, got, want)
		}
	}

	p := SupiRange{Pattern: "imsi-00101[0-9]{10}"}
	if !p.Contains("imsi-001010000000001") || p.Contains("imsi-001020000000001") || (SupiRange{Pattern: "("}).Contains("imsi-001010000000001") {
		t.Fatal("pattern ranges")
	}
}

func TestScopeProvesOnlyWithEvidence(t *testing.T) {
	group := Scope{GroupID: "g"}
	ranged := Scope{GroupID: "g", SupiRanges: []SupiRange{{Start: "001010000000000", End: "001010000000099"}}}
	rid := Scope{RoutingIndicators: []string{"12"}}

	cases := []struct {
		s    Scope
		sub  Subject
		want bool
	}{
		{Scope{}, Subject{Supi: "imsi-001010000000001"}, true},
		{group, Subject{Supi: "imsi-001010000000001"}, false},
		{group, Subject{Supi: "imsi-001010000000001", GroupID: "g"}, true},
		{group, Subject{RoutingIndicator: "0000"}, true},
		{ranged, Subject{Supi: "imsi-001010000000001"}, true},
		{ranged, Subject{Supi: "imsi-001010000000100"}, false},
		{rid, Subject{RoutingIndicator: "12"}, true},
		{rid, Subject{RoutingIndicator: "13"}, false},
	}

	for i, tc := range cases {
		if got := tc.s.Proves(tc.sub); got != tc.want {
			t.Errorf("case %d: %v, want %v", i, got, tc.want)
		}
	}
}

func TestOAuth2RequirementPrecedence(t *testing.T) {
	yes, no := true, false
	requester := PlmnID{Mcc: "999", Mnc: "70"}

	n := NFProfile{Oauth2Required: &yes}
	if !n.oauth2Required(NFService{}, requester) || n.oauth2Required(NFService{Oauth2Required: &no}, requester) {
		t.Fatal("the service indication must prevail over the profile")
	}

	per := &PlmnOauth2{Oauth2NotRequiredPlmnIDList: []PlmnID{requester}}
	if n.oauth2Required(NFService{Oauth2Required: &yes, PerPlmnOauth2ReqList: per}, requester) {
		t.Fatal("the per-PLMN list must prevail")
	}
}

func TestAccessTokenWire(t *testing.T) {
	home, serving := PlmnID{Mcc: "001", Mnc: "01"}, PlmnID{Mcc: "999", Mnc: "70"}

	v, err := AccessTokenReq{NfInstanceID: "00000000-0000-0000-0000-000000000001", NfType: NFTypeAMF, TargetNfType: NFTypeUDM, Scope: "nudm-sdm", RequesterPlmn: &serving, TargetPlmn: &home}.Values()
	if err != nil || v.Get("grant_type") != "client_credentials" || v.Get("targetPlmn") != `{"mcc":"001","mnc":"01"}` {
		t.Fatalf("%v, %v", v, err)
	}

	in := 120
	if d, ok := (AccessTokenRsp{AccessToken: "a", TokenType: "Bearer", ExpiresIn: &in}).Lifetime(time.Now()); !ok || d != 2*time.Minute {
		t.Fatalf("lifetime %v", d)
	}

	if e, ok := BearerChallenge([]string{`Bearer realm="nudm-sdm", error="invalid_token"`}); !ok || e != "invalid_token" {
		t.Fatalf("challenge %q %v", e, ok)
	}

	if _, ok := BearerChallenge([]string{`Basic realm="x"`}); ok {
		t.Fatal("a Basic challenge is not Bearer")
	}
}

func TestMSISDNFromGpsi(t *testing.T) {
	am := AccessAndMobilitySubscriptionData{Gpsis: []string{"extid-a@b", "msisdn-1234", "msisdn-15551230000"}}
	if m, ok := am.MSISDN(); !ok || m != "15551230000" {
		t.Fatalf("MSISDN %q %v", m, ok)
	}
}
