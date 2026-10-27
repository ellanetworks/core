// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"errors"
	"testing"
)

const (
	testRand  = "0123456789abcdef0123456789abcdef"
	testAutn  = "fedcba9876543210fedcba9876543210"
	testHxres = "00112233445566778899aabbccddeeff"
	testAuts  = "0102030405060708090a0b0c0d0e"
	testAv    = `{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"}`
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
		"no supiOrSuci":  func(a *AuthenticationInfo) { a.SupiOrSuci = "" },
		"bad supiOrSuci": func(a *AuthenticationInfo) { a.SupiOrSuci = "x" },
		"bad SNN":        func(a *AuthenticationInfo) { a.ServingNetworkName = "5G:mnc70.mcc999.3gppnetwork.org" },
		"lower NID":      func(a *AuthenticationInfo) { a.ServingNetworkName += ":000007ed9d5" },
		"short AUTS": func(a *AuthenticationInfo) {
			a.ResynchronizationInfo = &ResynchronizationInfo{Rand: testRand, Auts: "01"}
		},
		"no RAND":      func(a *AuthenticationInfo) { a.ResynchronizationInfo = &ResynchronizationInfo{Auts: testAuts} },
		"RI too long":  func(a *AuthenticationInfo) { a.RoutingIndicator = "12345" },
		"bad PEI":      func(a *AuthenticationInfo) { a.Pei = "imei-1" },
		"bad features": func(a *AuthenticationInfo) { a.SupportedFeatures = "x" },
	} {
		a := in
		mutate(&a)

		if _, err := Encode(a); err == nil {
			t.Errorf("%s: encoded", name)
		}
	}

	requireParams[AuthenticationInfo](t, []paramCase{
		{`{"servingNetworkName":"x"}`, "/supiOrSuci"},
		{`{"supiOrSuci":"x"}`, "/servingNetworkName"},
	})
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

func TestUEAuthenticationCtxSingleLink(t *testing.T) {
	body := `{"authType":"5G_AKA","5gAuthData":` + testAv + `,` +
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

	if err := ctx.Check(Strict); err != nil {
		t.Fatal(err)
	}
}

