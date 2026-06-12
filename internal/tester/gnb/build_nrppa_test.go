// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"testing"

	"github.com/ellanetworks/core/internal/nrppa"
	"github.com/free5gc/ngap/ngapType"
)

// TestBuildNRPPaECIDMeasurementResponse_RoundTrip verifies the gNB tester builds
// a valid NGAP UplinkUEAssociatedNRPPaTransport whose embedded NRPPa octet
// string decodes back to an E-CIDMeasurementInitiationResponse with the serving
// cell, access point position and timing advance the tester supplied.
func TestBuildNRPPaECIDMeasurementResponse_RoundTrip(t *testing.T) {
	const (
		amfUeNgapID = int64(1)
		ranUeNgapID = int64(2)
		lmfMeasID   = int64(5)
		ranMeasID   = int64(1)
	)

	opts := &NRPPaECIDResponseOpts{
		AMFUeNgapID:        amfUeNgapID,
		RANUeNgapID:        ranUeNgapID,
		LMFUEMeasurementID: lmfMeasID,
		RANUEMeasurementID: ranMeasID,
		TimingAdvance:      sampleTimingAdvance,
	}

	pdu, err := BuildNRPPaECIDMeasurementResponse(opts)
	if err != nil {
		t.Fatalf("BuildNRPPaECIDMeasurementResponse: %v", err)
	}

	// Extract the embedded NRPPa octet string from the NGAP transport.
	if pdu.Present != ngapType.NGAPPDUPresentInitiatingMessage {
		t.Fatalf("expected NGAP InitiatingMessage, got present=%d", pdu.Present)
	}

	transport := pdu.InitiatingMessage.Value.UplinkUEAssociatedNRPPaTransport
	if transport == nil {
		t.Fatal("UplinkUEAssociatedNRPPaTransport is nil")
	}

	var nrppaPdu []byte

	for _, ie := range transport.ProtocolIEs.List {
		if ie.Id.Value == ngapType.ProtocolIEIDNRPPaPDU && ie.Value.NRPPaPDU != nil {
			nrppaPdu = ie.Value.NRPPaPDU.Value
		}
	}

	if nrppaPdu == nil {
		t.Fatal("NRPPa PDU octet string missing from transport")
	}

	parsed, err := nrppa.ParsePDU(nrppaPdu)
	if err != nil {
		t.Fatalf("nrppa.ParsePDU: %v", err)
	}

	if parsed.Kind != nrppa.KindECIDMeasurementInitiationResponse || parsed.Response == nil {
		t.Fatalf("expected E-CIDMeasurementInitiationResponse, got kind=%d", parsed.Kind)
	}

	resp := parsed.Response
	if resp.LMFUEMeasurementID != lmfMeasID {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want %d", resp.LMFUEMeasurementID, lmfMeasID)
	}

	if resp.RANUEMeasurementID != ranMeasID {
		t.Errorf("RAN-UE-Measurement-ID: got %d, want %d", resp.RANUEMeasurementID, ranMeasID)
	}

	if resp.Result == nil {
		t.Fatal("result is nil")
	}

	if resp.Result.ServingCell.NRCellIdentity == nil || *resp.Result.ServingCell.NRCellIdentity != sampleNRCellIdentity {
		t.Errorf("NR cell identity: got %v, want %#x", resp.Result.ServingCell.NRCellIdentity, sampleNRCellIdentity)
	}

	if resp.Result.TimingAdvanceType1 == nil || *resp.Result.TimingAdvanceType1 != sampleTimingAdvance {
		t.Errorf("timing advance: got %v, want %d", resp.Result.TimingAdvanceType1, sampleTimingAdvance)
	}

	if resp.Result.APPosition == nil {
		t.Fatal("access point position is nil")
	}

	if lat := resp.Result.APPosition.LatitudeDegrees; lat < 44.99 || lat > 45.01 {
		t.Errorf("latitude degrees: got %f, want ~45", lat)
	}
}
