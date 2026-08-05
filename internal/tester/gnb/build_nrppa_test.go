// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"testing"

	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/nrppa"
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
		RoutingID:          ngap.RoutingID{0x00},
	}

	pdu, err := BuildNRPPaECIDMeasurementResponse(opts)
	if err != nil {
		t.Fatalf("BuildNRPPaECIDMeasurementResponse: %v", err)
	}

	// Extract the embedded NRPPa octet string from the NGAP transport.
	msg, err := ngap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("ngap.Unmarshal: %v", err)
	}

	im, ok := msg.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcUplinkUEAssociatedNRPPaTransport {
		t.Fatalf("expected an Uplink UE-associated NRPPa Transport, got %T", msg)
	}

	transport, err := ngap.ParseUplinkUEAssociatedNRPPaTransport(im.Value)
	if err != nil {
		t.Fatalf("ParseUplinkUEAssociatedNRPPaTransport: %v", err)
	}

	if int64(transport.AMFUENGAPID) != amfUeNgapID || int64(transport.RANUENGAPID) != ranUeNgapID {
		t.Errorf("transport AP IDs = (%d, %d), want (%d, %d)",
			transport.AMFUENGAPID, transport.RANUENGAPID, amfUeNgapID, ranUeNgapID)
	}

	nrppaPdu := []byte(transport.NRPPaPDU)
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