func TestUEAuthenticationCtxShapes(t *testing.T) {
	arrayLinks := `{"authType":"5G_AKA","5gAuthData":` + testAv + `,` +
		`"_links":{"5g-aka":[{"href":"https://ausf.example.com/a"},{"href":"https://ausf.example.com/b"}]},` +
		`"servingNetworkName":"5G:mnc070.mcc999.3gppnetwork.org"}`

	ctx, err := Decode[UEAuthenticationCtx]([]byte(arrayLinks))
	if err != nil {
		t.Fatal(err)
	}

	if href, _ := ctx.Href(LinkRel5GAKA); href != "https://ausf.example.com/a" || len(ctx.Links[LinkRel5GAKA]) != 2 {
		t.Fatalf("array links = %+v", ctx.Links)
	}

	if err := ctx.Check(Strict); err != nil {
		t.Fatal(err)
	}

	eap, err := Decode[UEAuthenticationCtx]([]byte(`{"authType":"EAP_AKA_PRIME","5gAuthData":"AQIDBA==","_links":{"eap-session":{"href":"https://ausf.example.com/x"}}}`))
	if err != nil || eap.AuthType != AuthTypeEAPAKAPrime || string(eap.AuthData.EapPayload) != "\x01\x02\x03\x04" {
		t.Fatalf("EAP = %+v, %v", eap, err)
	}

	if _, ok := eap.Href(LinkRel5GAKA); ok {
		t.Fatal("5g-aka link on EAP context")
	}

	unknown, err := Decode[UEAuthenticationCtx]([]byte(`{"authType":"FUTURE_AKA","5gAuthData":"AA==","_links":{}}`))
	if err != nil || unknown.AuthType.Known() {
		t.Fatalf("unknown authType = %+v, %v", unknown, err)
	}

	if unknown.Check(Strict) == nil {
		t.Fatal("strict accepted unknown authType")
	}

	nullLinks, err := Decode[UEAuthenticationCtx]([]byte(`{"authType":"EAP_TLS","5gAuthData":"AA==","_links":{"eap-session":null}}`))
	if err != nil || nullLinks.Check(Strict) == nil {
		t.Fatalf("null link value = %+v, %v", nullLinks, err)
	}

	requireParams[UEAuthenticationCtx](t, []paramCase{
		{`{"5gAuthData":"AA==","_links":{}}`, "/authType"},
		{`{"authType":"5G_AKA","_links":{}}`, "/5gAuthData"},
		{`{"authType":"5G_AKA","5gAuthData":null,"_links":{}}`, "/5gAuthData"},
		{`{"authType":"5G_AKA","5gAuthData":` + testAv + `}`, "/_links"},
		{`{"authType":"5G_AKA","5gAuthData":"AA==","_links":{}}`, "/5gAuthData"},
		{`{"authType":"EAP_AKA_PRIME","5gAuthData":` + testAv + `,"_links":{}}`, "/5gAuthData"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"00","hxresStar":"` + testHxres + `","autn":"` + testAutn + `"},"_links":{}}`, "/5gAuthData/rand"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","autn":"` + testAutn + `"},"_links":{}}`, "/5gAuthData/hxresStar"},
		{`{"authType":"5G_AKA","5gAuthData":{"rand":"` + testRand + `","hxresStar":"` + testHxres + `","autn":"zz` + testAutn[2:] + `"},"_links":{}}`, "/5gAuthData/autn"},
		{`{"authType":"5G_AKA","5gAuthData":` + testAv + `,"_links":{"eap-session":{"href":"x"}}}`, "/_links/5g-aka"},
		{`{"authType":"5G_AKA","5gAuthData":` + testAv + `,"_links":{"5g-aka":{"href":"/relative"}}}`, "/_links/5g-aka/href"},
	})

	for _, body := range []string{
		`{"authType":"5G_AKA","5gAuthData":7,"_links":{}}`,
		`{"authType":"5G_AKA","5gAuthData":{"rand":1},"_links":{}}`,
		`{"authType":"5G_AKA","5gAuthData":` + testAv + `,"_links":{"5g-aka":[1]}}`,
		`{"authType":"5G_AKA","5gAuthData":` + testAv + `,"_links":{"5g-aka":7}}`,
	} {
		if _, err := Decode[UEAuthenticationCtx]([]byte(body)); !errors.Is(err, ErrMalformed) {
			t.Errorf("Decode(%s) = %v", body, err)
		}
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
	suci := "suci-0-001-01-0000-0-0-0000000001"

	r, err := Decode[ConfirmationDataResponse]([]byte(`{"authResult":"AUTHENTICATION_SUCCESS","supi":"imsi-001010000000001","kseaf":"` + kseaf + `"}`))
	if err != nil || r.AuthResult != AuthResultSuccess || r.Supi != "imsi-001010000000001" {
		t.Fatalf("success = %+v, %v", r, err)
	}

	supi, k, err := r.Success(suci)
	if err != nil || supi != "imsi-001010000000001" || k[0] != 0x01 || k[31] != 0x10 {
		t.Fatalf("Success = %q %x, %v", supi, k, err)
	}

	if supi, _, err := r.Success("imsi-001010000000001"); err != nil || supi != "imsi-001010000000001" {
		t.Fatalf("Success(SUPI) = %q, %v", supi, err)
	}

	noSupi := ConfirmationDataResponse{AuthResult: AuthResultSuccess, Kseaf: kseaf}
	if supi, _, err := noSupi.Success("imsi-001010000000002"); err != nil || supi != "imsi-001010000000002" {
		t.Fatalf("SUPI sent, none returned = %q, %v", supi, err)
	}

	for name, tc := range map[string]struct {
		r          ConfirmationDataResponse
		supiOrSuci string
	}{
		"SUCI sent, no SUPI":   {noSupi, suci},
		"other SUPI":           {r, "imsi-001010000000002"},
		"SUPI of another PLMN": {ConfirmationDataResponse{AuthResult: AuthResultSuccess, Kseaf: kseaf, Supi: "imsi-999700000000001"}, suci},
		"bad SUCI":             {r, "suci-x"},
		"no Kseaf":             {ConfirmationDataResponse{AuthResult: AuthResultSuccess, Supi: "imsi-001010000000001"}, suci},
		"failure":              {ConfirmationDataResponse{AuthResult: AuthResultFailure}, suci},
	} {
		if _, _, err := tc.r.Success(tc.supiOrSuci); err == nil {
			t.Errorf("%s: Success accepted", name)
		}
	}

	if _, _, err := (ConfirmationDataResponse{AuthResult: AuthResultFailure}).Success(suci); !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("failure = %v", err)
	}

	failure, err := Decode[ConfirmationDataResponse]([]byte(`{"authResult":"AUTHENTICATION_FAILURE"}`))
	if err != nil || failure.AuthResult != AuthResultFailure {
		t.Fatalf("failure without kseaf = %+v, %v", failure, err)
	}

	if r, err := Decode[ConfirmationDataResponse]([]byte(`{"authResult":"AUTHENTICATION_MAYBE"}`)); err != nil || r.Check(Strict) == nil {
		t.Fatalf("unknown authResult = %v", err)
	}

	requireParams[ConfirmationDataResponse](t, []paramCase{
		{`{}`, "/authResult"},
		{`{"authResult":"AUTHENTICATION_SUCCESS","supi":"imsi-001010000000001"}`, "/kseaf"},
		{`{"authResult":"AUTHENTICATION_SUCCESS","kseaf":"00"}`, "/kseaf"},
		{`{"authResult":"AUTHENTICATION_SUCCESS","kseaf":"` + kseaf + `","supi":"bogus"}`, "/supi"},
	})
}
