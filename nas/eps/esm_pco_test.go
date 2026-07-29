// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestActivateDefaultPCOAndESMCauseRoundTrip(t *testing.T) {
	cause := ESMCausePDNTypeIPv4OnlyAllowed
	pco := nas.NewProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)
	pcoPtr := &pco

	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		PTI:                          1,
		EPSQoS:                       EPSQoS{QCI: 9},
		AccessPointName:              APN("internet"),
		PDNAddress:                   PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}},
		Cause:                        &cause,
		ProtocolConfigurationOptions: pcoPtr,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause == nil || *out.Cause != ESMCausePDNTypeIPv4OnlyAllowed {
		t.Fatalf("ESM cause = %v, want %d", out.Cause, ESMCausePDNTypeIPv4OnlyAllowed)
	}

	if !reflect.DeepEqual(out.ProtocolConfigurationOptions, pcoPtr) {
		t.Fatalf("PCO = %x, want %x", out.ProtocolConfigurationOptions, pco)
	}
}

// TestActivateDefaultUnknownOptionalIE checks that an optional IE the message
// does not declare ends the optional-IE walk without failing the parse: the ESM
// cause that precedes it is still decoded and the unknown IE is ignored.
func TestActivateDefaultUnknownOptionalIE(t *testing.T) {
	cause := ESMCausePDNTypeIPv6OnlyAllowed

	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		PTI:               1,
		EPSQoS:            EPSQoS{QCI: 9},
		AccessPointName:   APN("internet"),
		PDNAddress:        PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}},
		Cause:             &cause,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire = append(wire, 0x5e, 0x02, 0x00, 0x00) // APN-AMBR-shaped IE the message does not declare

	out, err := ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause == nil || *out.Cause != cause {
		t.Fatalf("ESM cause = %v, want %d", out.Cause, cause)
	}
}

func TestActivateDefaultNoOptionalIEsRoundTrip(t *testing.T) {
	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		PTI:               1,
		EPSQoS:            EPSQoS{QCI: 9},
		AccessPointName:   APN("internet"),
		PDNAddress:        PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}},
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause != nil || out.ProtocolConfigurationOptions != nil {
		t.Fatalf("unexpected optional IEs: cause=%v pco=%x", out.Cause, out.ProtocolConfigurationOptions)
	}
}
