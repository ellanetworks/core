// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestMOForwardShortMessageRoundTrip(t *testing.T) {
	m := MOForwardShortMessage{
		ServiceCentreAddress: "15550000000",
		User:                 tgpp.UserIdentifier{IMSI: "001010000000001", MSISDN: "15551230002"},
		SMRPUI:               []byte{0x01, 0x02, 0x03},
	}

	req, err := NewMOForwardShortMessageRequest(ofrEnvelope, m)
	if err != nil {
		t.Fatal(err)
	}

	if req.CommandCode != CommandMOForwardShortMessage || req.ApplicationID != ApplicationID {
		t.Fatalf("header = %+v", req)
	}

	if _, ok := req.Find(diameter.AVPDestinationHost, 0); ok {
		t.Fatal("OFR routed by host")
	}

	got, err := ParseMOForwardShortMessageRequest(roundTrip(t, req))
	if err != nil || !reflect.DeepEqual(got, m) {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
}

func TestMOForwardShortMessageValidation(t *testing.T) {
	user := tgpp.UserIdentifier{IMSI: "001010000000001"}

	for name, m := range map[string]MOForwardShortMessage{
		"no identity":    {ServiceCentreAddress: "15550000000", SMRPUI: []byte{1}},
		"no SC address":  {User: user, SMRPUI: []byte{1}},
		"no SM-RP-UI":    {ServiceCentreAddress: "15550000000", User: user},
		"large SM-RP-UI": {ServiceCentreAddress: "15550000000", User: user, SMRPUI: make([]byte, MaxSMRPUILength+1)},
	} {
		if _, err := NewMOForwardShortMessageRequest(ofrEnvelope, m); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParseMOForwardShortMessageRequestErrors(t *testing.T) {
	ui, err := tgpp.NewUserIdentifier(tgpp.UserIdentifier{IMSI: "001010000000001"})
	if err != nil {
		t.Fatal(err)
	}

	sc := diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155000000f0"))
	smRPUI := smRPUIAVP([]byte{1})

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no SM-RP-UI":       {request(CommandMOForwardShortMessage, ofrEnvelope, sc, ui), diameter.ResultMissingAVP},
		"empty SM-RP-UI":    {request(CommandMOForwardShortMessage, ofrEnvelope, sc, ui, smRPUIAVP(nil)), diameter.ResultInvalidAVPValue},
		"large SM-RP-UI":    {request(CommandMOForwardShortMessage, ofrEnvelope, sc, ui, smRPUIAVP(make([]byte, 201))), diameter.ResultInvalidAVPValue},
		"bad SC address":    {request(CommandMOForwardShortMessage, ofrEnvelope, diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{0xba}), ui, smRPUI), diameter.ResultInvalidAVPValue},
		"empty identity":    {request(CommandMOForwardShortMessage, ofrEnvelope, sc, diameter.Grouped(tgpp.AVPUserIdentifier, diameter.AVPFlagMandatory, tgpp.VendorID), smRPUI), diameter.ResultInvalidAVPValue},
		"repeated SM-RP-UI": {request(CommandMOForwardShortMessage, ofrEnvelope, sc, ui, smRPUI, smRPUI), diameter.ResultAVPOccursTooManyTimes},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMOForwardShortMessageRequest(tt.req)
			if code := resultCode(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}

	withFlags := request(CommandMOForwardShortMessage, ofrEnvelope, sc, ui, smRPUI,
		diameter.Unsigned32(AVPOFRFlags, 0, tgpp.VendorID, 1),
		diameter.Unsigned32(diameter.AVPOriginStateID, diameter.AVPFlagMandatory, 0, 1))
	if _, err := ParseMOForwardShortMessageRequest(withFlags); err != nil {
		t.Fatalf("OFR-Flags and Origin-State-Id rejected: %v", err)
	}
}
