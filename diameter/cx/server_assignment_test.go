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

func TestServerAssignmentRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]ServerAssignmentRequest{
		"registration": {PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer, Type: AssignmentRegistration},
		"re-registration with data": {
			PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer,
			Type: AssignmentReRegistration, UserDataAlreadyAvailable: true, Features: FeatureAliasIndication,
		},
		"unregistered user":           {PublicIdentities: []string{testTel}, ServerName: testServer, Type: AssignmentUnregisteredUser},
		"timeout by private identity": {PrivateIdentity: testPrivate, ServerName: testServer, Type: AssignmentTimeoutDeregistration},
		"user deregistration of several": {
			PublicIdentities: []string{testPublic, testTel}, ServerName: testServer, Type: AssignmentUserDeregistration,
		},
		"authentication failure": {PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer, Type: AssignmentAuthenticationFailure},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewServerAssignmentRequest(cscfEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseServerAssignmentRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestServerAssignmentRequestValidation(t *testing.T) {
	valid := ServerAssignmentRequest{PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer, Type: AssignmentRegistration}

	for name, mutate := range map[string]func(*ServerAssignmentRequest){
		"SWx type":                     func(r *ServerAssignmentRequest) { r.Type = 12 },
		"registration without private": func(r *ServerAssignmentRequest) { r.PrivateIdentity = "" },
		"registration of two":          func(r *ServerAssignmentRequest) { r.PublicIdentities = []string{testPublic, testTel} },
		"registration of none":         func(r *ServerAssignmentRequest) { r.PublicIdentities = nil },
		"deregistration of nobody": func(r *ServerAssignmentRequest) {
			r.Type, r.PrivateIdentity, r.PublicIdentities = AssignmentAdministrativeDeregistration, "", nil
		},
		"bad public identity": func(r *ServerAssignmentRequest) { r.PublicIdentities = []string{"alice"} },
		"no server name":      func(r *ServerAssignmentRequest) { r.ServerName = "" },
	} {
		r := valid
		mutate(&r)

		if _, err := NewServerAssignmentRequest(cscfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseServerAssignmentRequestErrors(t *testing.T) {
	base := request(CommandServerAssignment,
		userName(testPrivate),
		vendorString(AVPPublicIdentity, testPublic),
		vendorString(AVPServerName, testServer),
		vendorUnsigned(AVPServerAssignmentType, AssignmentRegistration),
		vendorUnsigned(AVPUserDataAlreadyAvailable, 0),
	)

	if _, err := ParseServerAssignmentRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	withType := func(m *diameter.Message, v uint32) *diameter.Message {
		return with(without(m, AVPServerAssignmentType, tgpp.VendorID), vendorUnsigned(AVPServerAssignmentType, v))
	}

	for name, tt := range map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no type":                      {without(base, AVPServerAssignmentType, tgpp.VendorID), diameter.ResultMissingAVP},
		"no data availability":         {without(base, AVPUserDataAlreadyAvailable, tgpp.VendorID), diameter.ResultMissingAVP},
		"SWx type":                     {withType(base, 13), diameter.ResultInvalidAVPValue},
		"bad availability":             {with(without(base, AVPUserDataAlreadyAvailable, tgpp.VendorID), vendorUnsigned(AVPUserDataAlreadyAvailable, 2)), diameter.ResultInvalidAVPValue},
		"registration without private": {without(base, diameter.AVPUserName, 0), diameter.ResultMissingAVP},
		"registration of none":         {without(base, AVPPublicIdentity, tgpp.VendorID), diameter.ResultMissingAVP},
		"registration of two":          {with(base, vendorString(AVPPublicIdentity, testTel)), diameter.ResultAVPOccursTooManyTimes},
		"empty User-Name":              {with(without(base, diameter.AVPUserName, 0), diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "")), diameter.ResultInvalidAVPValue},
		"deregistration of nobody": {
			withType(without(without(base, diameter.AVPUserName, 0), AVPPublicIdentity, tgpp.VendorID), AssignmentTimeoutDeregistration),
			diameter.ResultMissingAVP,
		},
		"bad Public-Identity": {with(without(base, AVPPublicIdentity, tgpp.VendorID), vendorString(AVPPublicIdentity, "x")), diameter.ResultInvalidAVPValue},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseServerAssignmentRequest(tt.req)
			if code := avpResult(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestServerAssignmentAnswerRoundTrip(t *testing.T) {
	req := mustMessage(t)(NewServerAssignmentRequest(cscfEnvelope, ServerAssignmentRequest{
		PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer, Type: AssignmentRegistration,
	}))

	a := ServerAssignment{
		Result:          tgpp.Result{Code: diameter.ResultSuccess},
		PrivateIdentity: testPrivate,
		UserData:        []byte("<IMSSubscription/>"),
		Charging: &ChargingInformation{
			PrimaryEventChargingFunction:        "aaa://ocs.example.org",
			PrimaryChargingCollectionFunction:   "aaa://cdf1.example.org",
			SecondaryChargingCollectionFunction: "aaa://cdf2.example.org",
		},
		AssociatedIdentities: []string{testPrivate, "second@example.org"},
		LooseRouteRequired:   true,
		ServerName:           testServer,
		PriviledgedSender:    true,
		Features:             FeatureAliasIndication,
	}

	ans, err := NewServerAssignmentAnswer(req, hssIdentity, a)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseServerAssignmentAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestServerAssignmentAnswerForDeregistration(t *testing.T) {
	req := mustMessage(t)(NewServerAssignmentRequest(cscfEnvelope, ServerAssignmentRequest{
		PrivateIdentity: testPrivate, ServerName: testServer, Type: AssignmentUserDeregistration,
	}))

	a := ServerAssignment{Result: tgpp.Experimental(tgpp.ResultSuccessServerNameNotStored), PrivateIdentity: testPrivate}

	ans, err := NewServerAssignmentAnswer(req, hssIdentity, a)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseServerAssignmentAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(got, a) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestServerAssignmentAnswerValidation(t *testing.T) {
	registration := mustMessage(t)(NewServerAssignmentRequest(cscfEnvelope, ServerAssignmentRequest{
		PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic}, ServerName: testServer, Type: AssignmentRegistration,
	}))

	valid := ServerAssignment{Result: tgpp.Result{Code: diameter.ResultSuccess}, UserData: []byte("<IMSSubscription/>")}

	for name, mutate := range map[string]func(*ServerAssignment){
		"error result": func(a *ServerAssignment) { a.Result = tgpp.Experimental(tgpp.ResultErrorUserUnknown) },
		"no user data": func(a *ServerAssignment) { a.UserData = nil },
		"no primary function": func(a *ServerAssignment) {
			a.Charging = &ChargingInformation{SecondaryEventChargingFunction: "aaa://x"}
		},
		"empty associated": func(a *ServerAssignment) { a.AssociatedIdentities = []string{""} },
		"bad server name":  func(a *ServerAssignment) { a.ServerName = "scscf" },
	} {
		a := valid
		mutate(&a)

		if _, err := NewServerAssignmentAnswer(registration, hssIdentity, a); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseServerAssignmentAnswerErrors(t *testing.T) {
	req := request(CommandServerAssignment)
	success := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)

	for name, ans := range map[string]*diameter.Message{
		"charging not grouped":  with(success, diameter.OctetString(AVPChargingInformation, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})),
		"empty associated name": with(success, diameter.Grouped(AVPAssociatedIdentities, 0, tgpp.VendorID, diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, ""))),
		"short loose route":     with(success, diameter.OctetString(AVPLooseRouteIndication, 0, tgpp.VendorID, []byte{1})),
		"short priviledged":     with(success, diameter.OctetString(AVPPriviledgedSenderIndication, 0, tgpp.VendorID, []byte{1})),
	} {
		if _, err := ParseServerAssignmentAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	_, err := ParseServerAssignmentAnswer(NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorInAssignmentType), 0))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorInAssignmentType) {
		t.Fatalf("err = %v", err)
	}
}

func TestServerAssignmentRequestIgnoresForeignOptionalAVPs(t *testing.T) {
	req := request(CommandServerAssignment,
		diameter.UTF8String(494, 0, 50, "4130282081_47464792@192.0.2.2"),
		vendorString(AVPPublicIdentity, testPublic),
		vendorString(AVPServerName, testServer),
		userName(testPrivate),
		vendorUnsigned(AVPServerAssignmentType, AssignmentRegistration),
		vendorUnsigned(AVPUserDataAlreadyAvailable, 0),
	)

	got, err := ParseServerAssignmentRequest(req)
	if err != nil || got.PrivateIdentity != testPrivate || got.Type != AssignmentRegistration {
		t.Fatalf("ParseServerAssignmentRequest = %+v, %v", got, err)
	}
}
