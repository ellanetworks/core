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

func TestRegistrationTerminationRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]RegistrationTerminationRequest{
		"permanent termination": {PrivateIdentity: testPrivate, Reason: DeregistrationReason{Code: ReasonPermanentTermination, Info: "subscription ended"}},
		"new server": {
			PrivateIdentity: testPrivate, AssociatedIdentities: []string{"second@example.org"},
			PublicIdentities: []string{testPublic, testTel}, Reason: DeregistrationReason{Code: ReasonNewServerAssigned},
			ReferenceLocationChanged: true, Features: FeatureIMSRestoration,
		},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewRegistrationTerminationRequest(hssEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			if host, ok := req.Find(diameter.AVPDestinationHost, 0); !ok || host.UTF8String() != hssEnvelope.DestinationHost {
				t.Fatalf("Destination-Host = %+v", host)
			}

			got, err := ParseRegistrationTerminationRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestRegistrationTerminationRequestValidation(t *testing.T) {
	valid := RegistrationTerminationRequest{PrivateIdentity: testPrivate}

	for name, tt := range map[string]struct {
		env    tgpp.Envelope
		mutate func(*RegistrationTerminationRequest)
	}{
		"no destination host":   {cscfEnvelope, func(*RegistrationTerminationRequest) {}},
		"no private identity":   {hssEnvelope, func(r *RegistrationTerminationRequest) { r.PrivateIdentity = "" }},
		"unknown reason":        {hssEnvelope, func(r *RegistrationTerminationRequest) { r.Reason.Code = 4 }},
		"new server, no public": {hssEnvelope, func(r *RegistrationTerminationRequest) { r.Reason.Code = ReasonNewServerAssigned }},
		"bad public identity":   {hssEnvelope, func(r *RegistrationTerminationRequest) { r.PublicIdentities = []string{"alice"} }},
		"empty associated":      {hssEnvelope, func(r *RegistrationTerminationRequest) { r.AssociatedIdentities = []string{""} }},
	} {
		r := valid
		tt.mutate(&r)

		if _, err := NewRegistrationTerminationRequest(tt.env, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseRegistrationTerminationRequestErrors(t *testing.T) {
	reason := func(avps ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(AVPDeregistrationReason, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
	}

	base := request(CommandRegistrationTermination, userName(testPrivate), reason(vendorUnsigned(AVPReasonCode, ReasonServerChange)))

	if _, err := ParseRegistrationTerminationRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	noReason := without(base, AVPDeregistrationReason, tgpp.VendorID)

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Destination-Host":   {without(base, diameter.AVPDestinationHost, 0), diameter.ResultMissingAVP},
		"no reason":             {noReason, diameter.ResultMissingAVP},
		"reason without code":   {with(noReason, reason(vendorString(AVPReasonInfo, "x"))), diameter.ResultMissingAVP},
		"unknown reason":        {with(noReason, reason(vendorUnsigned(AVPReasonCode, 9))), diameter.ResultInvalidAVPValue},
		"new server, no public": {with(noReason, reason(vendorUnsigned(AVPReasonCode, ReasonNewServerAssigned))), diameter.ResultMissingAVP},
		"bad associated": {
			with(base, diameter.Grouped(AVPAssociatedIdentities, 0, tgpp.VendorID, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, ""))),
			diameter.ResultInvalidAVPValue,
		},
		"short RTR-Flags": {with(base, diameter.OctetString(AVPRTRFlags, 0, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPValue},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseRegistrationTerminationRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestRegistrationTerminationAnswerRoundTrip(t *testing.T) {
	req := request(CommandRegistrationTermination)

	for name, a := range map[string]RegistrationTermination{
		"plain": {Result: tgpp.Result{Code: diameter.ResultSuccess}},
		"with emergency": {
			Result:                 tgpp.Result{Code: diameter.ResultSuccess},
			AssociatedIdentities:   []string{testPrivate, "second@example.org"},
			EmergencyRegistrations: []EmergencyRegistration{{PrivateIdentity: testPrivate, PublicIdentity: "sip:sos@example.org"}},
			Features:               FeatureIMSRestoration,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewRegistrationTerminationAnswer(req, cscfIdentity, a)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseRegistrationTerminationAnswer(roundTrip(t, ans))
			if err != nil || !reflect.DeepEqual(got, a) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestRegistrationTerminationAnswerErrors(t *testing.T) {
	req := request(CommandRegistrationTermination)

	for name, a := range map[string]RegistrationTermination{
		"error result":          {Result: tgpp.Result{Code: diameter.ResultUnableToComply}},
		"emergency, no private": {Result: tgpp.Result{Code: diameter.ResultSuccess}, EmergencyRegistrations: []EmergencyRegistration{{PublicIdentity: testPublic}}},
		"emergency, bad public": {Result: tgpp.Result{Code: diameter.ResultSuccess}, EmergencyRegistrations: []EmergencyRegistration{{PrivateIdentity: testPrivate, PublicIdentity: "x"}}},
	} {
		if _, err := NewRegistrationTerminationAnswer(req, cscfIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	success := NewAnswer(req, cscfIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)

	for name, ans := range map[string]*diameter.Message{
		"half emergency pair":    with(success, diameter.Grouped(AVPIdentityWithEmergencyRegistration, 0, tgpp.VendorID, userName(testPrivate))),
		"emergency not grouped":  with(success, diameter.OctetString(AVPIdentityWithEmergencyRegistration, 0, tgpp.VendorID, []byte{1})),
		"associated not grouped": with(success, diameter.OctetString(AVPAssociatedIdentities, 0, tgpp.VendorID, []byte{1})),
	} {
		if _, err := ParseRegistrationTerminationAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
