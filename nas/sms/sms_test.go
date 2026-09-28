// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sms

import (
	"bytes"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}

	return b
}

func TestCPWire(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  CPMessage
		wire string
	}{
		{"CP-DATA MO", &CPData{UserData: mustHex("000100079144775810065002aabb")}, "09010e000100079144775810065002aabb"},
		{"CP-DATA MT", &CPData{TransactionIdentifier: TransactionIdentifier{Value: 3}, UserData: mustHex("0102")}, "3901020102"},
		{"CP-ACK", &CPAck{TransactionIdentifier: TransactionIdentifier{Flag: true}}, "8904"},
		{"CP-ERROR", &CPError{TransactionIdentifier: TransactionIdentifier{Value: 6, Flag: true}, Cause: CPCauseProtocolErrorUnspecified}, "e9106f"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := tc.msg.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			if hex.EncodeToString(b) != tc.wire {
				t.Fatalf("encoded %x, want %s", b, tc.wire)
			}

			got, err := ParseCP(b)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.msg) {
				t.Fatalf("parsed %+v, want %+v", got, tc.msg)
			}
		})
	}
}

func TestCPAckAnswersCPData(t *testing.T) {
	data, err := ParseCP(mustHex("2901020102"))
	if err != nil {
		t.Fatal(err)
	}

	b, err := (&CPAck{TransactionIdentifier: data.TI().Peer()}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := mustHex("a904"); !bytes.Equal(b, want) {
		t.Fatalf("CP-ACK = % x, want % x", b, want)
	}
}

func TestCPRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire string
		want error
	}{
		{"too short", "09", nas.ErrTruncated},
		{"wrong PD", "0801", ErrProtocolDiscriminator},
		{"TI 7", "790104", ErrTransactionIdentifier},
		{"unknown type", "0902", ErrUnknownMessageType},
		{"CP-DATA no length", "0901", nas.ErrTruncated},
		{"CP-DATA truncated", "09010301", nas.ErrTruncated},
		{"CP-ERROR no cause", "0910", nas.ErrTruncated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseCP(mustHex(tc.wire)); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	if _, err := (&CPAck{TransactionIdentifier: TransactionIdentifier{Value: 7}}).MarshalBinary(); !errors.Is(err, ErrTransactionIdentifier) {
		t.Errorf("TI 7 encoded: err = %v", err)
	}
}

func TestRPWire(t *testing.T) {
	sc := E164Address("447785016005")

	for _, tc := range []struct {
		name string
		msg  RPMessage
		dir  nas.Direction
		wire string
	}{
		{
			"RP-DATA MO", &RPData{Direction: nas.DirectionUplink, Reference: 1, Destination: sc, UserData: mustHex("aabb")},
			nas.DirectionUplink, "000100079144775810065002aabb",
		},
		{
			"RP-DATA MT", &RPData{Direction: nas.DirectionDownlink, Reference: 0x42, Originator: sc, UserData: mustHex("04")},
			nas.DirectionDownlink, "01420791447758100650000104",
		},
		{"RP-ACK MT", &RPAck{Direction: nas.DirectionDownlink, Reference: 1}, nas.DirectionDownlink, "0301"},
		{
			"RP-ACK MO with user data", &RPAck{Direction: nas.DirectionUplink, Reference: 0x42, UserData: mustHex("0000")},
			nas.DirectionUplink, "0242410200 00",
		},
		{
			"RP-ERROR MT", &RPError{Direction: nas.DirectionDownlink, Reference: 1, Cause: RPCauseTemporaryFailure},
			nas.DirectionDownlink, "05010129",
		},
		{
			"RP-ERROR MO with diagnostic and user data",
			&RPError{Direction: nas.DirectionUplink, Reference: 9, Cause: RPCauseMemoryCapacityExceeded, Diagnostic: []byte{3}, UserData: mustHex("01")},
			nas.DirectionUplink, "040902160341 0101",
		},
		{"RP-SMMA", &RPSMMA{Reference: 7}, nas.DirectionUplink, "0607"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := mustHex(stripSpaces(tc.wire))

			b, err := tc.msg.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(b, wire) {
				t.Fatalf("encoded % x, want % x", b, wire)
			}

			got, err := ParseRP(wire, tc.dir)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tc.msg) {
				t.Fatalf("parsed %+v, want %+v", got, tc.msg)
			}
		})
	}
}

