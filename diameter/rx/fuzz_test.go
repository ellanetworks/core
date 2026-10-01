// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

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
		must(NewAARequest(afEnvelope, fullAARequest())),
		must(NewAARequest(afEnvelope, AARequest{
			NoStateMaintained: true, RequestType: ptr(RequestPCSCFRestoration),
			SubscriptionIDs: []SubscriptionID{{Type: SubscriptionIMSI, Data: "001010000000001"}},
		})),
		must(NewSessionTerminationRequest(afEnvelope, SessionTerminationRequest{Cause: TerminationLogout})),
		must(NewReAuthRequest(pcrfEnvelope, ReAuthRequest{
			SpecificActions: []SpecificAction{ActionChargingCorrelationExchange, ActionIndicationOfReleaseOfBearer},
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{
				{Value: []byte{1, 2, 3, 4}, Flows: []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1}}}},
			},
			AccessNetworkChargingAddress: netip.MustParseAddr("10.0.0.1"),
			Flows:                        []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1, 2}}},
			SubscriptionIDs:              []SubscriptionID{{Type: SubscriptionE164, Data: "15551230002"}},
			AbortCause:                   ptr(AbortInsufficientBearerResources),
		})),
		must(NewAbortSessionRequest(pcrfEnvelope, AbortSessionRequest{Cause: AbortPCEFFailure})),
	)

	f.Fuzz(func(t *testing.T, b []byte) {
		m, err := diameter.Unmarshal(b)
		if err != nil {
			return
		}

		aar, err := ParseAARequest(m)
		requireAVPError(t, err)
		rebuilds(t, aar, err, func(r AARequest) (*diameter.Message, error) { return NewAARequest(afEnvelope, r) }, ParseAARequest)

		str, err := ParseSessionTerminationRequest(m)
		requireAVPError(t, err)
		rebuilds(t, str, err, func(r SessionTerminationRequest) (*diameter.Message, error) {
			return NewSessionTerminationRequest(afEnvelope, r)
		}, ParseSessionTerminationRequest)

		rar, err := ParseReAuthRequest(m)
		requireAVPError(t, err)
		rebuilds(t, rar, err, func(r ReAuthRequest) (*diameter.Message, error) { return NewReAuthRequest(pcrfEnvelope, r) }, ParseReAuthRequest)

		asr, err := ParseAbortSessionRequest(m)
		requireAVPError(t, err)
		rebuilds(t, asr, err, func(r AbortSessionRequest) (*diameter.Message, error) {
			return NewAbortSessionRequest(pcrfEnvelope, r)
		}, ParseAbortSessionRequest)
	})
}

func FuzzParseAnswers(f *testing.F) {
	must := mustMessage(f)
	aar := request(CommandAA, afEnvelope)

	fuzzSeeds(f,
		must(NewAAAnswer(aar, pcrfIdentity, AAAnswer{
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{{Value: []byte{1}, Flows: []Flows{{MediaComponentNumber: 1}}}},
			AccessNetworkChargingAddress:     netip.MustParseAddr("2001:db8::1"),
			SubscriptionIDs:                  []SubscriptionID{{Type: SubscriptionIMSI, Data: "001010000000001"}},
			Features:                         FeatureRel8 | FeaturePCSCFRestorationEnhancement,
		})),
		must(NewAAErrorAnswer(aar, pcrfIdentity, AAError{
			ResultError: ResultError{Result: tgpp.Experimental(tgpp.ResultRequestedServiceTemporarilyNotAuthorized)},
			AcceptableServiceInfo: &AcceptableServiceInfo{
				MediaComponents:         []MediaBandwidth{{MediaComponentNumber: 1, MaxRequestedBandwidthUL: ptr(uint32(1))}},
				MaxRequestedBandwidthDL: ptr(uint32(2)),
			},
			RetryInterval: time.Minute,
			Features:      FeatureRel8,
		})),
		NewAnswer(request(CommandReAuth, pcrfEnvelope), afIdentity, tgpp.Result{Code: diameter.ResultUnknownSessionID}),
		must(NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, SessionTerminationAnswer{})),
		must(NewAbortSessionAnswer(request(CommandAbortSession, pcrfEnvelope), afIdentity, AbortSessionAnswer{})),
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

		aaa, err := ParseAAAnswer(m)
		check(err)
		rebuilds(t, aaa, err, func(a AAAnswer) (*diameter.Message, error) { return NewAAAnswer(aar, pcrfIdentity, a) }, ParseAAAnswer)

		var aae *AAError
		if errors.As(err, &aae) {
			rebuilt, buildErr := NewAAErrorAnswer(aar, pcrfIdentity, *aae)
			if buildErr != nil {
				t.Fatalf("parsed %+v does not rebuild: %v", aae, buildErr)
			}

			var again *AAError
			if _, err := ParseAAAnswer(rebuilt); !errors.As(err, &again) || !reflect.DeepEqual(*again, *aae) {
				t.Fatalf("rebuilt %+v parses as %v", aae, err)
			}
		}

		raa, err := ParseReAuthAnswer(m)
		check(err)
		rebuilds(t, raa, err, func(a ReAuthAnswer) (*diameter.Message, error) {
			return NewReAuthAnswer(request(CommandReAuth, pcrfEnvelope), afIdentity, a)
		}, ParseReAuthAnswer)

		sta, err := ParseSessionTerminationAnswer(m)
		check(err)
		rebuilds(t, sta, err, func(a SessionTerminationAnswer) (*diameter.Message, error) {
			return NewSessionTerminationAnswer(request(CommandSessionTermination, afEnvelope), pcrfIdentity, a)
		}, ParseSessionTerminationAnswer)

		asa, err := ParseAbortSessionAnswer(m)
		check(err)
		rebuilds(t, asa, err, func(a AbortSessionAnswer) (*diameter.Message, error) {
			return NewAbortSessionAnswer(request(CommandAbortSession, pcrfEnvelope), afIdentity, a)
		}, ParseAbortSessionAnswer)
	})
}

func FuzzParseFlowDescription(f *testing.F) {
	for _, s := range []string{
		"permit out 17 from 10.4.128.21 30000 to 192.168.101.4 1234",
		"permit in ip from 192.168.101.5 5060 to 10.4.128.21 5060",
		"permit out 6 from 2001:db8::/64 to any",
		"deny in 17 from any to any frag",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseFlowDescription(s)
		if err != nil {
			if !errors.Is(err, ErrInvalidFlowDescription) {
				t.Fatalf("ParseFlowDescription returned %v", err)
			}

			return
		}

		text, err := d.MarshalText()
		if err != nil {
			t.Fatalf("parsed %+v does not marshal: %v", d, err)
		}

		if again, err := ParseFlowDescription(string(text)); err != nil || again != d {
			t.Fatalf("%q re-parsed as %+v, %v", text, again, err)
		}
	})
}
