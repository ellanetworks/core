// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func TestMTForwardShortMessageEncoding(t *testing.T) {
	start := time.Date(2026, 9, 27, 11, 59, 0, 0, time.UTC)

	tfr, err := NewMTForwardShortMessageRequest(tfrEnvelope, MTForwardShortMessage{
		IMSI:                 "001010000000002",
		ServiceCentreAddress: "15550000000",
		SMRPUI:               []byte{0x04, 0x01},
		MMENumberForMTSMS:    "15550000010",
		DeliveryTimer:        30 * time.Second,
		DeliveryStartTime:    start,
	})
	if err != nil {
		t.Fatal(err)
	}

	if tfr.CommandCode != CommandMTForwardShortMessage || tfr.ApplicationID != ApplicationID ||
		tfr.Flags != diameter.FlagRequest|diameter.FlagProxiable || tfr.AVPs[0].Code != diameter.AVPSessionID {
		t.Fatalf("TFR header = %+v", tfr)
	}

	checks := map[string]struct {
		code, vendor uint32
		want         []byte
		mandatory    bool
	}{
		"Destination-Host":       {diameter.AVPDestinationHost, 0, []byte("mme.example.org"), true},
		"User-Name":              {diameter.AVPUserName, 0, []byte("001010000000002"), true},
		"SC-Address":             {tgpp.AVPSCAddress, tgpp.VendorID, mustHex(t, "5155000000f0"), true},
		"SM-RP-UI":               {AVPSMRPUI, tgpp.VendorID, []byte{0x04, 0x01}, true},
		"MME-Number-for-MT-SMS":  {tgpp.AVPMMENumberForMTSMS, tgpp.VendorID, mustHex(t, "5155000010f0"), false},
		"SM-Delivery-Timer":      {AVPSMDeliveryTimer, tgpp.VendorID, mustHex(t, "0000001e"), true},
		"SM-Delivery-Start-Time": {AVPSMDeliveryStartTime, tgpp.VendorID, diameter.Time(0, 0, 0, start).Data, true},
	}

	for name, c := range checks {
		a, ok := tfr.Find(c.code, c.vendor)
		if !ok || !bytes.Equal(a.Data, c.want) || (a.Flags&diameter.AVPFlagMandatory != 0) != c.mandatory {
			t.Errorf("%s = %+v, want %x (M %v)", name, a, c.want, c.mandatory)
		}
	}

	if _, ok := tfr.Find(AVPTFRFlags, tgpp.VendorID); ok {
		t.Error("TFR-Flags present with no further messages pending")
	}
}