func stripSpaces(s string) string {
	return string(bytes.ReplaceAll([]byte(s), []byte(" "), nil))
}

func TestRPDataBothAddressesAccepted(t *testing.T) {
	got, err := ParseRP(mustHex("0001028121079144775810065001aa"), nas.DirectionUplink)
	if err != nil {
		t.Fatal(err)
	}

	data, ok := got.(*RPData)
	if !ok || data.Originator == nil || data.Originator.Digits != "12" || data.Destination.String() != "+447785016005" {
		t.Fatalf("parsed %+v", got)
	}
}

func TestRPRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire string
		dir  nas.Direction
		want error
	}{
		{"too short", "00", nas.DirectionUplink, nas.ErrTruncated},
		{"MT RP-DATA uplink", "01010000", nas.DirectionUplink, ErrUnknownMessageType},
		{"MO RP-DATA downlink", "00010000", nas.DirectionDownlink, ErrUnknownMessageType},
		{"RP-SMMA downlink", "0601", nas.DirectionDownlink, ErrUnknownMessageType},
		{"MTI 7", "0701", nas.DirectionDownlink, ErrUnknownMessageType},
		{"RP-DATA truncated", "00010007914477", nas.DirectionUplink, nas.ErrTruncated},
		{"MO without destination", "0001000001aa", nas.DirectionUplink, ErrInvalidMandatoryIE},
		{"MT without originator", "0101000001aa", nas.DirectionDownlink, ErrInvalidMandatoryIE},
		{"MO with a 1-octet destination", "0001000191010a", nas.DirectionUplink, ErrInvalidMandatoryIE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseRP(mustHex(tc.wire), tc.dir)
			if m != nil || !errors.Is(err, tc.want) {
				t.Fatalf("ParseRP = %v, %v; want nil, %v", m, err, tc.want)
			}
		})
	}
}

func TestRPHeaderSurvivesAFailedParse(t *testing.T) {
	for _, tc := range []struct {
		wire string
		dir  nas.Direction
		mti  MessageTypeIndicator
	}{
		{"002a000191010a", nas.DirectionUplink, MTIDataMSToNetwork},
		{"012a", nas.DirectionUplink, MTIDataNetworkToMS},
	} {
		if _, err := ParseRP(mustHex(tc.wire), tc.dir); err == nil {
			t.Fatalf("ParseRP(%s) succeeded", tc.wire)
		}

		h, _ := ParseRPHeader(mustHex(tc.wire), tc.dir)
		if h.Reference != 0x2a || h.MTI != tc.mti {
			t.Fatalf("ParseRPHeader(%s) = %+v, want reference 0x2a", tc.wire, h)
		}
	}
}

func TestRPErrorWithInvalidCauseIsCause111(t *testing.T) {
	for _, wire := range []string{"050100", "0401", "040105"} {
		dir := MessageTypeIndicator(mustHex(wire)[0]).Direction()

		m, err := ParseRP(mustHex(wire), dir)
		if err == nil || !nas.SoftOnly(err) {
			t.Fatalf("ParseRP(%s): err = %v, want a soft error", wire, err)
		}

		e, ok := m.(*RPError)
		if !ok || e.Cause != RPCauseProtocolErrorUnspecified || e.Diagnostic != nil || e.UserData != nil || e.Reference != 1 {
			t.Fatalf("ParseRP(%s) = %+v, want RP-ERROR #111 for reference 1", wire, m)
		}
	}
}

