// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func fuzzSeeds(f *testing.F, msgs ...*diameter.Message) {
	f.Helper()

	for _, m := range msgs {
		b, err := m.Marshal()
		if err != nil {
			f.Fatal(err)
		}

		f.Add(b)
	}

	f.Add([]byte{})
}

func requireAVPError(t *testing.T, err error) {
	t.Helper()

	var avpErr *diameter.AVPError
	if err != nil && !errors.As(err, &avpErr) {
		t.Fatalf("request parser returned %v, want an AVP error", err)
	}
}

func rebuilds[T any](t *testing.T, parsed T, err error, build func(T) (*diameter.Message, error), parse func(*diameter.Message) (T, error)) {
	t.Helper()

	if err != nil {
		return
	}

	m, err := build(parsed)
	if err != nil {
		t.Fatalf("parsed %+v does not rebuild: %v", parsed, err)
	}

	again, err := parse(m)
	if err != nil || !reflect.DeepEqual(again, parsed) {
		t.Fatalf("rebuilt %+v parses as %+v, %v", parsed, again, err)
	}
}

func rebuildsError[E any, P interface {
	*E
	error
}](t *testing.T, parsed P, build func(E) (*diameter.Message, error), parse func(*diameter.Message) error) {
	t.Helper()

	m, err := build(*parsed)
	if err != nil {
		t.Fatalf("parsed %+v does not rebuild: %v", parsed, err)
	}

	var again P
	if err := parse(m); !errors.As(err, &again) || !reflect.DeepEqual(*again, *parsed) {
		t.Fatalf("rebuilt %+v parses as %v", parsed, err)
	}
}

func FuzzParseRequests(f *testing.F) {
	must := mustMessage(f)

	fuzzSeeds(f,
		must(NewUserAuthorizationRequest(cscfEnvelope, UserAuthorizationRequest{
			PrivateIdentity: testPrivate, PublicIdentity: testPublic, VisitedNetwork: testRealm,
			AuthorizationType: AuthorizationRegistrationAndCapabilities, EmergencyRegistration: true, Features: FeatureIMSRestoration,
		})),
		must(NewLocationInfoRequest(cscfEnvelope, LocationInfoRequest{PublicIdentity: testTel, Originating: true})),
		must(NewMultimediaAuthRequest(cscfEnvelope, MultimediaAuthRequest{
			PrivateIdentity: testPrivate, PublicIdentity: testPublic, ServerName: testServer, NumberOfItems: 2, Scheme: SchemeDigestAKAv1MD5,
			Resync: &Resync{RAND: octets(16, 1), AUTS: octets(14, 2)},
		})),
		must(NewServerAssignmentRequest(cscfEnvelope, ServerAssignmentRequest{
			PrivateIdentity: testPrivate, PublicIdentities: []string{testPublic, testTel}, ServerName: testServer, Type: AssignmentUserDeregistration,
		})),
		must(NewRegistrationTerminationRequest(hssEnvelope, RegistrationTerminationRequest{
			PrivateIdentity: testPrivate, AssociatedIdentities: []string{"b@example.org"}, PublicIdentities: []string{testPublic},
			Reason: DeregistrationReason{Code: ReasonNewServerAssigned, Info: "moved"},
		})),
		must(NewPushProfileRequest(hssEnvelope, PushProfileRequest{
			PrivateIdentity: testPrivate, UserData: []byte("<IMSSubscription/>"),
			Charging:      &ChargingInformation{PrimaryChargingCollectionFunction: "aaa://cdf.example.org"},
			AllowedWebRTC: &AllowedWebRTCFunctions{AuthenticationFunctions: []string{"waf"}, WebServerFunctions: []string{"wwsf"}},
		})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		uar, err := ParseUserAuthorizationRequest(m)
		requireAVPError(t, err)
		rebuilds(t, uar, err, func(r UserAuthorizationRequest) (*diameter.Message, error) {
			return NewUserAuthorizationRequest(cscfEnvelope, r)
		}, ParseUserAuthorizationRequest)

		lir, err := ParseLocationInfoRequest(m)
		requireAVPError(t, err)
		rebuilds(t, lir, err, func(r LocationInfoRequest) (*diameter.Message, error) {
			return NewLocationInfoRequest(cscfEnvelope, r)
		}, ParseLocationInfoRequest)

		mar, err := ParseMultimediaAuthRequest(m)
		requireAVPError(t, err)
		rebuilds(t, mar, err, func(r MultimediaAuthRequest) (*diameter.Message, error) {
			return NewMultimediaAuthRequest(cscfEnvelope, r)
		}, ParseMultimediaAuthRequest)

		sar, err := ParseServerAssignmentRequest(m)
		requireAVPError(t, err)
		rebuilds(t, sar, err, func(r ServerAssignmentRequest) (*diameter.Message, error) {
			return NewServerAssignmentRequest(cscfEnvelope, r)
		}, ParseServerAssignmentRequest)

		rtr, err := ParseRegistrationTerminationRequest(m)
		requireAVPError(t, err)
		rebuilds(t, rtr, err, func(r RegistrationTerminationRequest) (*diameter.Message, error) {
			return NewRegistrationTerminationRequest(hssEnvelope, r)
		}, ParseRegistrationTerminationRequest)

		ppr, err := ParsePushProfileRequest(m)
		requireAVPError(t, err)

		if len(ppr.UserData) > 0 || ppr.Charging != nil || ppr.AllowedWebRTC != nil {
			rebuilds(t, ppr, err, func(r PushProfileRequest) (*diameter.Message, error) {
				return NewPushProfileRequest(hssEnvelope, r)
			}, ParsePushProfileRequest)
		}
	})
}

