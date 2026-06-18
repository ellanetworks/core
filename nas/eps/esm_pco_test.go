// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
)

func TestBuildProtocolConfigurationOptions(t *testing.T) {
	v4 := []byte{8, 8, 8, 8}
	v6 := []byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	pco := BuildProtocolConfigurationOptions([][]byte{v4, v6}, 1400)

	// Config-protocol octet, the IPv4 (0x000D) and IPv6 (0x0003) DNS containers,
	// then the IPv4 Link MTU (0x0010, 2 octets; 1400 = 0x0578).
	want := []byte{0x80, 0x00, 0x0D, 0x04, 8, 8, 8, 8, 0x00, 0x03, 0x10}
	want = append(want, v6...)
	want = append(want, 0x00, 0x10, 0x02, 0x05, 0x78)

	if !bytes.Equal(pco, want) {
		t.Fatalf("PCO = %x, want %x", pco, want)
	}

	// MTU only (IPv6-only bearer would carry no IPv4 Link MTU, but the encoder
	// itself emits just the MTU container when given no DNS).
	mtuOnly := BuildProtocolConfigurationOptions(nil, 1500)
	if !bytes.Equal(mtuOnly, []byte{0x80, 0x00, 0x10, 0x02, 0x05, 0xDC}) {
		t.Fatalf("MTU-only PCO = %x", mtuOnly)
	}

	if BuildProtocolConfigurationOptions(nil, 0) != nil {
		t.Fatal("no DNS and no MTU should produce no PCO")
	}
}

func TestParseProtocolConfigurationOptions(t *testing.T) {
	v4 := []byte{1, 1, 1, 1}
	v6 := []byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	pco := BuildProtocolConfigurationOptions([][]byte{v4, v6}, 1400)

	dns, mtu, err := ParseProtocolConfigurationOptions(pco)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(dns) != 2 || !bytes.Equal(dns[0], v4) || !bytes.Equal(dns[1], v6) {
		t.Fatalf("DNS servers = %x, want [%x %x]", dns, v4, v6)
	}

	if mtu != 1400 {
		t.Fatalf("MTU = %d, want 1400", mtu)
	}
}

func TestActivateDefaultPCOAndESMCauseRoundTrip(t *testing.T) {
	cause := ESMCausePDNTypeIPv4OnlyAllowed
	pco := BuildProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)

	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 1,
		EPSQoS:                       []byte{9},
		AccessPointName:              []byte("internet"),
		PDNAddress:                   []byte{0x01, 10, 45, 0, 1},
		ESMCause:                     &cause,
		ProtocolConfigurationOptions: pco,
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.ESMCause == nil || *out.ESMCause != ESMCausePDNTypeIPv4OnlyAllowed {
		t.Fatalf("ESM cause = %v, want %d", out.ESMCause, ESMCausePDNTypeIPv4OnlyAllowed)
	}

	if !bytes.Equal(out.ProtocolConfigurationOptions, pco) {
		t.Fatalf("PCO = %x, want %x", out.ProtocolConfigurationOptions, pco)
	}
}

// TestActivateDefaultUnknownOptionalIE checks that an optional IE the message
// does not declare ends the optional-IE walk without failing the parse: the ESM
// cause that precedes it is still decoded and the unknown IE is ignored.
func TestActivateDefaultUnknownOptionalIE(t *testing.T) {
	cause := ESMCausePDNTypeIPv6OnlyAllowed

	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 1,
		EPSQoS:                       []byte{9},
		AccessPointName:              []byte("internet"),
		PDNAddress:                   []byte{0x02, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		ESMCause:                     &cause,
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire = append(wire, 0x5e, 0x02, 0x00, 0x00) // APN-AMBR-shaped IE the message does not declare

	out, err := ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.ESMCause == nil || *out.ESMCause != cause {
		t.Fatalf("ESM cause = %v, want %d", out.ESMCause, cause)
	}
}

func TestActivateDefaultNoOptionalIEsRoundTrip(t *testing.T) {
	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 1,
		EPSQoS:                       []byte{9},
		AccessPointName:              []byte("internet"),
		PDNAddress:                   []byte{0x01, 10, 45, 0, 1},
	}

	wire, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.ESMCause != nil || out.ProtocolConfigurationOptions != nil {
		t.Fatalf("unexpected optional IEs: cause=%v pco=%x", out.ESMCause, out.ProtocolConfigurationOptions)
	}
}
