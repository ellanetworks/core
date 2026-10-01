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
		vendorUnsigned(AVPServerAssignmentType, uint32(AssignmentRegistration)),
		vendorUnsigned(AVPUserDataAlreadyAvailable, 0),
	)

	if _, err := ParseServerAssignmentRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	withType := func(m *diameter.Message, v AssignmentType) *diameter.Message {
		return with(without(m, AVPServerAssignmentType, tgpp.VendorID), vendorUnsigned(AVPServerAssignmentType, uint32(v)))
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
		PrivilegedSender:     true,
		AllowedWebRTC:        &AllowedWebRTCFunctions{AuthenticationFunctions: []string{"waf.example.org"}, WebServerFunctions: []string{"wwsf.example.org"}},
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
		vendorUnsigned(AVPServerAssignmentType, uint32(AssignmentRegistration)),
		vendorUnsigned(AVPUserDataAlreadyAvailable, 0),
	)

	got, err := ParseServerAssignmentRequest(req)
	if err != nil || got.PrivateIdentity != testPrivate || got.Type != AssignmentRegistration {
		t.Fatalf("ParseServerAssignmentRequest = %+v, %v", got, err)
	}
}

func TestServerAssignmentAnswerUserDataRequirement(t *testing.T) {
	sar := func(r ServerAssignmentRequest, extra ...diameter.AVP) *diameter.Message {
		r.ServerName = testServer
		if r.PrivateIdentity == "" && r.Type != AssignmentUnregisteredUser {
			r.PrivateIdentity = testPrivate
		}

		return with(mustMessage(t)(NewServerAssignmentRequest(cscfEnvelope, r)), extra...)
	}

	one := []string{testPublic}
	success := ServerAssignment{Result: tgpp.Result{Code: diameter.ResultSuccess}}

	for name, tt := range map[string]struct {
		req      *diameter.Message
		required bool
	}{
		"registration":            {sar(ServerAssignmentRequest{PublicIdentities: one, Type: AssignmentRegistration}), true},
		"no assignment":           {sar(ServerAssignmentRequest{PublicIdentities: one, Type: AssignmentNoAssignment}), true},
		"unregistered user":       {sar(ServerAssignmentRequest{PublicIdentities: one, Type: AssignmentUnregisteredUser}), true},
		"re-registration, cached": {sar(ServerAssignmentRequest{PublicIdentities: one, Type: AssignmentReRegistration, UserDataAlreadyAvailable: true}), false},
		"P-CSCF restoration": {
			sar(ServerAssignmentRequest{PublicIdentities: one, Type: AssignmentUnregisteredUser},
				diameter.Unsigned32(AVPSARFlags, 0, tgpp.VendorID, sarFlagPCSCFRestoration)),
			false,
		},
		"deregistration":  {sar(ServerAssignmentRequest{Type: AssignmentUserDeregistration}), false},
		"no request type": {request(CommandServerAssignment), false},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewServerAssignmentAnswer(tt.req, hssIdentity, success)
			if tt.required != errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("answer without user data: %v", err)
			}

			withData := success
			withData.UserData = []byte("<IMSSubscription/>")

			if _, err := NewServerAssignmentAnswer(tt.req, hssIdentity, withData); err != nil {
				t.Fatalf("answer with user data: %v", err)
			}
		})
	}
}

func TestServerAssignmentWildcardedIdentity(t *testing.T) {
	wildcard := "sip:conf-!.*!@example.org"
	r := ServerAssignmentRequest{
		PublicIdentities: []string{"sip:conf-1@example.org"}, WildcardedPublicIdentity: wildcard, ServerName: testServer, Type: AssignmentUnregisteredUser,
	}

	req, err := NewServerAssignmentRequest(cscfEnvelope, r)
	if err != nil {
		t.Fatal(err)
	}

	if w, _ := req.Find(AVPWildcardedPublicIdentity, tgpp.VendorID); w.Flags&diameter.AVPFlagMandatory != 0 {
		t.Fatalf("Wildcarded-Public-Identity = %+v", w)
	}

	got, err := ParseServerAssignmentRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("request round trip = %+v, %v", got, err)
	}

	a := ServerAssignment{Result: tgpp.Result{Code: diameter.ResultSuccess}, UserData: []byte("<IMSSubscription/>"), WildcardedPublicIdentity: wildcard}

	ans, err := NewServerAssignmentAnswer(req, hssIdentity, a)
	if err != nil {
		t.Fatal(err)
	}

	gotAnswer, err := ParseServerAssignmentAnswer(roundTrip(t, ans))
	if err != nil || !reflect.DeepEqual(gotAnswer, a) {
		t.Fatalf("answer round trip = %+v, %v", gotAnswer, err)
	}

	r.WildcardedPublicIdentity = "conf-!.*!"
	if _, err := NewServerAssignmentRequest(cscfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("bad wildcard = %v", err)
	}

	if _, err := ParseServerAssignmentRequest(with(without(req, AVPWildcardedPublicIdentity, tgpp.VendorID),
		diameter.UTF8String(AVPWildcardedPublicIdentity, 0, tgpp.VendorID, "x"))); avpResult(t, err) != diameter.ResultInvalidAVPValue {
		t.Fatalf("bad wildcard parse = %v", err)
	}
}