func FuzzParseAnswers(f *testing.F) {
	must := mustMessage(f)
	req := request(CommandServerAssignment, vendorUnsigned(AVPServerAssignmentType, uint32(AssignmentRegistration)))
	bare := request(CommandServerAssignment)
	success := tgpp.Result{Code: diameter.ResultSuccess}

	fuzzSeeds(f,
		must(NewUserAuthorizationAnswer(req, hssIdentity, UserAuthorization{
			Result: tgpp.Experimental(tgpp.ResultFirstRegistration), Capabilities: &ServerCapabilities{Mandatory: []uint32{1}, ServerNames: []string{testServer}},
		})),
		must(NewLocationInfoAnswer(req, hssIdentity, LocationInfo{Result: success, ServerName: testServer, PSIDirectRouting: true})),
		must(NewMultimediaAuthAnswer(req, hssIdentity, MultimediaAuth{
			Result: success, PrivateIdentity: testPrivate, PublicIdentity: testPublic,
			Items: []AuthItem{{ItemNumber: 1, Scheme: SchemeDigestAKAv1MD5, AKA: akaVector(1)}},
		})),
		must(NewServerAssignmentAnswer(req, hssIdentity, ServerAssignment{
			Result: success, PrivateIdentity: testPrivate, UserData: []byte("<IMSSubscription/>"),
			Charging: &ChargingInformation{PrimaryChargingCollectionFunction: "aaa://cdf.example.org"}, AssociatedIdentities: []string{testPrivate},
		})),
		must(NewRegistrationTerminationAnswer(req, cscfIdentity, RegistrationTermination{
			Result: success, EmergencyIdentities: []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: testPublic}},
		})),
		NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch), FeatureIMSRestoration),
		must(NewPushProfileAnswer(req, cscfIdentity, PushProfile{Features: FeatureAliasIndication})),
		must(NewServerAssignmentErrorAnswer(req, hssIdentity, ServerAssignmentError{
			ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultErrorIdentityAlreadyRegistered)}, PrivateIdentity: testPrivate, ServerName: testServer,
		})),
		must(NewRegistrationTerminationErrorAnswer(req, cscfIdentity, RegistrationTerminationError{
			ResultError:         ResultError{Result: tgpp.Result{Code: diameter.ResultUnableToComply}},
			EmergencyIdentities: []EmergencyIdentity{{PrivateIdentity: testPrivate, PublicIdentity: testPublic}},
		})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		check := func(err error) {
			var re *ResultError
			if err != nil && !errors.As(err, &re) && !errors.Is(err, ErrMalformedAnswer) {
				t.Fatalf("answer parser returned %v", err)
			}
		}

		uaa, err := ParseUserAuthorizationAnswer(m)
		check(err)
		rebuilds(t, uaa, err, func(a UserAuthorization) (*diameter.Message, error) {
			return NewUserAuthorizationAnswer(req, hssIdentity, a)
		}, ParseUserAuthorizationAnswer)

		lia, err := ParseLocationInfoAnswer(m)
		check(err)
		rebuilds(t, lia, err, func(a LocationInfo) (*diameter.Message, error) {
			return NewLocationInfoAnswer(req, hssIdentity, a)
		}, ParseLocationInfoAnswer)

		maa, err := ParseMultimediaAuthAnswer(m)
		check(err)

		if err == nil && !slices.ContainsFunc(maa.Items, func(i AuthItem) bool { return i.AKA == nil }) {
			rebuilds(t, maa, err, func(a MultimediaAuth) (*diameter.Message, error) {
				return NewMultimediaAuthAnswer(req, hssIdentity, a)
			}, ParseMultimediaAuthAnswer)
		}

		saa, err := ParseServerAssignmentAnswer(m)
		check(err)
		rebuilds(t, saa, err, func(a ServerAssignment) (*diameter.Message, error) {
			return NewServerAssignmentAnswer(bare, hssIdentity, a)
		}, ParseServerAssignmentAnswer)

		var sae *ServerAssignmentError
		if errors.As(err, &sae) {
			rebuildsError(t, sae, func(e ServerAssignmentError) (*diameter.Message, error) {
				return NewServerAssignmentErrorAnswer(bare, hssIdentity, e)
			}, func(m *diameter.Message) error { _, err := ParseServerAssignmentAnswer(m); return err })
		}

		ppa, err := ParsePushProfileAnswer(m)
		check(err)
		rebuilds(t, ppa, err, func(a PushProfile) (*diameter.Message, error) {
			return NewPushProfileAnswer(bare, cscfIdentity, a)
		}, ParsePushProfileAnswer)

		rta, err := ParseRegistrationTerminationAnswer(m)
		check(err)
		rebuilds(t, rta, err, func(a RegistrationTermination) (*diameter.Message, error) {
			return NewRegistrationTerminationAnswer(bare, cscfIdentity, a)
		}, ParseRegistrationTerminationAnswer)

		var rte *RegistrationTerminationError
		if errors.As(err, &rte) {
			rebuildsError(t, rte, func(e RegistrationTerminationError) (*diameter.Message, error) {
				return NewRegistrationTerminationErrorAnswer(bare, cscfIdentity, e)
			}, func(m *diameter.Message) error { _, err := ParseRegistrationTerminationAnswer(m); return err })
		}
	})
}

func FuzzParseUserData(f *testing.F) {
	full, err := MarshalUserData(fullSubscription())
	if err != nil {
		f.Fatal(err)
	}

	f.Add(full)
	f.Add([]byte(peerProfile))
	f.Add([]byte("<IMSSubscription/>"))

	f.Fuzz(func(t *testing.T, b []byte) {
		s, err := ParseUserData(b)
		if err != nil {
			if !errors.Is(err, ErrInvalidUserData) {
				t.Fatalf("ParseUserData returned %v", err)
			}

			return
		}

		out, err := MarshalUserData(s)
		if err != nil {
			t.Fatalf("parsed %+v does not marshal: %v", s, err)
		}

		again, err := ParseUserData(out)
		if err != nil || !reflect.DeepEqual(again, s) {
			t.Fatalf("re-parsed %+v as %+v, %v", s, again, err)
		}
	})
}
