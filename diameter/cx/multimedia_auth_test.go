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
		"no vectors":          func(r *MultimediaAuthRequest) { r.NumberOfItems = 0 },
		"no scheme":           func(r *MultimediaAuthRequest) { r.Scheme = "" },
		"bad server name":     func(r *MultimediaAuthRequest) { r.ServerName = "tel:123" },
		"short AUTS":          func(r *MultimediaAuthRequest) { r.Resync = &Resync{RAND: octets(16, 1), AUTS: octets(13, 1)} },
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
		item(vendorString(AVPSIPAuthenticationScheme, SchemeDigestAKAv1MD5)),
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
		"item without scheme": {with(noItem, item(vendorUnsigned(AVPSIPItemNumber, 1))), diameter.ResultInvalidAVPValue},
		"short resync": {
			with(noItem, item(vendorString(AVPSIPAuthenticationScheme, SchemeDigestAKAv1MD5),
				diameter.OctetString(AVPSIPAuthorization, diameter.AVPFlagMandatory, tgpp.VendorID, octets(29, 1)))),
			diameter.ResultInvalidAVPValue,
		},
		"bad Server-Name": {with(without(base, AVPServerName, tgpp.VendorID), vendorString(AVPServerName, "scscf")), diameter.ResultInvalidAVPValue},
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

	for name, mutate := range map[string]func(*MultimediaAuth){
		"error result": func(a *MultimediaAuth) { a.Result = tgpp.Experimental(tgpp.ResultErrorAuthSchemeNotSupported) },
		"no private":   func(a *MultimediaAuth) { a.PrivateIdentity = "" },
		"no public":    func(a *MultimediaAuth) { a.PublicIdentity = "" },
		"no items":     func(a *MultimediaAuth) { a.Items = nil },
		"no scheme":    func(a *MultimediaAuth) { a.Items = []AuthItem{{AKA: akaVector(1)}} },
		"short RAND":   func(a *MultimediaAuth) { a.Items = []AuthItem{{Scheme: "x", AKA: &AKAVector{RAND: octets(15, 1)}}} },
		"long XRES": func(a *MultimediaAuth) {
			v := akaVector(1)
			v.XRES = octets(17, 1)
			a.Items = []AuthItem{{Scheme: "x", AKA: v}}
		},
		"short IK": func(a *MultimediaAuth) {
			v := akaVector(1)
			v.IK = octets(15, 1)
			a.Items = []AuthItem{{Scheme: "x", AKA: v}}
		},
	} {
		a := valid
		mutate(&a)

		if _, err := NewMultimediaAuthAnswer(req, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseMultimediaAuthAnswerErrors(t *testing.T) {
	req := request(CommandMultimediaAuth)
	success := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)

	item := func(avps ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(AVPSIPAuthDataItem, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
	}
	scheme := vendorString(AVPSIPAuthenticationScheme, SchemeDigestAKAv1MD5)

	for name, ans := range map[string]*diameter.Message{
		"no items":            success,
		"item without scheme": with(success, item()),
		"short AUTN":          with(success, item(scheme, diameter.OctetString(AVPSIPAuthenticate, 0, tgpp.VendorID, octets(31, 1)))),
		"AKA without keys": with(success, item(scheme,
			diameter.OctetString(AVPSIPAuthenticate, 0, tgpp.VendorID, octets(32, 1)),
			diameter.OctetString(AVPSIPAuthorization, 0, tgpp.VendorID, octets(8, 1)))),
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

	digest, err := ParseMultimediaAuthAnswer(with(success, item(vendorString(AVPSIPAuthenticationScheme, SchemeSIPDigest))))
	if err != nil || len(digest.Items) != 1 || digest.Items[0].AKA != nil || digest.Items[0].Scheme != SchemeSIPDigest {
		t.Fatalf("non-AKA item = %+v, %v", digest, err)
	}
}