func TestServerAssignmentErrorAnswer(t *testing.T) {
	req := request(CommandServerAssignment)

	result := func(code uint32) ResultError { return ResultError{Result: tgpp.Experimental(code)} }

	for name, e := range map[string]ServerAssignmentError{
		"already registered": {
			ResultError:     ResultError{Result: tgpp.Experimental(tgpp.ResultErrorIdentityAlreadyRegistered), Features: FeatureIMSRestoration},
			PrivateIdentity: testPrivate, ServerName: "sip:other-scscf.example.org",
		},
		"in assignment type": {ResultError: result(tgpp.ResultErrorInAssignmentType), WildcardedPublicIdentity: "sip:conf-!.*!@example.org"},
		"plain":              {ResultError: result(tgpp.ResultErrorUserUnknown)},
	} {
		t.Run(name, func(t *testing.T) {
			ans, err := NewServerAssignmentErrorAnswer(req, hssIdentity, e)
			if err != nil {
				t.Fatal(err)
			}

			requireVendorSpecificApplicationID(t, ans)

			_, err = ParseServerAssignmentAnswer(roundTrip(t, ans))

			var sae *ServerAssignmentError
			if !errors.As(err, &sae) || !reflect.DeepEqual(*sae, e) {
				t.Fatalf("parsed error = %#v, want %#v", err, e)
			}

			if !tgpp.IsExperimental(err, e.Code) {
				t.Fatalf("tgpp.IsExperimental(%v) = false", err)
			}
		})
	}

	for name, e := range map[string]ServerAssignmentError{
		"success":         {ResultError: ResultError{Result: tgpp.Result{Code: diameter.ResultSuccess}}},
		"bad server name": {ResultError: result(tgpp.ResultErrorIdentityAlreadyRegistered), ServerName: "scscf"},
		"bad wildcard":    {ResultError: result(tgpp.ResultErrorInAssignmentType), WildcardedPublicIdentity: "x"},
	} {
		if _, err := NewServerAssignmentErrorAnswer(req, hssIdentity, e); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseServerAssignmentAnswerStrictness(t *testing.T) {
	req := request(CommandServerAssignment)
	success := NewAnswer(req, hssIdentity, tgpp.Result{Code: diameter.ResultSuccess}, 0)
	failure := NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorIdentityAlreadyRegistered), 0)
	charging := func(avps ...diameter.AVP) diameter.AVP {
		return diameter.Grouped(AVPChargingInformation, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
	}

	for name, ans := range map[string]*diameter.Message{
		"empty charging":  with(success, charging()),
		"secondary only":  with(success, charging(vendorString(AVPSecondaryEventChargingFunctionName, "aaa://ocs2.example.org"))),
		"bad server name": with(success, vendorString(AVPServerName, "scscf")),
		"bad wildcard":    with(success, diameter.UTF8String(AVPWildcardedPublicIdentity, 0, tgpp.VendorID, "x")),
		"empty WWSF name": with(success, diameter.Grouped(AVPAllowedWAFWWSFIdentities, 0, tgpp.VendorID,
			diameter.UTF8String(AVPWebRTCWebServerFunctionName, 0, tgpp.VendorID, ""))),
		"WAF not grouped": with(success, diameter.OctetString(AVPAllowedWAFWWSFIdentities, 0, tgpp.VendorID, []byte{1})),
	} {
		if _, err := ParseServerAssignmentAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	_, err := ParseServerAssignmentAnswer(with(failure, vendorString(AVPServerName, "scscf"), userName(testPrivate)))

	var re *ServerAssignmentError
	if !errors.As(err, &re) || !re.IsExperimental(tgpp.ResultErrorIdentityAlreadyRegistered) || re.ServerName != "" || re.PrivateIdentity != testPrivate {
		t.Fatalf("error answer with a bad Server-Name = %#v", err)
	}
}