func TestMTForwardShortMessageRoundTrip(t *testing.T) {
	for name, m := range map[string]MTForwardShortMessage{
		"MME": {
			IMSI: "001010000000002", ServiceCentreAddress: "15550000000", SMRPUI: []byte{0x04},
			MMENumberForMTSMS: "15550000010", MoreMessagesToSend: true, DeliveryTimer: time.Minute,
			DeliveryStartTime: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		"SGSN":    {IMSI: "001010000000002", ServiceCentreAddress: "15550000000", SMRPUI: []byte{0x04}, SGSNNumber: "15550000020"},
		"minimal": {IMSI: "001010000000002", ServiceCentreAddress: "15550000000", SMRPUI: []byte{0x04}},
	} {
		t.Run(name, func(t *testing.T) {
			tfr, err := NewMTForwardShortMessageRequest(tfrEnvelope, m)
			if err != nil {
				t.Fatal(err)
			}

			got, err := ParseMTForwardShortMessageRequest(roundTrip(t, tfr))
			if err != nil || !reflect.DeepEqual(got, m) {
				t.Fatalf("round trip = %+v, %v", got, err)
			}
		})
	}

	minimal, _ := NewMTForwardShortMessageRequest(tfrEnvelope, MTForwardShortMessage{IMSI: "001010000000002", ServiceCentreAddress: "1", SMRPUI: []byte{4}})
	for _, code := range []uint32{AVPSMDeliveryTimer, AVPSMDeliveryStartTime} {
		if _, ok := minimal.Find(code, tgpp.VendorID); ok {
			t.Errorf("AVP %d sent while unset", code)
		}
	}
}

func TestMTForwardShortMessageValidation(t *testing.T) {
	base := MTForwardShortMessage{IMSI: "001010000000002", ServiceCentreAddress: "15550000000", SMRPUI: []byte{0x04}}

	cases := map[string]func(*MTForwardShortMessage){
		"bad IMSI":          func(m *MTForwardShortMessage) { m.IMSI = "hss-id" },
		"bad SC address":    func(m *MTForwardShortMessage) { m.ServiceCentreAddress = "+1555" },
		"bad MME number":    func(m *MTForwardShortMessage) { m.MMENumberForMTSMS = "+1" },
		"no SM-RP-UI":       func(m *MTForwardShortMessage) { m.SMRPUI = nil },
		"large SM-RP-UI":    func(m *MTForwardShortMessage) { m.SMRPUI = make([]byte, MaxSMRPUILength+1) },
		"negative timer":    func(m *MTForwardShortMessage) { m.DeliveryTimer = -time.Second },
		"overflowing timer": func(m *MTForwardShortMessage) { m.DeliveryTimer = time.Duration(1<<33) * time.Second },
	}

	for name, mutate := range cases {
		m := base
		mutate(&m)

		if _, err := NewMTForwardShortMessageRequest(tfrEnvelope, m); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: err = %v", name, err)
		}
	}

	noHost := tfrEnvelope
	noHost.DestinationHost = ""

	if _, err := NewMTForwardShortMessageRequest(noHost, base); !errors.Is(err, ErrInvalidMessage) {
		t.Errorf("no Destination-Host: err = %v", err)
	}
}

func TestParseMTForwardShortMessageRequestErrors(t *testing.T) {
	userName := diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "001010000000002")
	sc := diameter.OctetString(tgpp.AVPSCAddress, diameter.AVPFlagMandatory, tgpp.VendorID, mustHex(t, "5155000000f0"))
	smRPUI := smRPUIAVP([]byte{1})
	noHost := tfrEnvelope
	noHost.DestinationHost = ""

	tests := map[string]struct {
		req  *diameter.Message
		want uint32
	}{
		"no Destination-Host": {request(CommandMTForwardShortMessage, noHost, userName, sc, smRPUI), diameter.ResultMissingAVP},
		"no User-Name":        {request(CommandMTForwardShortMessage, tfrEnvelope, sc, smRPUI), diameter.ResultMissingAVP},
		"bad IMSI": {request(CommandMTForwardShortMessage, tfrEnvelope,
			diameter.UTF8String(diameter.AVPUserName, diameter.AVPFlagMandatory, 0, "hss-id"), sc, smRPUI), diameter.ResultInvalidAVPValue},
		"large SM-RP-UI": {request(CommandMTForwardShortMessage, tfrEnvelope, userName, sc, smRPUIAVP(make([]byte, 201))), diameter.ResultInvalidAVPValue},
		"bad MME number": {request(CommandMTForwardShortMessage, tfrEnvelope, userName, sc, smRPUI,
			diameter.OctetString(tgpp.AVPMMENumberForMTSMS, 0, tgpp.VendorID, []byte{0xba})), diameter.ResultInvalidAVPValue},
		"short timer": {request(CommandMTForwardShortMessage, tfrEnvelope, userName, sc, smRPUI,
			diameter.OctetString(AVPSMDeliveryTimer, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})), diameter.ResultInvalidAVPValue},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMTForwardShortMessageRequest(tt.req)
			if code := resultCode(t, err); code != tt.want {
				t.Fatalf("Result-Code = %d, want %d", code, tt.want)
			}
		})
	}

	withRetransmission := request(CommandMTForwardShortMessage, tfrEnvelope, userName, sc, smRPUI,
		diameter.Time(AVPMaximumRetransmissionTime, 0, tgpp.VendorID, time.Now()),
		diameter.OctetString(AVPSMSGMSCAddress, 0, tgpp.VendorID, mustHex(t, "5155000000f0")))
	if _, err := ParseMTForwardShortMessageRequest(withRetransmission); err != nil {
		t.Fatalf("retransmission AVPs rejected: %v", err)
	}
}
