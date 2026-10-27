// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"testing"
)

const (
	testRand  = "0123456789abcdef0123456789abcdef"
	testAutn  = "fedcba9876543210fedcba9876543210"
	testHxres = "00112233445566778899aabbccddeeff"
	testAuts  = "0102030405060708090a0b0c0d0e"
)

func TestAuthenticationInfoEncoding(t *testing.T) {
	op := PlmnID{Mcc: "999", Mnc: "70"}
	in := AuthenticationInfo{
		SupiOrSuci:         "suci-0-001-01-0000-0-0-0000000001",
		ServingNetworkName: op.ServingNetworkName(),
		RoutingIndicator:   "0",
	}

	b, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"supiOrSuci":"suci-0-001-01-0000-0-0-0000000001","servingNetworkName":"5G:mnc070.mcc999.3gppnetwork.org","routingIndicator":"0"}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}

	var rand [16]byte

	var auts [14]byte

	copy(auts[:], []byte{1, 2, 3})

	in.ResynchronizationInfo = NewResynchronizationInfo(rand, auts)

	b, err = Encode(in)
	if err != nil {
		t.Fatal(err)
	}

	back, err := Decode[AuthenticationInfo](b)
	if err != nil || *back.ResynchronizationInfo != *in.ResynchronizationInfo || back.ResynchronizationInfo.Auts != "0102030000000000000000000000" {
		t.Fatalf("round trip = %+v, %v", back.ResynchronizationInfo, err)
	}

	for name, mutate := range map[string]func(*AuthenticationInfo){
		"no supiOrSuci": func(a *AuthenticationInfo) { a.SupiOrSuci = "" },
		"bad SNN":       func(a *AuthenticationInfo) { a.ServingNetworkName = "5G:mnc70.mcc999.3gppnetwork.org" },
		"lower NID":     func(a *AuthenticationInfo) { a.ServingNetworkName += ":000007ed9d5" },
		"short AUTS": func(a *AuthenticationInfo) {
			a.ResynchronizationInfo = &ResynchronizationInfo{Rand: testRand, Auts: "01"}
		},
		"no RAND":     func(a *AuthenticationInfo) { a.ResynchronizationInfo = &ResynchronizationInfo{Auts: testAuts} },
		"RI too long": func(a *AuthenticationInfo) { a.RoutingIndicator = "12345" },
		"bad PEI":     func(a *AuthenticationInfo) { a.Pei = "imei-1" },
	} {
		a := in
		mutate(&a)

		if _, err := Encode(a); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}
}

func TestServingNetworkName(t *testing.T) {
	sn, err := ParseServingNetworkName("5G:mnc070.mcc999.3gppnetwork.org")
	if err != nil || sn != (ServingNetwork{Mcc: "999", Mnc: "070"}) {
		t.Fatalf("Parse = %+v, %v", sn, err)
	}

	if !sn.Matches(PlmnID{Mcc: "999", Mnc: "70"}) || !sn.Matches(PlmnID{Mcc: "999", Mnc: "070"}) || sn.Matches(PlmnID{Mcc: "999", Mnc: "71"}) {
		t.Fatal("Matches")
	}

	sn, err = ParseServingNetworkName("5G:mnc070.mcc999.3gppnetwork.org:000007ED9D5")
	if err != nil || sn.Nid != "000007ED9D5" || sn.Matches(PlmnID{Mcc: "999", Mnc: "70"}) {
		t.Fatalf("SNPN = %+v, %v", sn, err)
	}

	for _, s := range []string{"", "5G:mnc70.mcc999.3gppnetwork.org", "4G:mnc070.mcc999.3gppnetwork.org", "5G:mnc070.mcc999.3gppnetwork.org:000007ed9d5"} {
		if _, err := ParseServingNetworkName(s); err == nil {
			t.Errorf("Parse(%q) accepted", s)
		}
	}
}

