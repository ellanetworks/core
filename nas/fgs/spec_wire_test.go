// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestSTMSIFillerNibble pins the filler TS 24.501 table 9.11.3.4.1 mandates:
// "For the 5G-S-TMSI, bits 5 to 8 of octet 4 are coded as '1111'". Encoding the
// type alone went undetected because ParseSTMSI did not check the type field.
func TestSTMSIFillerNibble(t *testing.T) {
	raw, err := STMSI{AMFSetID: 0x01, AMFPointer: 0x02, TMSI: [4]byte{1, 2, 3, 4}}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0xf4, 0x00, 0x42, 0x01, 0x02, 0x03, 0x04}
	if !bytes.Equal(raw, want) {
		t.Fatalf("5G-S-TMSI = % x, want % x", raw, want)
	}

	if _, err := ParseSTMSI([]byte{0xf2, 0x00, 0x42, 1, 2, 3, 4}); err == nil {
		t.Fatal("ParseSTMSI accepted a value whose type of identity is 5G-GUTI")
	}

	// A sender that omitted the filler still decodes; the element canonicalizes
	// on re-encode rather than being rejected.
	got, err := ParseSTMSI([]byte{0x04, 0x00, 0x42, 1, 2, 3, 4})
	if err != nil {
		t.Fatalf("ParseSTMSI rejected a value with an unset filler nibble: %v", err)
	}

	if again, err := got.MarshalBinary(); err != nil || !bytes.Equal(again, want) {
		t.Fatalf("re-encode = % x (err %v), want % x", again, err, want)
	}
}

// TestCapability5GMMN3DataIsInverted pins the one bit of the 5GMM capability
// whose polarity is inverted on the wire: TS 24.501 table 9.11.3.1.1 codes
// octet 3 bit 6 as 0 "N3 data transfer supported", 1 "not supported".
func TestCapability5GMMN3DataIsInverted(t *testing.T) {
	supported, err := GMMCapability{N3Data: true}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if supported[0]&(1<<5) != 0 {
		t.Fatalf("N3 data supported encoded as %#02x, want bit 6 clear", supported[0])
	}

	back, err := ParseGMMCapability(supported)
	if err != nil {
		t.Fatal(err)
	}

	if !back.N3Data {
		t.Fatal("N3Data did not survive a round trip")
	}

	if c, err := ParseGMMCapability([]byte{1 << 5}); err != nil || c.N3Data {
		t.Fatalf("bit 6 set = %+v (err %v), want N3Data false", c, err)
	}
}

// TestPEIAcceptsTransmittedSpareDigit pins TS 23.003 §6.2.1: the 15th IMEI digit
// is the Spare Digit over the air, "set to zero, when transmitted by the MS",
// and the Luhn check of Annex B does not apply to it. Requiring a check digit
// rejected roughly nine handsets in ten.
func TestPEIAcceptsTransmittedSpareDigit(t *testing.T) {
	transmitted := PEI{Type: IdentityIMEI, Digits: "123456789012340"}
	if !transmitted.Valid() {
		t.Error("an IMEI with the transmitted spare digit was rejected")
	}

	// An IMEI written with its real check digit still validates.
	if !(PEI{Type: IdentityIMEI, Digits: "490154203237518"}).Valid() {
		t.Error("an IMEI carrying its Luhn check digit was rejected")
	}

	// A non-zero last digit that is not the correct check digit stays invalid.
	if (PEI{Type: IdentityIMEI, Digits: "490154203237519"}).Valid() {
		t.Error("an IMEI with a wrong check digit was accepted")
	}
}

// TestQoSBitRateUnitLadder pins the 26 unit codes of TS 24.501 §9.11.4.12: 1, 4,
// 16, 64, 256 within each decade, then on to the next.
func TestQoSBitRateUnitLadder(t *testing.T) {
	tests := []struct {
		unit uint8
		kbps uint64
	}{
		{0x00, 1},             // unused; NOTE 2 reads it as 1 Kbps
		{0x01, 1},             // 1 Kbps
		{0x05, 256},           // 256 Kbps
		{0x06, 1_000},         // 1 Mbps
		{0x0B, 1_000_000},     // 1 Gbps
		{0x10, 1_000_000_000}, // 1 Tbps
		{0x19, 256_000_000_000_000},
		{0xFF, 256_000_000_000_000}, // above the table, same as 256 Pbps
	}

	for _, tc := range tests {
		got, ok := QoSFlowParameter{ID: QoSFlowParamGFBRUplink, Value: []byte{tc.unit, 0x00, 0x01}}.Kbps()
		if !ok || got != tc.kbps {
			t.Errorf("unit %#02x = %d kbps (ok %v), want %d", tc.unit, got, ok, tc.kbps)
		}
	}

	if _, ok := (QoSFlowParameter{ID: QoSFlowParamGFBRUplink, Value: []byte{0x01, 0x00}}).Kbps(); ok {
		t.Error("a two-octet value decoded as a bit rate")
	}

	if _, ok := (QoSFlowParameter{ID: QoSFlowParam5QI, Value: []byte{0x01, 0x00, 0x01}}).Kbps(); ok {
		t.Error("a 5QI parameter decoded as a bit rate")
	}
}

