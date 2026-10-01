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

func TestUserAuthorizationRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]UserAuthorizationRequest{
		"registration": {PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm},
		"deregistration": {
			PrivateIdentity: testPrivate, PublicIdentity: testTel, VisitedNetwork: "visited.example.org",
			AuthorizationType: AuthorizationDeregistration,
		},
		"emergency with capabilities": {
			PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm,
			AuthorizationType: AuthorizationRegistrationAndCapabilities, EmergencyRegistration: true, Features: FeatureIMSRestoration,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewUserAuthorizationRequest(cscfEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			if req.CommandCode != CommandUserAuthorization {
				t.Fatalf("command %d", req.CommandCode)
			}

			got, err := ParseUserAuthorizationRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestUserAuthorizationRequestOmitsDefaultType(t *testing.T) {
	req, err := NewUserAuthorizationRequest(cscfEnvelope, UserAuthorizationRequest{PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm})
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := req.Find(AVPUserAuthorizationType, tgpp.VendorID); ok {
		t.Fatal("REGISTRATION sent explicitly")
	}

	if _, ok := req.Find(AVPUARFlags, tgpp.VendorID); ok {
		t.Fatal("UAR-Flags sent without flags")
	}
}

func TestUserAuthorizationRequestValidation(t *testing.T) {
	valid := UserAuthorizationRequest{PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm}

	for name, mutate := range map[string]func(*UserAuthorizationRequest){
		"no private identity": func(r *UserAuthorizationRequest) { r.PrivateIdentity = "" },
		"bad public identity": func(r *UserAuthorizationRequest) { r.PublicIdentity = "alice@example.org" },
		"no visited network":  func(r *UserAuthorizationRequest) { r.VisitedNetwork = "" },
		"unknown type":        func(r *UserAuthorizationRequest) { r.AuthorizationType = 3 },
	} {
		r := valid
		mutate(&r)

		if _, err := NewUserAuthorizationRequest(cscfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseUserAuthorizationRequestErrors(t *testing.T) {
	base := request(CommandUserAuthorization,
		userName(testPrivate),
		vendorString(AVPPublicIdentity, testPublic),
		diameter.OctetString(AVPVisitedNetworkIdentifier, diameter.AVPFlagMandatory, tgpp.VendorID, []byte(testRealm)),
	)

	if _, err := ParseUserAuthorizationRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no User-Name":       {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"no Public-Identity": {without(base, AVPPublicIdentity, tgpp.VendorID), diameter.ResultMissingAVP},
		"no visited network": {without(base, AVPVisitedNetworkIdentifier, tgpp.VendorID), diameter.ResultMissingAVP},
		"bad Public-Identity": {
			with(without(base, AVPPublicIdentity, tgpp.VendorID), vendorString(AVPPublicIdentity, "alice")), diameter.ResultInvalidAVPValue,
		},
		"unknown type":    {with(base, vendorUnsigned(AVPUserAuthorizationType, 7)), diameter.ResultInvalidAVPValue},
		"short UAR-Flags": {with(base, diameter.OctetString(AVPUARFlags, 0, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPLength},
		"unknown M AVP":   {with(base, vendorUnsigned(AVPServerAssignmentType, 1)), diameter.ResultAVPUnsupported},
		"two User-Names":  {with(base, userName(testPrivate)), diameter.ResultAVPOccursTooManyTimes},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseUserAuthorizationRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestUserAuthorizationAnswerRoundTrip(t *testing.T) {
	req := request(CommandUserAuthorization)

	for name, a := range map[string]UserAuthorization{
		"first registration": {
			Result:       tgpp.Experimental(tgpp.ResultFirstRegistration),
			Capabilities: &ServerCapabilities{Mandatory: []uint32{1, 2}, Optional: []uint32{3}, ServerNames: []string{testServer}},
			Features:     FeatureIMSRestoration,
		},
		"any S-CSCF": {Result: tgpp.Experimental(tgpp.ResultFirstRegistration), Capabilities: &ServerCapabilities{}},
		"subsequent": {Result: tgpp.Experimental(tgpp.ResultSubsequentRegistration), ServerName: testServer},
		"success":    {Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: testServer},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewUserAuthorizationAnswer(req, hssIdentity, a)
			if err != nil {
				t.Fatal(err)
			}

			requireVendorSpecificApplicationID(t, ans)

			got, err := ParseUserAuthorizationAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestUserAuthorizationAnswerValidation(t *testing.T) {
	req := request(CommandUserAuthorization)

	for name, a := range map[string]UserAuthorization{
		"error result": {Result: tgpp.Experimental(tgpp.ResultErrorRoamingNotAllowed)},
		"name and capabilities": {
			Result: tgpp.Experimental(tgpp.ResultFirstRegistration), ServerName: testServer, Capabilities: &ServerCapabilities{},
		},
		"bad server name":     {Result: tgpp.Experimental(tgpp.ResultSubsequentRegistration), ServerName: "scscf.example.org"},
		"bad capability name": {Result: tgpp.Experimental(tgpp.ResultFirstRegistration), Capabilities: &ServerCapabilities{ServerNames: []string{"x"}}},
	} {
		if _, err := NewUserAuthorizationAnswer(req, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseUserAuthorizationAnswerErrors(t *testing.T) {
	req := request(CommandUserAuthorization)

	_, err := ParseUserAuthorizationAnswer(NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch), 0))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorIdentitiesDontMatch) {
		t.Fatalf("err = %v", err)
	}

	_, err = ParseUserAuthorizationAnswer(NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultAuthorizationRejected}, 0))
	if r, ok := tgpp.ResultOf(err); !ok || r.Code != diameter.ResultAuthorizationRejected {
		t.Fatalf("err = %v", err)
	}

	bad := with(NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultFirstRegistration), 0),
		diameter.Grouped(AVPServerCapabilities, diameter.AVPFlagMandatory, tgpp.VendorID,
			diameter.OctetString(AVPMandatoryCapability, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})))

	if _, err := ParseUserAuthorizationAnswer(bad); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("short capability = %v", err)
	}
}