func TestOverlongElementsAreNotSyntaxErrors(t *testing.T) {
	cp := append(mustHex("0901f9"), make([]byte, 249)...)

	m, err := ParseCP(cp)
	if err != nil || len(m.(*CPData).UserData) != 249 {
		t.Fatalf("249-octet CP-User data: %v", err)
	}

	if _, err := m.MarshalBinary(); !errors.Is(err, ErrElementTooLong) {
		t.Fatalf("re-encode: err = %v, want ErrElementTooLong", err)
	}

	for _, tc := range []struct {
		name string
		wire []byte
		dir  nas.Direction
	}{
		{"RP-DATA user data", append(mustHex("00010003912143e9"), make([]byte, 233)...), nas.DirectionUplink},
		{"RP-ACK user data", append(mustHex("034241e9"), make([]byte, 233)...), nas.DirectionDownlink},
		{"RP-DATA destination", mustHex("0001000d91214365870921436587092143010a"), nas.DirectionUplink},
		{"RP-ERROR diagnostic", mustHex("050103290507"), nas.DirectionDownlink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseRP(tc.wire, tc.dir)
			if err != nil {
				t.Fatal(err)
			}

			if _, err := m.MarshalBinary(); !errors.Is(err, ErrElementTooLong) {
				t.Fatalf("re-encode: err = %v, want ErrElementTooLong", err)
			}
		})
	}
}

func TestEncoderLimits(t *testing.T) {
	sc := E164Address("447785016005")

	for _, tc := range []struct {
		name string
		msg  interface{ MarshalBinary() ([]byte, error) }
		want error
	}{
		{"CP-User data 249", &CPData{UserData: make([]byte, 249)}, ErrElementTooLong},
		{"RP-DATA user data 233", &RPData{Destination: sc, UserData: make([]byte, 233)}, ErrElementTooLong},
		{"RP-ACK user data 233", &RPAck{UserData: make([]byte, 233)}, ErrElementTooLong},
		{"RP-ERROR user data 233", &RPError{UserData: make([]byte, 233)}, ErrElementTooLong},
		{"RP-ERROR diagnostic 2", &RPError{Diagnostic: []byte{1, 2}}, ErrElementTooLong},
		{"address 21 digits", &RPData{Destination: E164Address("123456789012345678901")}, ErrElementTooLong},
		{"MO without destination", &RPData{Direction: nas.DirectionUplink, Originator: sc}, ErrInvalidMandatoryIE},
		{"MT without originator", &RPData{Direction: nas.DirectionDownlink, Destination: sc}, ErrInvalidMandatoryIE},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.msg.MarshalBinary(); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	for _, tc := range []interface{ MarshalBinary() ([]byte, error) }{
		&CPData{UserData: make([]byte, 248)},
		&RPData{Destination: sc, UserData: make([]byte, 232)},
		&RPAck{UserData: make([]byte, 232)},
		&RPError{Diagnostic: []byte{1}, UserData: make([]byte, 232)},
		&RPData{Destination: E164Address("12345678901234567890")},
	} {
		if _, err := tc.MarshalBinary(); err != nil {
			t.Errorf("%T at its limit: %v", tc, err)
		}
	}
}

func TestRPDataUnusedAddressMalformed(t *testing.T) {
	wire := mustHex("0001019104912143f5010a")

	m, err := ParseRP(wire, nas.DirectionUplink)
	if err == nil || !nas.SoftOnly(err) {
		t.Fatalf("err = %v, want a soft error", err)
	}

	data := m.(*RPData)
	if data.Originator == nil || !bytes.Equal(data.Originator.Raw, []byte{0x91}) || data.Destination.Digits != "12345" {
		t.Fatalf("parsed %+v", data)
	}

	b, err := data.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b, wire) {
		t.Fatalf("re-encode = % x, want % x", b, wire)
	}
}

func TestRPErrorKeepsEveryDiagnosticOctet(t *testing.T) {
	m, err := ParseRP(mustHex("0501032905074101aa"), nas.DirectionDownlink)
	if err != nil {
		t.Fatal(err)
	}

	if e := m.(*RPError); !bytes.Equal(e.Diagnostic, []byte{0x05, 0x07}) || !bytes.Equal(e.UserData, []byte{0xaa}) {
		t.Fatalf("parsed %+v", e)
	}
}

