// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"testing"
	"time"
)

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
}

func TestMSISDNFromGpsi(t *testing.T) {
	am := AccessAndMobilitySubscriptionData{Gpsis: []string{"extid-a@b", "msisdn-1234", "msisdn-15551230000"}}
	if m, ok := am.MSISDN(); !ok || m != "15551230000" {
		t.Fatalf("MSISDN %q %v", m, ok)
	}
}
