// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func octets(n int, b byte) []byte {
	return bytes.Repeat([]byte{b}, n)
}

func akaVector(seed byte) *AKAVector {
	return &AKAVector{RAND: octets(16, seed), AUTN: octets(16, seed+1), XRES: octets(8, seed+2), CK: octets(16, seed+3), IK: octets(16, seed+4)}
}

func TestMultimediaAuthRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]MultimediaAuthRequest{
		"initial": {PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5},
		"unknown scheme": {
			PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 3, Scheme: "unknown",
		},
		"resync": {
			PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5,
			Resync: &Resync{RAND: octets(16, 0xaa), AUTS: octets(14, 0xbb)}, Features: FeatureIMSRestoration,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewMultimediaAuthRequest(cscfEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseMultimediaAuthRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestMultimediaAuthResyncEncoding(t *testing.T) {
	req, err := NewMultimediaAuthRequest(cscfEnvelope, MultimediaAuthRequest{
		PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5,
		Resync: &Resync{RAND: octets(16, 0xaa), AUTS: octets(14, 0xbb)},
	})
	if err != nil {
		t.Fatal(err)
	}

	item, _ := req.Find(AVPSIPAuthDataItem, tgpp.VendorID)
	inner, _ := item.Grouped()
	authz, _ := diameter.Find(inner, AVPSIPAuthorization, tgpp.VendorID)

	if want := append(octets(16, 0xaa), octets(14, 0xbb)...); !bytes.Equal(authz.Data, want) {
		t.Fatalf("SIP-Authorization = %x", authz.Data)
	}
}

func TestMultimediaAuthRequestValidation(t *testing.T) {
	valid := MultimediaAuthRequest{PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5}

	for name, mutate := range map[string]func(*MultimediaAuthRequest){
		"no private identity": func(r *MultimediaAuthRequest) { r.PrivateIdentity = "" },
		"no scheme":           func(r *MultimediaAuthRequest) { r.Scheme = "" },
		"bad server name":     func(r *MultimediaAuthRequest) { r.ServerName = "tel:123" },
		"short AUTS":          func(r *MultimediaAuthRequest) { r.Resync = &Resync{RAND: octets(16, 1), AUTS: octets(13, 1)} },
		"resync for digest": func(r *MultimediaAuthRequest) {
			r.Scheme, r.Resync = SchemeSIPDigest, &Resync{RAND: octets(16, 1), AUTS: octets(14, 1)}
		},
	} {
		r := valid
		mutate(&r)

		if _, err := NewMultimediaAuthRequest(cscfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseMultimediaAuthRequestErrors(t *testing.T) {
	item := func(avps ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
	}

	base := request(CommandMultimediaAuth,
		userName(testPrivate),
		vendorString(AVPPublicIdentity, testPublic),
		vendorUnsigned(AVPSIPNumberAuthItems, 1),
		item(vendorString(AVPSIPAuthenticationScheme, string(SchemeDigestAKAv1MD5))),
		vendorString(AVPServerName, testServer),
	)

	if _, err := ParseMultimediaAuthRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	noItem := without(base, AVPSIPAuthDataItem, tgpp.VendorID)

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Server-Name":      {without(base, AVPServerName, tgpp.VendorID), diameter.ResultMissingAVP},
		"no item":             {noItem, diameter.ResultMissingAVP},
		"zero vectors":        {with(without(base, AVPSIPNumberAuthItems, tgpp.VendorID), vendorUnsigned(AVPSIPNumberAuthItems, 0)), diameter.ResultInvalidAVPValue},
		"item without scheme": {with(noItem, item(vendorUnsigned(AVPSIPItemNumber, 1))), diameter.ResultMissingAVP},
		"empty scheme":        {with(noItem, item(vendorString(AVPSIPAuthenticationScheme, ""))), diameter.ResultInvalidAVPValue},
		"resync for a digest scheme": {
			with(noItem, item(vendorString(AVPSIPAuthenticationScheme, string(SchemeSIPDigest)),
				diameter.OctetString(AVPSIPAuthorization, diameter.AVPFlagMandatory, tgpp.VendorID, octets(30, 1)))),
			diameter.ResultInvalidAVPValue,
		},
		"short resync": {
			with(noItem, item(vendorString(AVPSIPAuthenticationScheme, string(SchemeDigestAKAv1MD5)),
				diameter.OctetString(AVPSIPAuthorization, diameter.AVPFlagMandatory, tgpp.VendorID, octets(29, 1)))),
			diameter.ResultInvalidAVPValue,
		},
		"bad Server-Name": {with(without(base, AVPServerName, tgpp.VendorID), vendorString(AVPServerName, "scscf")), diameter.ResultInvalidAVPValue},
		"bad Public-Identity": {
			with(without(base, AVPPublicIdentity, tgpp.VendorID), vendorString(AVPPublicIdentity, "alice")), diameter.ResultInvalidAVPValue,
		},
		"item not grouped": {
			with(noItem, diameter.OctetString(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPValue,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMultimediaAuthRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestMultimediaAuthAnswerRoundTrip(t *testing.T) {
	req := request(CommandMultimediaAuth)

	a := MultimediaAuth{
		Result:          tgpp.Result{Code: diameter.ResultSuccess},
		PrivateIdentity: testPrivate,
		PublicIdentity:  testPublic,
		Items: []AuthItem{
			{ItemNumber: 1, Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)},
			{ItemNumber: 2, Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(10)},
		},
		Features: FeatureIMSRestoration,
	}

	ans, err := NewMultimediaAuthAnswer(req, hssIdentity, a)
	if err != nil {
		t.Fatal(err)
	}

	count, _ := ans.Find(AVPSIPNumberAuthItems, tgpp.VendorID)
	if n, _ := count.Unsigned32(); n != 2 {
		t.Fatalf("SIP-Number-Auth-Items = %d", n)
	}

	got, err := ParseMultimediaAuthAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestMultimediaAuthAnswerEncoding(t *testing.T) {
	v := akaVector(1)

	avp, err := authItemAVP(AuthItem{Scheme: SchemeDigestAKAv1MD5, AKA: v})
	if err != nil {
		t.Fatal(err)
	}

	inner, _ := avp.Grouped()

	for code, want := range map[uint32][]byte{
		AVPSIPAuthenticate:    append(append([]byte(nil), v.RAND...), v.AUTN...),
		AVPSIPAuthorization:   v.XRES,
		AVPConfidentialityKey: v.CK,
		AVPIntegrityKey:       v.IK,
	} {
		a, ok := diameter.Find(inner, code, tgpp.VendorID)
		if !ok || !bytes.Equal(a.Data, want) || a.Flags&diameter.AVPFlagMandatory == 0 {
			t.Errorf("AVP %d = %+v", code, a)
		}
	}

	if _, ok := diameter.Find(inner, AVPSIPItemNumber, tgpp.VendorID); ok {
		t.Fatal("SIP-Item-Number sent without a number")
	}
}

func TestMultimediaAuthAnswerValidation(t *testing.T) {
	req := request(CommandMultimediaAuth)
	valid := MultimediaAuth{
		Result: tgpp.Result{Code: diameter.ResultSuccess}, PrivateIdentity: testPrivate, PublicIdentity: testPublic,
		Items: []AuthItem{{Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)}},
	}

	withVector := func(mutate func(*AKAVector)) func(*MultimediaAuth) {
		return func(a *MultimediaAuth) {
			v := akaVector(1)
			mutate(v)
			a.Items = []AuthItem{{Scheme: SchemeDigestAKAv1MD5, AKA: v}}
		}
	}

	for name, mutate := range map[string]func(*MultimediaAuth){
		"error result":         func(a *MultimediaAuth) { a.Result = tgpp.Experimental(tgpp.ResultErrorAuthSchemeNotSupported) },
		"experimental success": func(a *MultimediaAuth) { a.Result = tgpp.Experimental(diameter.ResultSuccess) },
		"no private":           func(a *MultimediaAuth) { a.PrivateIdentity = "" },
		"no public":            func(a *MultimediaAuth) { a.PublicIdentity = "" },
		"no items":             func(a *MultimediaAuth) { a.Items = nil },
		"no scheme":            func(a *MultimediaAuth) { a.Items = []AuthItem{{AKA: akaVector(1)}} },
		"digest scheme":        func(a *MultimediaAuth) { a.Items = []AuthItem{{Scheme: SchemeSIPDigest, AKA: akaVector(1)}} },
		"AKA without AKA":      func(a *MultimediaAuth) { a.Items = []AuthItem{{Scheme: SchemeDigestAKAv1MD5}} },
		"short RAND":           withVector(func(v *AKAVector) { v.RAND = octets(15, 1) }),
		"short AUTN":           withVector(func(v *AKAVector) { v.AUTN = octets(15, 1) }),
		"short XRES":           withVector(func(v *AKAVector) { v.XRES = octets(3, 1) }),
		"long XRES":            withVector(func(v *AKAVector) { v.XRES = octets(17, 1) }),
		"short CK":             withVector(func(v *AKAVector) { v.CK = octets(15, 1) }),
		"short IK":             withVector(func(v *AKAVector) { v.IK = octets(15, 1) }),
	} {
		a := valid
		mutate(&a)

		if _, err := NewMultimediaAuthAnswer(req, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestMultimediaAuthAnswerEchoesRequestIdentities(t *testing.T) {
	marIdentity := "sip:Alice@IMS.example.org;user=phone"

	req, err := NewMultimediaAuthRequest(cscfEnvelope, MultimediaAuthRequest{
		PrivateIdentity: testPrivate, PublicIdentity: marIdentity, ServerName: testServer, NumberOfItems: 1, Scheme: "digest-akav2-md5",
	})
	if err != nil {
		t.Fatal(err)
	}

	ans, err := NewMultimediaAuthAnswer(roundTrip(t, req), hssIdentity, MultimediaAuth{
		Result: tgpp.Result{Code: diameter.ResultSuccess},
		Items:  []AuthItem{{Scheme: "digest-akav2-md5", AKA: akaVector(1)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseMultimediaAuthAnswer(roundTrip(t, ans))
	if err != nil || got.PrivateIdentity != testPrivate || got.PublicIdentity != marIdentity || got.Items[0].AKA == nil {
		t.Fatalf("ParseMultimediaAuthAnswer = %+v, %v", got, err)
	}
}

func TestParsedAKAMaterialIsIndependent(t *testing.T) {
	r := MultimediaAuthRequest{
		PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5,
		Resync: &Resync{RAND: octets(16, 0xaa), AUTS: octets(14, 0xbb)},
	}

	req, err := ParseMultimediaAuthRequest(roundTrip(t, mustMessage(t)(NewMultimediaAuthRequest(cscfEnvelope, r))))
	if err != nil {
		t.Fatal(err)
	}

	_ = append(req.Resync.RAND, octets(14, 0xff)...)

	if !bytes.Equal(req.Resync.AUTS, octets(14, 0xbb)) {
		t.Fatalf("append to RAND overwrote AUTS: %x", req.Resync.AUTS)
	}

	ans, err := NewMultimediaAuthAnswer(request(CommandMultimediaAuth), hssIdentity, MultimediaAuth{
		Result: tgpp.Result{Code: diameter.ResultSuccess}, PrivateIdentity: testPrivate, PublicIdentity: testPublic,
		Items: []AuthItem{{Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)}},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseMultimediaAuthAnswer(roundTrip(t, ans))
	if err != nil {
		t.Fatal(err)
	}

	v := got.Items[0].AKA
	_ = append(v.RAND, octets(16, 0xff)...)

	if !bytes.Equal(v.AUTN, akaVector(1).AUTN) {
		t.Fatalf("append to RAND overwrote AUTN: %x", v.AUTN)
	}
}

func TestParseMultimediaAuthAnswerErrors(t *testing.T) {
	req := request(CommandMultimediaAuth)
	bare := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)
	success := with(bare, userName(testPrivate), vendorString(AVPPublicIdentity, testPublic))

	item := func(avps ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
	}
	scheme := vendorString(AVPSIPAuthenticationScheme, string(SchemeDigestAKAv1MD5))
	vector := mustAVP(t)(authItemAVP(AuthItem{Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)}))

	for name, ans := range map[string]*diameter.Message{
		"no items":            success,
		"no User-Name":        with(bare, vendorString(AVPPublicIdentity, testPublic), vector),
		"no Public-Identity":  with(bare, userName(testPrivate), vector),
		"bad Public-Identity": with(bare, userName(testPrivate), vendorString(AVPPublicIdentity, "alice"), vector),
		"item without scheme": with(success, item()),
		"AKA without vector":  with(success, item(scheme)),
		"short AUTN":          with(success, item(scheme, diameter.OctetString(AVPSIPAuthenticate, 0, tgpp.VendorID, octets(31, 1)))),
		"AKA without keys": with(success, item(scheme,
			diameter.OctetString(AVPSIPAuthenticate, 0, tgpp.VendorID, octets(32, 1)),
			diameter.OctetString(AVPSIPAuthorization, 0, tgpp.VendorID, octets(8, 1)))),
		"short XRES": with(success, item(scheme,
			diameter.OctetString(AVPSIPAuthenticate, 0, tgpp.VendorID, octets(32, 1)),
			diameter.OctetString(AVPSIPAuthorization, 0, tgpp.VendorID, octets(3, 1)),
			diameter.OctetString(AVPConfidentialityKey, 0, tgpp.VendorID, octets(16, 1)),
			diameter.OctetString(AVPIntegrityKey, 0, tgpp.VendorID, octets(16, 1)))),
		"short item number": with(success, item(scheme, diameter.OctetString(AVPSIPItemNumber, 0, tgpp.VendorID, octets(2, 1)))),
	} {
		if _, err := ParseMultimediaAuthAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	_, err := ParseMultimediaAuthAnswer(NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorAuthSchemeNotSupported), 0))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorAuthSchemeNotSupported) {
		t.Fatalf("err = %v", err)
	}

	md5 := with(success, item(vendorString(AVPSIPAuthenticationScheme, "Digest-MD5"),
		diameter.OctetString(AVPSIPAuthenticate, diameter.AVPFlagMandatory, tgpp.VendorID, []byte("nonce")),
		diameter.OctetString(AVPSIPAuthorization, diameter.AVPFlagMandatory, tgpp.VendorID, []byte("response"))))

	digest, err := ParseMultimediaAuthAnswer(md5)
	if err != nil || len(digest.Items) != 1 || digest.Items[0].AKA != nil || digest.Items[0].Scheme != "Digest-MD5" {
		t.Fatalf("non-AKA item = %+v, %v", digest, err)
	}
}

func TestIsAKAScheme(t *testing.T) {
	for scheme, want := range map[AuthenticationScheme]bool{
		SchemeDigestAKAv1MD5: true,
		SchemeDigestAKAv2MD5: true,
		"digest-akav1-md5":   true,
		"Digest-AKA":         false,
		SchemeSIPDigest:      false,
		"unknown":            false,
		"":                   false,
	} {
		if got := scheme.IsAKA(); got != want {
			t.Errorf("IsAKAScheme(%q) = %v", scheme, got)
		}
	}
}

func TestMultimediaAuthAnswerVectorLimit(t *testing.T) {
	req := mustMessage(t)(NewMultimediaAuthRequest(cscfEnvelope, MultimediaAuthRequest{
		PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 1, Scheme: SchemeDigestAKAv1MD5,
	}))

	a := MultimediaAuth{
		Result: tgpp.Result{Code: diameter.ResultSuccess},
		Items:  []AuthItem{{Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)}, {Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(2)}},
	}

	if _, err := NewMultimediaAuthAnswer(req, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("two vectors for one = %v", err)
	}

	a.Items = a.Items[:1]
	if _, err := NewMultimediaAuthAnswer(req, hssIdentity, a); err != nil {
		t.Fatal(err)
	}
}

func TestParseMultimediaAuthAnswerRejectsExperimentalSuccess(t *testing.T) {
	ans := with(NewAnswer(request(CommandMultimediaAuth), hssIdentity, tgpp.Experimental(diameter.ResultSuccess), 0),
		userName(testPrivate), vendorString(AVPPublicIdentity, testPublic),
		mustAVP(t)(authItemAVP(AuthItem{Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)})))

	if _, err := ParseMultimediaAuthAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("err = %v", err)
	}
}
