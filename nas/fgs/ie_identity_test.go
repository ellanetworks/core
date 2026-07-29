// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestSUCIIMSIFormat(t *testing.T) {
	// SUPI=IMSI, MCC=001 MNC=01, routing 0000, null scheme, HNPKI 0, MSIN 0000000001.
	buf, _ := hex.DecodeString("0100f110000000000000000010")

	s, err := ParseSUCI(buf)
	if err != nil {
		t.Fatalf("ParseSUCI: %v", err)
	}

	if s.PLMN.MCC != "001" || s.PLMN.MNC != "01" || s.RoutingIndicator != "0000" ||
		s.ProtectionScheme != ProtectionSchemeNull || s.HomeNetworkPKID != 0 {
		t.Fatalf("SUCI = %+v", s)
	}

	if msin, ok := s.MSIN(); !ok || msin != "0000000001" {
		t.Errorf("MSIN = %q (revealed %v)", msin, ok)
	}

	if supi, ok := s.SUPI(); !ok || supi != "imsi-001010000000001" {
		t.Errorf("SUPI = %q (recovered %v)", supi, ok)
	}

	if got := s.String(); got != "suci-0-001-01-0000-0-0-0000000001" {
		t.Errorf("String = %q", got)
	}
}

func TestSUCINAIFormat(t *testing.T) {
	// The SUCI NAI field is a UTF-8 string (TS 24.501 table 9.11.3.4.1).
	nai := "user@example.com"
	buf := append([]byte{0x11}, nai...)

	s, err := ParseSUCI(buf)
	if err != nil {
		t.Fatalf("ParseSUCI: %v", err)
	}

	if s.Format != SUPIFormatNetworkSpecific || !reflect.DeepEqual(s.NAI, []byte(nai)) {
		t.Fatalf("SUCI = %+v", s)
	}

	if _, ok := s.SUPI(); ok {
		t.Error("a NAI-format SUCI must not yield a SUPI")
	}

	if got := s.String(); got != "nai-1-"+nai {
		t.Errorf("String = %q", got)
	}
}

// TestSUCIGCIAndGLIUseTheNAILayout confirms SUPI formats 2 and 3 take the NAI
// layout of TS 24.501 figure 9.11.3.4.4, whose caption names all three formats;
// decoding them as the IMSI layout invents a PLMN out of NAI octets.
func TestSUCIGCIAndGLIUseTheNAILayout(t *testing.T) {
	for _, f := range []SUPIFormat{SUPIFormatGCI, SUPIFormatGLI} {
		nai := "line@operator.example"

		s, err := ParseSUCI(append([]byte{uint8(f)<<4 | uint8(IdentitySUCI)}, nai...))
		if err != nil {
			t.Fatalf("ParseSUCI(format %d): %v", f, err)
		}

		if s.Format != f || string(s.NAI) != nai {
			t.Fatalf("format %d decoded as %+v", f, s)
		}

		if s.PLMN.MCC != "" || s.PLMN.MNC != "" {
			t.Errorf("format %d invented a PLMN %s-%s", f, s.PLMN.MCC, s.PLMN.MNC)
		}
	}
}

// TestSUCIAbsentRoutingIndicator pins TS 24.501 table 9.11.3.4.1: a UE with no
// routing indicator configured codes the first digit as 0 and the rest as
// fillers, so the two octets read F0 FF rather than FF FF.
func TestSUCIAbsentRoutingIndicator(t *testing.T) {
	raw, err := SUCI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if got := raw[4:6]; got[0] != 0xF0 || got[1] != 0xFF {
		t.Fatalf("routing indicator octets = % x, want f0 ff", got)
	}
}

// TestSUCIConcealedKeepsSchemeOutput checks a SUCI under a real protection
// scheme neither reveals the subscriber nor loses the ciphertext.
func TestSUCIConcealedKeepsSchemeOutput(t *testing.T) {
	buf, _ := hex.DecodeString("0100f110ffff0201aabbccdd")

	s, err := ParseSUCI(buf)
	if err != nil {
		t.Fatalf("ParseSUCI: %v", err)
	}

	if s.ProtectionScheme != 2 || s.HomeNetworkPKID != 1 {
		t.Fatalf("SUCI = %+v", s)
	}

	if _, ok := s.MSIN(); ok {
		t.Error("a concealed SUCI must not yield an MSIN")
	}

	if !reflect.DeepEqual(s.SchemeOutput, []byte{0xaa, 0xbb, 0xcc, 0xdd}) {
		t.Errorf("SchemeOutput = %x", s.SchemeOutput)
	}
}

