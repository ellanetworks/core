// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sgd

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

var (
	smscIdentity = diameter.Identity{OriginHost: "smsc.example.org", OriginRealm: "example.org"}
	mmeIdentity  = diameter.Identity{OriginHost: "mme.example.org", OriginRealm: "epc.example.org"}
	tfrEnvelope  = tgpp.Envelope{
		SessionID:        "smsc.example.org;1;1",
		Origin:           smscIdentity,
		DestinationHost:  "mme.example.org",
		DestinationRealm: "epc.example.org",
	}
	ofrEnvelope = tgpp.Envelope{
		SessionID:        "mme.example.org;1;1",
		Origin:           mmeIdentity,
		DestinationRealm: "example.org",
	}
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func roundTrip(t *testing.T, m *diameter.Message) *diameter.Message {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := diameter.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return out
}

func resultCode(t *testing.T, err error) uint32 {
	t.Helper()

	var avpErr *diameter.AVPError
	if !errors.As(err, &avpErr) {
		t.Fatalf("err = %v, want an AVP error", err)
	}

	return avpErr.ResultCode
}

func request(command uint32, env tgpp.Envelope, avps ...diameter.AVP) *diameter.Message {
	return &diameter.Message{
		Flags:         diameter.FlagRequest | diameter.FlagProxiable,
		CommandCode:   command,
		ApplicationID: ApplicationID,
		AVPs:          append(env.AVPs(), avps...),
	}
}

func absent(v tgpp.AbsentUserDiagnostic) *tgpp.AbsentUserDiagnostic {
	return &v
}

func TestParseAnswers(t *testing.T) {
	req := request(CommandMTForwardShortMessage, tfrEnvelope)

	ok, err := NewMTForwardShortMessageAnswer(req, mmeIdentity, []byte{0x00, 0x01})
	if err != nil {
		t.Fatal(err)
	}

	if a, err := ParseMTForwardShortMessageAnswer(roundTrip(t, ok)); err != nil || string(a.SMRPUI) != "\x00\x01" {
		t.Fatalf("success = %+v, %v", a, err)
	}

	failure, err := NewDeliveryFailureAnswer(req, mmeIdentity, CauseMemoryCapacityExceeded, []byte{0x02})
	if err != nil {
		t.Fatal(err)
	}

	failure.AVPs = append(failure.AVPs, smRPUIAVP([]byte{0x03}))

	_, err = ParseMTForwardShortMessageAnswer(roundTrip(t, failure))
	if !tgpp.IsExperimental(err, tgpp.ResultErrorSMDeliveryFailure) {
		t.Fatalf("err = %v", err)
	}

	var re *ResultError
	if !errors.As(err, &re) || re.DeliveryFailureCause == nil || *re.DeliveryFailureCause != CauseMemoryCapacityExceeded ||
		string(re.DiagnosticInfo) != "\x02" || string(re.SMRPUI) != "\x03" {
		t.Fatalf("result error = %+v", re)
	}

	_, err = ParseMTForwardShortMessageAnswer(roundTrip(t, NewAbsentUserAnswer(req, mmeIdentity, absent(tgpp.AbsentUserIMSIDetached))))
	if !errors.As(err, &re) || !re.IsExperimental(tgpp.ResultErrorAbsentUser) || re.AbsentUserDiagnostic == nil || *re.AbsentUserDiagnostic != 1 {
		t.Fatalf("absent user = %v", err)
	}

	for _, code := range []uint32{tgpp.ResultErrorUserBusyForMTSMS, tgpp.ResultErrorIllegalUser, tgpp.ResultErrorIllegalEquipment, tgpp.ResultErrorUserUnknown} {
		if _, err := ParseMTForwardShortMessageAnswer(tgpp.NewExperimentalAnswer(req, mmeIdentity, code)); !tgpp.IsExperimental(err, code) {
			t.Errorf("code %d: err = %v", code, err)
		}
	}

	if _, err := ParseMOForwardShortMessageAnswer(tgpp.NewAnswer(req, smscIdentity, diameter.ResultUnableToComply)); !errors.As(err, &re) || re.Experimental {
		t.Fatalf("base error = %v", err)
	}
}

func TestParseAnswersMalformed(t *testing.T) {
	experimental := func(avps ...diameter.AVP) *diameter.Message {
		m := tgpp.NewExperimentalAnswer(request(CommandMTForwardShortMessage, tfrEnvelope), mmeIdentity, tgpp.ResultErrorSMDeliveryFailure)
		m.AVPs = append(m.AVPs, avps...)

		return m
	}

	for name, ans := range map[string]*diameter.Message{
		"no result":          {},
		"cause not grouped":  experimental(diameter.OctetString(AVPSMDeliveryFailureCause, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})),
		"cause without enum": experimental(diameter.Grouped(AVPSMDeliveryFailureCause, diameter.AVPFlagMandatory, tgpp.VendorID)),
		"short diagnostic":   experimental(diameter.OctetString(tgpp.AVPAbsentUserDiagnosticSM, diameter.AVPFlagMandatory, tgpp.VendorID, []byte{1})),
	} {
		if _, err := ParseMTForwardShortMessageAnswer(ans); !errors.Is(err, ErrMalformedAnswer) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestAnswerBuildersValidate(t *testing.T) {
	req := request(CommandMTForwardShortMessage, tfrEnvelope)

	if _, err := NewDeliveryFailureAnswer(req, mmeIdentity, 9, nil); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("unknown cause: %v", err)
	}

	if _, err := NewMTForwardShortMessageAnswer(req, mmeIdentity, make([]byte, MaxSMRPUILength+1)); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("oversized SM-RP-UI: %v", err)
	}

	if _, err := NewMOForwardShortMessageAnswer(req, smscIdentity, make([]byte, MaxSMRPUILength+1)); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("oversized SM-RP-UI: %v", err)
	}

	ans, err := NewMOForwardShortMessageAnswer(req, smscIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := ans.Find(AVPSMRPUI, tgpp.VendorID); ok {
		t.Fatal("SM-RP-UI sent without one")
	}
}

func TestDeliveryFailureCauseString(t *testing.T) {
	for cause, want := range map[DeliveryFailureCause]string{
		CauseMemoryCapacityExceeded: "MEMORY_CAPACITY_EXCEEDED",
		CauseEquipmentProtocolError: "EQUIPMENT_PROTOCOL_ERROR",
		CauseEquipmentNotSMEquipped: "EQUIPMENT_NOT_SM_EQUIPPED",
		CauseUnknownServiceCentre:   "UNKNOWN_SERVICE_CENTRE",
		CauseSCCongestion:           "SC_CONGESTION",
		CauseInvalidSMEAddress:      "INVALID_SME_ADDRESS",
		CauseUserNotSCUser:          "USER_NOT_SC_USER",
		7:                           "DeliveryFailureCause(7)",
	} {
		if got := cause.String(); got != want {
			t.Errorf("DeliveryFailureCause(%d).String() = %q, want %q", uint32(cause), got, want)
		}
	}
}