func TestEffectiveCause(t *testing.T) {
	for _, tc := range []struct {
		got, want RPCause
	}{
		{RPCauseMemoryCapacityExceeded.Effective(TransferMobileTerminated), RPCauseMemoryCapacityExceeded},
		{RPCauseMemoryCapacityExceeded.Effective(TransferMobileOriginated), RPCauseTemporaryFailure},
		{RPCause(2).Effective(TransferMobileOriginated), RPCauseTemporaryFailure},
		{RPCause(2).Effective(TransferMobileTerminated), RPCauseProtocolErrorUnspecified},
		{RPCauseCallBarred.Effective(TransferMemoryAvailable), RPCauseTemporaryFailure},
		{RPCauseUnknownSubscriber.Effective(TransferMemoryAvailable), RPCauseUnknownSubscriber},
	} {
		if tc.got != tc.want {
			t.Errorf("Effective = %s, want %s", tc.got, tc.want)
		}
	}

	if got := CPCause(5).Effective(); got != CPCauseProtocolErrorUnspecified {
		t.Errorf("CPCause(5).Effective() = %s", got)
	}

	if got := CPCauseCongestion.Effective(); got != CPCauseCongestion {
		t.Errorf("CPCauseCongestion.Effective() = %s", got)
	}
}

func TestCPTrailingOctetsArePreserved(t *testing.T) {
	wire := mustHex("09010200aa4102bbcc")

	m, err := ParseCP(wire)
	if err != nil {
		t.Fatal(err)
	}

	if data := m.(*CPData); !bytes.Equal(data.UserData, []byte{0x00, 0xaa}) || len(data.Unrecognized) != 1 {
		t.Fatalf("parsed %+v", data)
	}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b, wire) {
		t.Fatalf("re-encode = % x, want % x", b, wire)
	}
}

func TestRPUnknownIEIsTypeFour(t *testing.T) {
	wire := mustHex("020178020102410103")

	m, err := ParseRP(wire, nas.DirectionUplink)
	if err != nil {
		t.Fatal(err)
	}

	ack := m.(*RPAck)
	if !bytes.Equal(ack.UserData, []byte{0x03}) || len(ack.Unrecognized) != 1 || ack.Unrecognized[0].Format != nas.IETLV {
		t.Fatalf("parsed %+v", ack)
	}
}

func TestAddress(t *testing.T) {
	for _, tc := range []struct {
		addr Address
		wire string
	}{
		{Address{TypeOfNumber: TypeOfNumberUnknown, NumberingPlan: NumberingPlanISDN, Digits: "12345"}, "812143f5"},
		{Address{TypeOfNumber: TypeOfNumberInternational, NumberingPlan: NumberingPlanISDN, Digits: "*#abc0"}, "91ba dc 0e"},
		{*E164Address("12345678901234567890"), "912143658709214365870 9"},
	} {
		wire := mustHex(stripSpaces(tc.wire))

		b, err := tc.addr.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(b, wire) {
			t.Fatalf("%s = % x, want % x", tc.addr, b, wire)
		}

		got, err := ParseAddress(wire)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(got, tc.addr) {
			t.Fatalf("ParseAddress(% x) = %+v, want %+v", wire, got, tc.addr)
		}
	}
}

func TestAddressRejects(t *testing.T) {
	for _, wire := range []string{"91", "91f1 21", "91ff"} {
		if _, err := ParseAddress(mustHex(stripSpaces(wire))); err == nil {
			t.Errorf("ParseAddress(%s) succeeded", wire)
		}
	}

	for _, a := range []Address{
		{Digits: ""},
		{Digits: "12x"},
		{TypeOfNumber: 8, Digits: "1"},
	} {
		if _, err := a.MarshalBinary(); err == nil {
			t.Errorf("%+v encoded", a)
		}
	}
}

func TestNames(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{CPMessageTypeData.Name(), "CP-DATA"},
		{CPMessageType(2).Name(), ""},
		{CPCauseInvalidTransactionIdentifier.Name(), "Invalid Transaction Identifier value"},
		{CPCause(1).Name(), ""},
		{MTISMMAMSToNetwork.Name(), "RP-SMMA"},
		{MessageTypeIndicator(7).Name(), ""},
		{RPCauseMemoryCapacityExceeded.Name(), "Memory capacity exceeded"},
		{RPCause(0).Name(), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("Name() = %q, want %q", tc.got, tc.want)
		}
	}
}