func TestSUCITooShort(t *testing.T) {
	if _, err := ParseSUCI([]byte{0x01, 0x00}); err == nil {
		t.Error("expected an error for a too-short SUCI")
	}
}

func TestPEIIMEI(t *testing.T) {
	buf, _ := hex.DecodeString("4b09512430325781") // IMEI 490154203237518 (Luhn-valid)

	p, err := ParsePEI(buf)
	if err != nil {
		t.Fatalf("ParsePEI: %v", err)
	}

	if p.Type != IdentityIMEI || p.Digits != "490154203237518" || !p.Valid() {
		t.Errorf("PEI = %+v (valid %v)", p, p.Valid())
	}

	if got := p.String(); got != "imei-490154203237518" {
		t.Errorf("String = %q", got)
	}
}

// TestPEIInvalidChecksumStillDecodes checks a bad check digit is reported by
// Valid rather than by rejecting the message, so the identity still round-trips.
func TestPEIInvalidChecksumStillDecodes(t *testing.T) {
	// The valid IMEI with its last digit changed from 8 to 9.
	buf, _ := hex.DecodeString("4b09512430325791")

	p, err := ParsePEI(buf)
	if err != nil {
		t.Fatalf("ParsePEI: %v", err)
	}

	if p.Digits != "490154203237519" {
		t.Fatalf("PEI = %+v", p)
	}

	if p.Valid() {
		t.Error("Valid must reject an IMEI whose check digit is wrong")
	}
}

func TestPEIIMEISV(t *testing.T) {
	buf, _ := hex.DecodeString("1532547698103254f6")

	p, err := ParsePEI(buf)
	if err != nil {
		t.Fatalf("ParsePEI: %v", err)
	}

	if p.Type != IdentityIMEISV || p.Digits != "1234567890123456" || !p.Valid() {
		t.Errorf("PEI = %+v (valid %v)", p, p.Valid())
	}
}

func TestMobileIdentityRoundTrip(t *testing.T) {
	cases := map[string]MobileIdentity{
		"SUCI, IMSI format": SUCIIdentity(SUCI{
			PLMN: nas.PLMN{MCC: "001", MNC: "01"}, RoutingIndicator: "12",
			ProtectionScheme: ProtectionSchemeNull, SchemeOutput: []byte{0x00, 0x00, 0x00, 0x00, 0x10},
		}),
		"SUCI, NAI format": SUCIIdentity(SUCI{Format: SUPIFormatNetworkSpecific, NAI: []byte("user@example.com")}),
		"5G-GUTI": GUTIIdentity(GUTI{
			PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 205, AMFSetID: 1018, AMFPointer: 1,
			TMSI: [4]byte{0x21, 0x43, 0x65, 0x84},
		}),
		"5G-S-TMSI":   STMSIIdentity(STMSI{AMFSetID: 1018, AMFPointer: 1, TMSI: [4]byte{1, 2, 3, 4}}),
		"IMEI":        PEIIdentity(PEI{Type: IdentityIMEI, Digits: "490154203237518"}),
		"IMEISV":      PEIIdentity(PEI{Type: IdentityIMEISV, Digits: "1234567890123456"}),
		"no identity": NoIdentity(),
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			b, err := in.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}

			got, err := ParseMobileIdentity(b)
			if err != nil {
				t.Fatalf("ParseMobileIdentity(% x): %v", b, err)
			}

			if !reflect.DeepEqual(got, in) {
				t.Fatalf("round-trip = %+v, want %+v (wire % x)", got, in, b)
			}
		})
	}
}

func TestTypeOfIdentity(t *testing.T) {
	cases := map[byte]MobileIdentityType{
		0x00: IdentityNoIdentity,
		0x01: IdentitySUCI,
		0xf2: IdentityGUTI,
		0x0b: IdentityIMEI,
		0x04: IdentitySTMSI,
		0x05: IdentityIMEISV,
		0x06: IdentityMACAddress,
		0x0f: IdentityEUI64,
	}

	for octet, want := range cases {
		if got := TypeOfIdentity(octet); got != want {
			t.Errorf("TypeOfIdentity(%#x) = %d, want %d", octet, got, want)
		}
	}
}
