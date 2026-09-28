// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"encoding/hex"
	"math"
	"testing"

	"github.com/ellanetworks/core/internal/lmf/lpp/lpptype"
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

func TestEncodeDecodeRequestCapabilities(t *testing.T) {
	encoded, err := EncodeRequestCapabilities(0x01, 0x00)
	if err != nil {
		t.Fatalf("EncodeRequestCapabilities: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.TransactionID != 0x01 {
		t.Errorf("expected transaction ID 0x01, got 0x%02x", decoded.TransactionID)
	}

	if decoded.Initiator != lpptype.InitiatorLocationServer {
		t.Errorf("expected initiator locationServer, got %d", decoded.Initiator)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentRequestCapabilities {
		t.Errorf("expected body kind RequestCapabilities, got %d", decoded.BodyKind)
	}
}

func TestEncodeDecodeRequestLocationInformation(t *testing.T) {
	encoded, err := EncodeRequestLocationInformation(0x02, 0x00, 25)
	if err != nil {
		t.Fatalf("EncodeRequestLocationInformation: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.TransactionID != 0x02 {
		t.Errorf("expected transaction ID 0x02, got 0x%02x", decoded.TransactionID)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentRequestLocationInformation {
		t.Errorf("expected body kind RequestLocationInformation, got %d", decoded.BodyKind)
	}
}

func TestEncodeDecodeProvideCapabilities(t *testing.T) {
	encoded, err := EncodeProvideCapabilities(0x03, []int64{lpptype.GNSSIDGPS, lpptype.GNSSIDGLONASS})
	if err != nil {
		t.Fatalf("EncodeProvideCapabilities: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.TransactionID != 0x03 {
		t.Errorf("expected transaction ID 0x03, got 0x%02x", decoded.TransactionID)
	}

	if decoded.Initiator != lpptype.InitiatorTargetDevice {
		t.Errorf("expected initiator targetDevice, got %d", decoded.Initiator)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideCapabilities {
		t.Errorf("expected body kind ProvideCapabilities, got %d", decoded.BodyKind)
	}

	if decoded.ProvideCapabilities == nil {
		t.Fatal("expected ProvideCapabilities to be non-nil")
	}

	if !decoded.ProvideCapabilities.GNSSCapability.Supports(models.GnssIDGps) {
		t.Error("expected GPS capability to be true")
	}

	if !decoded.ProvideCapabilities.GNSSCapability.Supports(models.GnssIDGlonass) {
		t.Error("expected GLO capability to be true")
	}
}

func TestDecodeMediaTekProvideCapabilities(t *testing.T) {
	raw, err := hex.DecodeString("f0010142087800174027a68050300bf80ea15020701000100720641ec0501f0080")
	if err != nil {
		t.Fatalf("hex: %v", err)
	}

	decoded, err := DecodeLPPMessage(raw)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideCapabilities {
		t.Fatalf("expected body kind ProvideCapabilities, got %d", decoded.BodyKind)
	}

	if !decoded.AckRequested {
		t.Error("expected an acknowledgement request")
	}

	if decoded.SequenceNumber == nil || *decoded.SequenceNumber != 1 {
		t.Errorf("expected sequence number 1, got %v", decoded.SequenceNumber)
	}

	supported := decoded.ProvideCapabilities.GNSSCapability.Supported()
	if len(supported) != 1 || supported[0] != models.GnssIDGps {
		t.Errorf("expected GPS only, got %v", supported)
	}
}

func TestEncodeDecodeProvideCapabilitiesExtensionGNSS(t *testing.T) {
	encoded, err := EncodeProvideCapabilities(0x04, []int64{lpptype.GNSSIDBDS, lpptype.GNSSIDNavIC})
	if err != nil {
		t.Fatalf("EncodeProvideCapabilities: %v", err)
	}

	decoded, err := DecodeLPPMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if !decoded.ProvideCapabilities.GNSSCapability.Supports(models.GnssIDBds) {
		t.Error("expected BDS capability to be true")
	}

	if !decoded.ProvideCapabilities.GNSSCapability.Supports(models.GnssIDNavic) {
		t.Error("expected NavIC capability to be true")
	}
}

func TestEncodeDecodeProvideLocationInformation(t *testing.T) {
	originalLat := int32(48856000) // 48.856 degrees
	originalLon := int32(2352200)  // 2.3522 degrees
	originalAlt := int32(35000)    // 350m in cm

	encoded, err := EncodeProvideLocationInformation(0x04, originalLat, originalLon, originalAlt, 10, 15)
	if err != nil {
		t.Fatalf("EncodeProvideLocationInformation: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.TransactionID != 0x04 {
		t.Errorf("expected transaction ID 0x04, got 0x%02x", decoded.TransactionID)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideLocationInformation {
		t.Errorf("expected body kind ProvideLocationInformation, got %d", decoded.BodyKind)
	}

	if decoded.ProvideLocationInformation == nil {
		t.Fatal("expected ProvideLocationInformation to be non-nil")
	}

	e := decoded.ProvideLocationInformation.LocationEstimate
	if e == nil {
		t.Fatal("expected a location estimate")
	}

	if math.Abs(e.LatitudeDegrees-float64(originalLat)/1e7) > 1e-4 || math.Abs(e.LongitudeDegrees-float64(originalLon)/1e7) > 1e-4 {
		t.Errorf("position = %f, %f, want ~%f, ~%f", e.LatitudeDegrees, e.LongitudeDegrees, float64(originalLat)/1e7, float64(originalLon)/1e7)
	}

	if e.AltitudeMeters == nil || *e.AltitudeMeters != 350 {
		t.Errorf("altitude = %v, want 350 m", e.AltitudeMeters)
	}

	if e.UncertaintyEllipse == nil || math.Abs(e.UncertaintyEllipse.SemiMajorMeters-10) > 1.5 || math.Abs(e.UncertaintyEllipse.SemiMinorMeters-10) > 1.5 {
		t.Errorf("uncertainty ellipse = %+v, want ~10 m axes", e.UncertaintyEllipse)
	}

	if e.UncertaintyAltitudeMeters == nil || math.Abs(*e.UncertaintyAltitudeMeters-15) > 1.5 {
		t.Errorf("altitude uncertainty = %v, want ~15 m", e.UncertaintyAltitudeMeters)
	}

	if e.ConfidencePercent == nil || *e.ConfidencePercent != defaultConfidence {
		t.Errorf("confidence = %v, want %d", e.ConfidencePercent, defaultConfidence)
	}
}

func TestEncodeDecodeProvideAssistanceData(t *testing.T) {
	encoded, err := EncodeProvideAssistanceData(0x05, nil)
	if err != nil {
		t.Fatalf("EncodeProvideAssistanceData: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(encoded)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.TransactionID != 0x05 {
		t.Errorf("expected transaction ID 0x05, got 0x%02x", decoded.TransactionID)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideAssistanceData {
		t.Errorf("expected body kind ProvideAssistanceData, got %d", decoded.BodyKind)
	}
}

func TestParseLPPMessage(t *testing.T) {
	// Encode a ProvideCapabilities message and parse it with ParseLPPMessage.
	encoded, err := EncodeProvideCapabilities(0x01, []int64{lpptype.GNSSIDGPS})
	if err != nil {
		t.Fatalf("EncodeProvideCapabilities: %v", err)
	}

	msg, err := ParseLPPMessage(encoded)
	if err != nil {
		t.Fatalf("ParseLPPMessage: %v", err)
	}

	capMsg, ok := msg.(*models.ProvideLocationCapabilities)
	if !ok {
		t.Fatalf("expected *ProvideLocationCapabilities, got %T", msg)
	}

	if !capMsg.GNSSCapability.Supports(models.GnssIDGps) {
		t.Error("expected GPS capability to be true")
	}

	// Test empty payload.
	_, err = ParseLPPMessage([]byte{})
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestBuildRequestCapabilities(t *testing.T) {
	data, err := BuildRequestCapabilities(0x01, 0x00)
	if err != nil {
		t.Fatalf("BuildRequestCapabilities: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	// Verify the message can be decoded.
	decoded, err := DecodeLPPMessage(data)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentRequestCapabilities {
		t.Errorf("expected RequestCapabilities, got %d", decoded.BodyKind)
	}
}

func TestBuildAssistanceData(t *testing.T) {
	data, err := BuildAssistanceData(0x02)
	if err != nil {
		t.Fatalf("BuildAssistanceData: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(data)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideAssistanceData {
		t.Errorf("expected ProvideAssistanceData, got %d", decoded.BodyKind)
	}
}

func TestBuildLocationInformation(t *testing.T) {
	data, err := buildLocationInformation(0x03, 48856000, 2352200, 35000, 10, 15)
	if err != nil {
		t.Fatalf("BuildLocationInformation: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	decoded, err := DecodeLPPMessage(data)
	if err != nil {
		t.Fatalf("DecodeLPPMessage: %v", err)
	}

	if decoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideLocationInformation {
		t.Errorf("expected ProvideLocationInformation, got %d", decoded.BodyKind)
	}
}

// TestUERoundTrip simulates the full LMF↔UE LPP exchange:
// 1. LMF encodes RequestCapabilities → UE decodes it
// 2. UE encodes ProvideCapabilities → LMF decodes it
// 3. LMF encodes RequestLocationInformation → UE decodes it
// 4. UE encodes ProvideLocationInformation → LMF decodes it
func TestUERoundTrip(t *testing.T) {
	// Step 1: LMF → UE: RequestCapabilities
	lmfReqCaps, err := EncodeRequestCapabilities(0x01, 0x00)
	if err != nil {
		t.Fatalf("step 1 encode: %v", err)
	}

	decoded1, err := DecodeLPPMessage(lmfReqCaps)
	if err != nil {
		t.Fatalf("step 1 decode: %v", err)
	}

	if decoded1.TransactionID != 0x01 {
		t.Errorf("step 1: expected txID 0x01, got 0x%02x", decoded1.TransactionID)
	}

	if decoded1.BodyKind != lpptype.LPPMessageBodyC1PresentRequestCapabilities {
		t.Errorf("step 1: expected RequestCapabilities, got %d", decoded1.BodyKind)
	}

	// Step 2: UE → LMF: ProvideCapabilities
	ueProvCaps, err := EncodeProvideCapabilities(0x01, []int64{lpptype.GNSSIDGPS})
	if err != nil {
		t.Fatalf("step 2 encode: %v", err)
	}

	lmfDecoded, err := DecodeLPPMessage(ueProvCaps)
	if err != nil {
		t.Fatalf("step 2 decode: %v", err)
	}

	if lmfDecoded.BodyKind != lpptype.LPPMessageBodyC1PresentProvideCapabilities {
		t.Errorf("step 2: expected ProvideCapabilities, got %d", lmfDecoded.BodyKind)
	}

	if !lmfDecoded.ProvideCapabilities.GNSSCapability.Supports(models.GnssIDGps) {
		t.Error("step 2: expected GPS capability")
	}

	// Step 3: LMF → UE: RequestLocationInformation
	lmfReqLoc, err := EncodeRequestLocationInformation(0x02, 0x00, 25)
	if err != nil {
		t.Fatalf("step 3 encode: %v", err)
	}

	decoded3, err := DecodeLPPMessage(lmfReqLoc)
	if err != nil {
		t.Fatalf("step 3 decode: %v", err)
	}

	if decoded3.TransactionID != 0x02 {
		t.Errorf("step 3: expected txID 0x02, got 0x%02x", decoded3.TransactionID)
	}

	if decoded3.BodyKind != lpptype.LPPMessageBodyC1PresentRequestLocationInformation {
		t.Errorf("step 3: expected RequestLocationInformation, got %d", decoded3.BodyKind)
	}

	// Step 4: UE → LMF: ProvideLocationInformation
	ueProvLoc, err := EncodeProvideLocationInformation(0x02, 450000000, 214500000, 10000, 10, 15)
	if err != nil {
		t.Fatalf("step 4 encode: %v", err)
	}

	lmfDecoded2, err := DecodeLPPMessage(ueProvLoc)
	if err != nil {
		t.Fatalf("step 4 decode: %v", err)
	}

	if lmfDecoded2.BodyKind != lpptype.LPPMessageBodyC1PresentProvideLocationInformation {
		t.Errorf("step 4: expected ProvideLocationInformation, got %d", lmfDecoded2.BodyKind)
	}

	// Verify location is roughly correct (within encoding resolution).
	e := lmfDecoded2.ProvideLocationInformation.LocationEstimate
	if e == nil || math.Abs(e.LatitudeDegrees-45) > 1e-4 || math.Abs(e.LongitudeDegrees-21.45) > 1e-4 {
		t.Errorf("step 4: position = %+v, want ~45, ~21.45", e)
	}
}

// buildLocationInformation constructs an LPP ProvideLocationInformation message
// from a GNSS fix result. Latitude/longitude are in 1e-7 degrees, altitude in cm.
// hAcc and vAcc are horizontal/vertical accuracy in meters.
func buildLocationInformation(transactionID byte, lat int32, lon int32, alt int32, hAcc, vAcc uint32) ([]byte, error) {
	return EncodeProvideLocationInformation(transactionID, lat, lon, alt, hAcc, vAcc)
}

// TS 37.355 §4.3.3: a UE that sets ackRequested retransmits until acknowledged.
// The acknowledgement must echo the UE message's sequenceNumber in ackIndicator.
func TestBuildAcknowledgement(t *testing.T) {
	b, err := BuildAcknowledgement(1, 1)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if got := hex.EncodeToString(b); got != "600c02" {
		t.Errorf("acknowledgement: got %s, want 600c02", got)
	}
}
