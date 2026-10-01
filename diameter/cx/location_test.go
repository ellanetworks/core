// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestLocationInfoRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]LocationInfoRequest{
		"terminating": {PublicIdentity: testTel},
		"originating": {PublicIdentity: testPublic, Originating: true, Features: FeatureIMSRestoration},
		"restoration": {PublicIdentity: testPublic, AuthorizationType: AuthorizationRegistrationAndCapabilities},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewLocationInfoRequest(cscfEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseLocationInfoRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestLocationInfoRequestValidation(t *testing.T) {
	for name, r := range map[string]LocationInfoRequest{
		"bad identity":   {PublicIdentity: "+15551230002"},
		"deregistration": {PublicIdentity: testTel, AuthorizationType: AuthorizationDeregistration},
	} {
		if _, err := NewLocationInfoRequest(cscfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseLocationInfoRequestErrors(t *testing.T) {
	base := request(CommandLocationInfo, vendorString(AVPPublicIdentity, testTel))

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no identity":           {without(base, AVPPublicIdentity, tgpp.VendorID), diameter.ResultMissingAVP},
		"bad originating":       {with(base, vendorUnsigned(AVPOriginatingRequest, 1)), diameter.ResultInvalidAVPValue},
		"deregistration":        {with(base, vendorUnsigned(AVPUserAuthorizationType, AuthorizationDeregistration)), diameter.ResultInvalidAVPValue},
		"no Origin-Host":        {without(base, diameter.AVPOriginHost, 0), diameter.ResultMissingAVP},
		"no Session-Id":         {without(base, diameter.AVPSessionID, 0), diameter.ResultMissingAVP},
		"no Auth-Session-State": {without(base, diameter.AVPAuthSessionState, 0), diameter.ResultMissingAVP},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseLocationInfoRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestLocationInfoAnswerRoundTrip(t *testing.T) {
	req := request(CommandLocationInfo)

	for name, l := range map[string]LocationInfo{
		"registered":   {Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: testServer},
		"unregistered": {Result: tgpp.Experimental(tgpp.ResultUnregisteredService), Capabilities: &ServerCapabilities{Mandatory: []uint32{1}}},
		"PSI direct routing": {
			Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: "sip:as.example.org",
			WildcardedPublicIdentity: "sip:chatlist!.*!@example.org", PSIDirectRouting: true, Features: FeatureIMSRestoration,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewLocationInfoAnswer(req, hssIdentity, l)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseLocationInfoAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, l) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestLocationInfoAnswerErrors(t *testing.T) {
	req := request(CommandLocationInfo)

	if _, err := NewLocationInfoAnswer(req, hssIdentity, LocationInfo{Result: tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered)}); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("error result = %v", err)
	}

	_, err := ParseLocationInfoAnswer(NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorIdentityNotRegistered), 0))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorIdentityNotRegistered) {
		t.Fatalf("err = %v", err)
	}

	bad := with(NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0), diameter.OctetString(AVPLIAFlags, 0, tgpp.VendorID, []byte{1}))
	if _, err := ParseLocationInfoAnswer(bad); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("short LIA-Flags = %v", err)
	}
}
