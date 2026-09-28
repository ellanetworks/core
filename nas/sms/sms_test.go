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
		{"RP-ERROR no cause", "0401", nas.DirectionUplink, nas.ErrTruncated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseRP(mustHex(tc.wire), tc.dir); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	if _, err := ParseRP(mustHex("04010000"), nas.DirectionUplink); err == nil {
		t.Error("empty RP-Cause parsed")
	}
}

func TestOverlongElementsAreNotSyntaxErrors(t *testing.T) {
	cp := append(mustHex("0901f9"), make([]byte, 249)...)
	if m, err := ParseCP(cp); err != nil || len(m.(*CPData).UserData) != 249 {
		t.Fatalf("249-octet CP-User data: %v", err)
	}

	for _, tc := range []struct {
		name string
		wire []byte
		dir  nas.Direction
	}{
		{"RP-DATA user data", append(mustHex("00010003912143e9"), make([]byte, 233)...), nas.DirectionUplink},
		{"RP-ACK user data", append(mustHex("034241e9"), make([]byte, 233)...), nas.DirectionDownlink},
		{"RP-DATA destination", mustHex("0001000d91214365870921436587092143010a"), nas.DirectionUplink},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := ParseRP(tc.wire, tc.dir)
			if err != nil {
				t.Fatal(err)
			}

			b, err := m.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(b, tc.wire) {
				t.Fatalf("re-encode = % x, want % x", b, tc.wire)
			}
		})
	}
}

func TestRPDataUnusedAddressMalformed(t *testing.T) {
	wire := mustHex("0001019104912143f5010a")

	m, err := ParseRP(wire, nas.DirectionUplink)
	if err == nil || !nas.SoftOnly(err) {
		t.Fatalf("err = %v, want a soft error", err)
	}

	data := m.(*RPData)
	if data.Originator != nil || data.Destination.Digits != "12345" {
		t.Fatalf("parsed %+v", data)
	}

	if _, err := ParseRP(mustHex("0001000191010a"), nas.DirectionUplink); err == nil || nas.SoftOnly(err) {
		t.Fatalf("malformed destination on MO: err = %v, want a hard error", err)
	}
}

func TestRPErrorKeepsEveryDiagnosticOctet(t *testing.T) {
	wire := mustHex("0501032905074101aa")

	m, err := ParseRP(wire, nas.DirectionDownlink)
	if err != nil {
		t.Fatal(err)
	}

	if e := m.(*RPError); !bytes.Equal(e.Diagnostic, []byte{0x05, 0x07}) || !bytes.Equal(e.UserData, []byte{0xaa}) {
		t.Fatalf("parsed %+v", e)
	}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(b, wire) {
		t.Fatalf("re-encode = % x, want % x", b, wire)
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

		if got != tc.addr {
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
