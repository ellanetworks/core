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

	base := request(CommandRegistrationTermination, userName(testPrivate), reason(vendorUnsigned(AVPReasonCode, uint32(ReasonServerChange))))

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
		"new server, no public": {with(noReason, reason(vendorUnsigned(AVPReasonCode, uint32(ReasonNewServerAssigned)))), diameter.ResultMissingAVP},
		"bad associated": {
			with(base, diameter.Grouped(AVPAssociatedIdentities, 0, tgpp.VendorID, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, ""))),
			diameter.ResultInvalidAVPValue,
		},
		"short RTR-Flags":     {with(base, diameter.OctetString(AVPRTRFlags, 0, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPLength},
		"bad Public-Identity": {with(base, vendorString(AVPPublicIdentity, "alice")), diameter.ResultInvalidAVPValue},
		"reason not grouped": {
			with(noReason, diameter.OctetString(AVPDeregistrationReason, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPValue,
		},
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
			Result:               tgpp.Result{Code: diameter.ResultSuccess},
			AssociatedIdentities: []string{testPrivate, "second@example.org"},
			EmergencyIdentities:  []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: "sip:sos@example.org"}},
			Features:             FeatureIMSRestoration,
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
		"emergency, no private": {Result: tgpp.Result{Code: diameter.ResultSuccess}, EmergencyIdentities: []EmergencyIdentity{{PublicIdentity: testPublic}}},
		"emergency, bad public": {Result: tgpp.Result{Code: diameter.ResultSuccess}, EmergencyIdentities: []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: "x"}}},
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

func TestRegistrationTerminationErrorAnswer(t *testing.T) {
	req := request(CommandRegistrationTermination)
	e := RegistrationTerminationError{
		ResultError:          ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}, Features: FeatureIMSRestoration},
		AssociatedIdentities: []string{testPrivate},
		EmergencyIdentities:  []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: "sip:sos@example.org"}},
	}

	ans, err := NewRegistrationTerminationErrorAnswer(req, cscfIdentity, e)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseRegistrationTerminationAnswer(roundTrip(t, ans))

	var rte *RegistrationTerminationError
	if !errors.As(err, &rte) || !reflect.DeepEqual(*rte, e) {
		t.Fatalf("parsed error = %#v, want %#v", err, e)
	}

	var re *ResultError
	if !errors.As(err, &re) || re.Code != diameter.ResultUnableToComply || re.Features != FeatureIMSRestoration {
		t.Fatalf("base result error = %#v", re)
	}

	unable := ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}}

	for name, bad := range map[string]RegistrationTerminationError{
		"success":          {ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}},
		"empty associated": {ResultError: unable, AssociatedIdentities: []string{""}},
		"bad pair":         {ResultError: unable, EmergencyIdentities: []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: "sos"}}},
	} {
		if _, err := NewRegistrationTerminationErrorAnswer(req, cscfIdentity, bad); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	invalidPair := with(NewAnswer(req, cscfIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0),
		diameter.Grouped(AVPIdentityWithEmergencyRegistration, 0, tgpp.VendorID, userName(testPrivate), vendorString(AVPPublicIdentity, "sos")))
	if _, err := ParseRegistrationTerminationAnswer(invalidPair); !errors.Is(err, ErrMalformedAnswer) {
		t.Fatalf("invalid emergency pair = %v", err)
	}
}

func TestParseRegistrationTerminationRequestPointsAtReasonCode(t *testing.T) {
	code := vendorUnsigned(AVPReasonCode, 9)
	req := request(CommandRegistrationTermination, userName(testPrivate),
		diameter.Grouped(AVPDeregistrationReason, diameter.AVPFlagMandatory, tgpp.VendorID, code))

	_, err := ParseRegistrationTerminationRequest(req)

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) || avpErr.AVP.Code != AVPReasonCode {
		t.Fatalf("err = %v, want Reason-Code reported", err)
	}
}

func TestRegistrationTerminationErrorSurvivesBadFields(t *testing.T) {
	ans := with(NewAnswer(request(CommandRegistrationTermination), cscfIdentity, tgpp.Result{Code: diameter.ResultUnableToComply}, 0),
		diameter.Grouped(AVPIdentityWithEmergencyRegistration, 0, tgpp.VendorID, userName(testPrivate), vendorString(AVPPublicIdentity, "sos")),
		diameter.Grouped(AVPIdentityWithEmergencyRegistration, 0, tgpp.VendorID, userName(testPrivate), vendorString(AVPPublicIdentity, testPublic)),
	)

	_, err := ParseRegistrationTerminationAnswer(ans)

	var re *RegistrationTerminationError
	if !errors.As(err, &re) || re.Code != diameter.ResultUnableToComply ||
		!reflect.DeepEqual(re.EmergencyIdentities, []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: testPublic}}) {
		t.Fatalf("err = %#v", err)
	}
}
