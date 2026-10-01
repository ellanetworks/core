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
	})
}

func FuzzParseAnswers(f *testing.F) {
	must := mustMessage(f)
	req := request(CommandServerAssignment, vendorUnsigned(AVPServerAssignmentType, AssignmentRegistration))
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
			Result: success, EmergencyRegistrations: []EmergencyRegistration{{PrivateIdentity: testPrivate, PublicIdentity: testPublic}},
		})),
		NewAnswer(req, hssIdentity, tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch), FeatureIMSRestoration),
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

		_, err = ParseUserAuthorizationAnswer(m)
		check(err)

		_, err = ParseLocationInfoAnswer(m)
		check(err)

		_, err = ParseMultimediaAuthAnswer(m)
		check(err)

		_, err = ParseServerAssignmentAnswer(m)
		check(err)

		_, err = ParseRegistrationTerminationAnswer(m)
		check(err)
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