func TestUEAuthenticationCtxOpen5GS(t *testing.T) {
	body := `{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"},` +
		`"_links":{"5g-aka":{"href":"http://ausf.5gc.mnc001.mcc001.3gppnetwork.org/nausf-auth/v1/ue-authentications/1/5g-aka-confirmation"}}}`

	ctx, err := Decode[UEAuthenticationCtx]([]byte(body))
	if err != nil {
		t.Fatal(err)
	}

	href, ok := ctx.Href(LinkRel5GAKA)
	if !ok || href != "http://ausf.5gc.mnc001.mcc001.3gppnetwork.org/nausf-auth/v1/ue-authentications/1/5g-aka-confirmation" {
		t.Fatalf("Href = %q, %v", href, ok)
	}

	rand, hxres, autn, err := ctx.AuthData.Av5gAka.Vector()
	if err != nil || rand[0] != 0x01 || hxres[15] != 0xff || autn[0] != 0xfe {
		t.Fatalf("Vector = %x %x %x, %v", rand, hxres, autn, err)
	}

	b, err := Encode(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != body {
		t.Fatalf("re-encode\n%s\n%s", b, body)
	}
}

func TestUEAuthenticationCtxShapes(t *testing.T) {
	arrayLinks := `{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"},` +
		`"_links":{"5g-aka":[{"href":"https://ausf.example.com/a"},{"href":"https://ausf.example.com/b"}]},` +
		`"servingNetworkName":"5G:mnc070.mcc999.3gppnetwork.org"}`

	ctx, err := Decode[UEAuthenticationCtx]([]byte(arrayLinks))
	if err != nil {
		t.Fatal(err)
	}

	if href, _ := ctx.Href(LinkRel5GAKA); href != "https://ausf.example.com/a" || len(ctx.Links[LinkRel5GAKA]) != 2 {
		t.Fatalf("array links = %+v", ctx.Links)
	}

	b, _ := json.Marshal(ctx)
	if back, err := Decode[UEAuthenticationCtx](b); err != nil || len(back.Links[LinkRel5GAKA]) != 2 {
		t.Fatalf("array round trip = %s, %v", b, err)
	}

	eap, err := Decode[UEAuthenticationCtx]([]byte(`{"authType":"EAP_AKA_PRIME","5gAuthData":"AQIDBA==","_links":{"eap-session":{"href":"https://ausf.example.com/x"}}}`))
	if err != nil || eap.AuthType != AuthTypeEAPAKAPrime || string(eap.AuthData.EapPayload) != "\x01\x02\x03\x04" {
		t.Fatalf("EAP = %+v, %v", eap, err)
	}

	unknown, err := Decode[UEAuthenticationCtx]([]byte(`{"authType":"FUTURE_AKA","5gAuthData":"AA==","_links":{}}`))
	if err != nil || unknown.AuthType.Known() {
		t.Fatalf("unknown authType = %+v, %v", unknown, err)
	}

	if unknown.Check(Strict) == nil {
		t.Fatal("strict accepted unknown authType")
	}

	for _, tc := range []struct{ body, param string }{
		{`{"5gAuthData":"AA==","_links":{}}`, "/authType"},
		{`{"authType":"5G_AKA","_links":{}}`, "/5gAuthData"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"}}`, "/_links"},
		{`{"authType":"5G_AKA","5gAuthData":"AA==","_links":{}}`, "/5gAuthData"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"00","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"},"_links":{}}`, "/5gAuthData/rand"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","autn":"` + testAutn + `"},"_links":{}}`, "/5gAuthData/hxresStar"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"zz` + testAutn[2:] + `"},"_links":{}}`, "/5gAuthData/autn"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"},"_links":{"eap-session":{"href":"x"}}}`, "/_links/5g-aka"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"},"_links":{"5g-aka":{"href":"/relative"}}}`, "/_links/5g-aka/href"},
	} {
		_, err := Decode[UEAuthenticationCtx]([]byte(tc.body))

		var ie *IEError
		if !errors.As(err, &ie) || ie.Param != tc.param {
			t.Errorf("Decode(%s) = %v, want %s", tc.body, err, tc.param)
		}
	}

	if _, err := Decode[UEAuthenticationCtx]([]byte(`{"authType":"5G_AKA","5gAuthData":7,"_links":{}}`)); !errors.Is(err, ErrMalformed) {
		t.Fatalf("numeric 5gAuthData: %v", err)
	}
}

func TestConfirmationData(t *testing.T) {
	b, err := Encode(ConfirmationData{})
	if err != nil || string(b) != `{"resStar":null}` {
		t.Fatalf("null RES* = %s, %v", b, err)
	}

	res := [16]byte{0xab}

	b, err = Encode(ConfirmationWithResStar(res))
	if err != nil || string(b) != `{"resStar":"ab000000000000000000000000000000"}` {
		t.Fatalf("RES* = %s, %v", b, err)
	}

	back, err := Decode[ConfirmationData]([]byte(`{"resStar":null}`))
	if err != nil || back.ResStar != nil {
		t.Fatalf("decode null = %+v, %v", back, err)
	}

	short := "abcd"
	if _, err := Encode(ConfirmationData{ResStar: &short}); err == nil {
		t.Fatal("short RES* encoded")
	}

	if _, err := Encode(ConfirmationData{SupportedFeatures: "zz"}); err == nil {
		t.Fatal("bad features encoded")
	}
}

func TestConfirmationDataResponse(t *testing.T) {
	kseaf := testRand + testAutn

	r, err := Decode[ConfirmationDataResponse]([]byte(`{"authResult":"AUTHENTICATION_SUCCESS","supi":"imsi-001010000000001","kseaf":"` + kseaf + `"}`))
	if err != nil || r.AuthResult != AuthResultSuccess || r.Supi != "imsi-001010000000001" {
		t.Fatalf("success = %+v, %v", r, err)
	}

	k, err := r.KseafBytes()
	if err != nil || k[0] != 0x01 || k[31] != 0x10 {
		t.Fatalf("Kseaf = %x, %v", k, err)
	}

	failure, err := Decode[ConfirmationDataResponse]([]byte(`{"authResult":"AUTHENTICATION_FAILURE"}`))
	if err != nil || failure.AuthResult != AuthResultFailure {
		t.Fatalf("Open5GS failure = %+v, %v", failure, err)
	}

	if _, err := failure.KseafBytes(); err == nil {
		t.Fatal("absent Kseaf decoded")
	}

	if r, err := Decode[ConfirmationDataResponse]([]byte(`{"authResult":"AUTHENTICATION_MAYBE"}`)); err != nil || r.Check(Strict) == nil {
		t.Fatalf("unknown authResult = %v", err)
	}

	for _, body := range []string{`{}`, `{"authResult":"AUTHENTICATION_SUCCESS","kseaf":"00"}`} {
		if _, err := Decode[ConfirmationDataResponse]([]byte(body)); err == nil {
			t.Errorf("Decode(%s) accepted", body)
		}
	}

	if _, err := Encode(ConfirmationDataResponse{AuthResult: AuthResultSuccess, Supi: "bogus"}); err == nil {
		t.Fatal("bad SUPI encoded")
	}
}