// TestSessionAMBRRoundTrip covers the 5GS counterpart of the EPS APN-AMBR
// conversion: the unit ladder of TS 24.501 table 9.11.4.14.1.
func TestSessionAMBRRoundTrip(t *testing.T) {
	for _, kbps := range []uint64{1, 1000, 65535, 100_000, 1_000_000, 4_000_000} {
		a, err := SessionAMBRFromKbps(kbps, kbps)
		if err != nil {
			t.Fatalf("%d kbps: %v", kbps, err)
		}

		dl, ul, ok := a.Kbps()
		if !ok || dl != kbps || ul != kbps {
			t.Errorf("%d kbps round-tripped to %d/%d (ok %v) via %+v", kbps, dl, ul, ok, a)
		}
	}

	// "Value is not used" carries no rate.
	if _, _, ok := (SessionAMBR{DownlinkUnit: SessionAMBRUnitNotUsed}).Kbps(); ok {
		t.Error("the not-used unit reported a rate")
	}
}

// TestIEErrorNamesTheElement confirms a soft failure reports the element by the
// name its message gives it, not by a bare IEI, and that IEErrors surfaces every
// failure rather than only the first.
func TestIEErrorNamesTheElement(t *testing.T) {
	// A REGISTRATION ACCEPT whose T3512 value and T3502 value are both malformed;
	// neither is security-critical, so both are soft.
	b := []byte{
		uint8(EPD5GMM), 0x00, uint8(MsgRegistrationAccept),
		0x01, 0x01, // 5GS registration result (LV)
		ieiT3512Value, 0x02, 0x0a, 0x0b, // GPRS timer 3 is one octet
		ieiT3502Value, 0x02, 0x0a, 0x0b, // GPRS timer 2 is one octet
	}

	msg, err := ParseRegistrationAccept(b)
	if msg == nil || !nas.SoftOnly(err) {
		t.Fatalf("want a usable message and soft errors, got %v", err)
	}

	found := nas.IEErrors(err)
	if len(found) != 2 {
		t.Fatalf("IEErrors returned %d failures, want 2: %v", len(found), err)
	}

	for _, e := range found {
		if e.Name == "" {
			t.Errorf("IE %#02x reported with no name: %v", e.IEI, e)
		}

		if !strings.Contains(e.Error(), e.Name) {
			t.Errorf("error %q omits the element name %q", e, e.Name)
		}
	}
}

// TestQoSRuleOperationWireValues pins the operation codes to TS 24.501
// table 9.11.4.13.1, which assigns 011 "modify and add packet filters" and 100
// "modify and replace all packet filters". The two were swapped, so a caller
// asking to replace a rule's filters emitted the code a UE executes as "add",
// duplicating them. Byte round-trips cannot catch this: both values encode and
// decode cleanly, only the meaning differs.
func TestQoSRuleOperationWireValues(t *testing.T) {
	for _, tc := range []struct {
		op   QoSRuleOperation
		want uint8
	}{
		{QoSRuleOpCreate, 0b001},
		{QoSRuleOpDelete, 0b010},
		{QoSRuleOpModifyAddFilters, 0b011},
		{QoSRuleOpModifyReplaceFilters, 0b100},
		{QoSRuleOpModifyDeleteFilters, 0b101},
		{QoSRuleOpModifyWithoutFilters, 0b110},
	} {
		if uint8(tc.op) != tc.want {
			t.Errorf("%s = %03b, want %03b", tc.op, uint8(tc.op), tc.want)
		}
	}
}

// TestIMEISVRequestUnassignedValues pins TS 24.008 §10.5.5.10, which TS 24.501
// §9.11.3.28 adopts: only 1 asks for the identity and "all other values are
// interpreted as IMEISV not requested". They were rejected as malformed, which
// made a spec-valid SECURITY MODE COMMAND lose the element.
func TestIMEISVRequestUnassignedValues(t *testing.T) {
	for v := range uint8(8) {
		got, err := parseIMEISVRequest([]byte{v})
		if err != nil {
			t.Fatalf("value %d: %v", v, err)
		}

		if want := v == 1; got.Requested() != want {
			t.Errorf("value %d requested = %v, want %v", v, got.Requested(), want)
		}

		// The value re-encodes as it arrived, so an unassigned one is not
		// rewritten to zero.
		if uint8(got) != v {
			t.Errorf("value %d decoded to %d", v, uint8(got))
		}
	}
}
