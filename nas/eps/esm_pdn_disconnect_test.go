// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestPDNDisconnectRequestRoundTrip(t *testing.T) {
	in := &PDNDisconnectRequest{
		PTI:                     3,
		LinkedEPSBearerIdentity: 5,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	// Header (EBI=0, PD=ESM, PTI, message type) then the spare+linked-EBI octet.
	if wire[0] != uint8(PDESM) || wire[2] != uint8(MsgPDNDisconnectRequest) {
		t.Fatalf("header = %x", wire[:3])
	}

	if mt, err := PeekESMMessageType(wire); err != nil || mt != MsgPDNDisconnectRequest {
		t.Fatalf("PeekESMMessageType = %#x, err %v", mt, err)
	}

	out, err := ParsePDNDisconnectRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.PTI != 3 || out.LinkedEPSBearerIdentity != 5 {
		t.Fatalf("out = %+v", out)
	}
}

func TestPDNDisconnectRequestSpareHalfOctetIgnored(t *testing.T) {
	// A sender that fills the spare high half-octet must not corrupt the linked
	// EPS bearer identity (TS 24.301).
	wire := []byte{uint8(PDESM), 0x02, uint8(MsgPDNDisconnectRequest), 0xF5}

	out, err := ParsePDNDisconnectRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.LinkedEPSBearerIdentity != 5 {
		t.Fatalf("LinkedEPSBearerIdentity = %d, want 5", out.LinkedEPSBearerIdentity)
	}
}

func TestPDNDisconnectRejectRoundTrip(t *testing.T) {
	in := &PDNDisconnectReject{
		EPSBearerIdentity: 5,
		PTI:               3,
		Cause:             49, // last PDN disconnection not allowed
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePDNDisconnectReject(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSBearerIdentity != 5 || out.PTI != 3 || out.Cause != 49 {
		t.Fatalf("out = %+v", out)
	}
}

func TestModifyEPSBearerContextRejectRoundTrip(t *testing.T) {
	in := &ModifyEPSBearerContextReject{
		EPSBearerIdentity: 5,
		PTI:               1,
		Cause:             43, // invalid EPS bearer identity
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseModifyEPSBearerContextReject(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSBearerIdentity != 5 || out.PTI != 1 || out.Cause != 43 {
		t.Fatalf("out = %+v", out)
	}
}

func TestPDNConnectivityRequestWithAPNAndPCO(t *testing.T) {
	pco := nas.NewProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)
	pco.Direction = nas.PCOMSToNetwork
	pcoPtr := &pco

	in := &PDNConnectivityRequest{
		PTI:                          2,
		RequestType:                  1, // initial request
		PDNType:                      1, // IPv4
		AccessPointName:              ptr(APN("internet")),
		ProtocolConfigurationOptions: pcoPtr,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePDNConnectivityRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.RequestType != 1 || out.PDNType != 1 {
		t.Fatalf("request/pdn type = %d/%d", out.RequestType, out.PDNType)
	}

	if out.AccessPointName == nil || *out.AccessPointName != "internet" {
		t.Fatalf("APN = %q", *out.AccessPointName)
	}

	if !reflect.DeepEqual(out.ProtocolConfigurationOptions, pcoPtr) {
		t.Fatalf("PCO = %x, want %x", out.ProtocolConfigurationOptions, pco)
	}
}

// TestPDNConnectivityRequestNoOptionalIEs confirms the default-bearer form the UE
// sends inside the Attach Request (no optional IEs) still round-trips, leaving
// the APN and PCO absent.
func TestPDNConnectivityRequestNoOptionalIEs(t *testing.T) {
	in := &PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePDNConnectivityRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	if out.AccessPointName != nil || out.ProtocolConfigurationOptions != nil {
		t.Fatalf("expected no optional IEs, got %+v", out)
	}
}
