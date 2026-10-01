// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"errors"
	"net/netip"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestReAuthRequestRoundTrip(t *testing.T) {
	for name, r := range map[string]ReAuthRequest{
		"release of bearer": {
			SpecificActions: []SpecificAction{ActionIndicationOfReleaseOfBearer},
			Flows:           []Flows{{MediaComponentNumber: 1, FlowNumbers: []uint32{1, 2}}},
			AbortCause:      ptr(AbortBearerReleased),
		},
		"charging correlation": {
			SpecificActions: []SpecificAction{ActionChargingCorrelationExchange},
			AccessNetworkChargingIdentifiers: []AccessNetworkChargingIdentifier{
				{Value: []byte{0xde, 0xad, 0xbe, 0xef}, Flows: []Flows{{MediaComponentNumber: 1}}},
			},
			AccessNetworkChargingAddress: netip.MustParseAddr("2001:db8::1"),
			SubscriptionIDs:              []SubscriptionID{{Type: SubscriptionE164, Data: "15551230002"}},
		},
		"IP-CAN change": {SpecificActions: []SpecificAction{ActionIPCANChange, ActionAccessNetworkInfoReport}},
	} {
		t.Run(name, func(t *testing.T) {
			req, err := NewReAuthRequest(pcrfEnvelope, r)
			if err != nil {
				t.Fatal(err)
			}

			if host, ok := req.Find(diameter.AVPDestinationHost, 0); !ok || host.UTF8String() != afIdentity.OriginHost {
				t.Fatalf("Destination-Host = %+v", host)
			}

			got, err := ParseReAuthRequest(roundTrip(t, req))
			if err != nil || !reflect.DeepEqual(got, r) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}
}

func TestReAuthRequestValidation(t *testing.T) {
	valid := ReAuthRequest{SpecificActions: []SpecificAction{ActionIndicationOfLossOfBearer}}

	if _, err := NewReAuthRequest(afEnvelope, valid); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("no destination host: err = %v", err)
	}

	for name, mutate := range map[string]func(*ReAuthRequest){
		"no specific action":       func(r *ReAuthRequest) { r.SpecificActions = nil },
		"Specific-Action":          func(r *ReAuthRequest) { r.SpecificActions = []SpecificAction{22} },
		"Abort-Cause":              func(r *ReAuthRequest) { r.AbortCause = ptr(AbortCause(6)) },
		"charging without the id":  func(r *ReAuthRequest) { r.SpecificActions = []SpecificAction{ActionChargingCorrelationExchange} },
		"empty charging id":        func(r *ReAuthRequest) { r.AccessNetworkChargingIdentifiers = []AccessNetworkChargingIdentifier{{}} },
		"bad subscription id type": func(r *ReAuthRequest) { r.SubscriptionIDs = []SubscriptionID{{Type: 5, Data: "x"}} },
	} {
		r := valid
		mutate(&r)

		if _, err := NewReAuthRequest(pcrfEnvelope, r); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseReAuthRequestErrors(t *testing.T) {
	base := request(CommandReAuth, pcrfEnvelope, vendorUnsigned(AVPSpecificAction, uint32(ActionIndicationOfReleaseOfBearer)))

	if _, err := ParseReAuthRequest(base); err != nil {
		t.Fatalf("base request: %v", err)
	}

	for name, tc := range map[string]struct {
		msg    *diameter.Message
		result uint32
	}{
		"no Destination-Host": {without(base, diameter.AVPDestinationHost, 0), diameter.ResultMissingAVP},
		"no Specific-Action":  {without(base, AVPSpecificAction, tgpp.VendorID), diameter.ResultMissingAVP},
		"Specific-Action":     {with(base, vendorUnsigned(AVPSpecificAction, 22)), diameter.ResultInvalidAVPValue},
		"Abort-Cause":         {with(base, vendorUnsigned(AVPAbortCause, 6)), diameter.ResultInvalidAVPValue},
		"charging without the id": {
			with(base, vendorUnsigned(AVPSpecificAction, uint32(ActionChargingCorrelationExchange))),
			diameter.ResultMissingAVP,
		},
		"charging id without value": {with(base, vendorGrouped(AVPAccessNetworkChargingIdentifier)), diameter.ResultMissingAVP},
		"charging address": {
			with(base, vendorOctets(AVPAccessNetworkChargingAddress, []byte{0, 1, 10, 0})),
			diameter.ResultInvalidAVPValue,
		},
		"Flows without number": {with(base, vendorGrouped(AVPFlows)), diameter.ResultMissingAVP},
		"short Flow-Number": {
			with(base, vendorGrouped(AVPFlows, vendorUnsigned(AVPMediaComponentNumber, 1), vendorOctets(AVPFlowNumber, []byte{1}))),
			diameter.ResultInvalidAVPValue,
		},
		"Subscription-Id not grouped": {
			with(base, diameter.OctetString(AVPSubscriptionID, diameter.AVPFlagMandatory, 0, []byte{1})),
			diameter.ResultInvalidAVPValue,
		},
		"unknown mandatory AVP": {with(base, vendorUnsigned(999, 1)), diameter.ResultAVPUnsupported},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseReAuthRequest(tc.msg)
			if got := avpError(t, err).ResultCode; got != tc.result {
				t.Fatalf("result = %d, want %d (%v)", got, tc.result, err)
			}
		})
	}
}

func TestParseReAuthRequestToleratesBaseAVPs(t *testing.T) {
	req := request(CommandReAuth, pcrfEnvelope,
		vendorUnsigned(AVPSpecificAction, uint32(ActionIPCANChange)),
		diameter.Unsigned32(diameter.AVPReAuthRequestType, diameter.AVPFlagMandatory, 0, 0),
		vendorUnsigned(avpIPCANType, 5),
		diameter.Unsigned32(avpRATType, diameter.AVPFlagVendor, tgpp.VendorID, 1004),
		diameter.OctetString(diameter.AVPClass, diameter.AVPFlagMandatory, 0, []byte("c")),
	)

	if _, err := ParseReAuthRequest(req); err != nil {
		t.Fatal(err)
	}
}

func TestReAuthAnswer(t *testing.T) {
	req := request(CommandReAuth, pcrfEnvelope)

	for _, r := range []tgpp.Result{{Code: diameter.ResultSuccess}, tgpp.Experimental(2001)} {
		ans, err := NewReAuthAnswer(req, afIdentity, ReAuthAnswer{Result: r})
		if err != nil {
			t.Fatal(err)
		}

		if got, err := ParseReAuthAnswer(roundTrip(t, ans)); err != nil || got.Result != r {
			t.Fatalf("round trip = %+v, %v", got, err)
		}
	}
}
