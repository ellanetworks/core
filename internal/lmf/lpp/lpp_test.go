// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

func TestEncodeDecodeRequestLocationInformation(t *testing.T) {
	original := &models.RequestLocationInformation{
		TransactionID:     0x01,
		PositioningMethod: PosMethodGNSS,
		NumberOfSVs:       4,
	}

	encoded, err := EncodeRequestLocationInformation(original)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	decoded, err := DecodeRequestLocationInformation(encoded)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if decoded.TransactionID != original.TransactionID {
		t.Errorf("expected transaction ID %d, got %d", original.TransactionID, decoded.TransactionID)
	}

	if decoded.PositioningMethod != original.PositioningMethod {
		t.Errorf("expected positioning method %d, got %d", original.PositioningMethod, decoded.PositioningMethod)
	}

	if decoded.NumberOfSVs != original.NumberOfSVs {
		t.Errorf("expected num SVs %d, got %d", original.NumberOfSVs, decoded.NumberOfSVs)
	}
}

// TestDecodeRequestLocationInformationTruncatedSVs ensures a payload whose
// numberOfSVs presence bit is set but which is missing the trailing value
// byte is decoded without panicking (index-out-of-range).
func TestDecodeRequestLocationInformationTruncatedSVs(t *testing.T) {
	// 4 bytes: type, transactionID, positioningMethod, presence byte (0x80)
	// with the high bit set but no following numberOfSVs value.
	data := []byte{MsgTypeRequestLocationInformation, 0x01, PosMethodGNSS, 0x80}

	decoded, err := DecodeRequestLocationInformation(data)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if decoded.NumberOfSVs != 0 {
		t.Errorf("expected num SVs 0 for truncated payload, got %d", decoded.NumberOfSVs)
	}
}

func TestEncodeDecodeProvideAssistanceData(t *testing.T) {
	original := &models.ProvideAssistanceData{
		TransactionID:      0x02,
		GNSSAssistanceData: []byte{0x01, 0x02, 0x03, 0x04},
	}

	encoded, err := EncodeProvideAssistanceData(original)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	decoded, err := DecodeProvideAssistanceData(encoded)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if decoded.TransactionID != original.TransactionID {
		t.Errorf("expected transaction ID %d, got %d", original.TransactionID, decoded.TransactionID)
	}

	if len(decoded.GNSSAssistanceData) != len(original.GNSSAssistanceData) {
		t.Errorf("expected assistance data length %d, got %d", len(original.GNSSAssistanceData), len(decoded.GNSSAssistanceData))
	}

	for i := range original.GNSSAssistanceData {
		if decoded.GNSSAssistanceData[i] != original.GNSSAssistanceData[i] {
			t.Errorf("assistance data[%d] mismatch: expected %d, got %d", i, original.GNSSAssistanceData[i], decoded.GNSSAssistanceData[i])
		}
	}
}

func TestEncodeDecodeProvideLocationInformation(t *testing.T) {
	original := &models.ProvideLocationInformation{
		TransactionID: 0x03,
		GNSSPositionResult: models.GNSSPositionResult{
			Latitude:           48856000,
			Longitude:          2352200,
			Altitude:           35000,
			HorizontalAccuracy: 10,
			VerticalAccuracy:   15,
			Timestamp:          time.Now().UnixMilli(),
		},
	}

	encoded, err := EncodeProvideLocationInformation(original)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	decoded, err := DecodeProvideLocationInformation(encoded)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if decoded.TransactionID != original.TransactionID {
		t.Errorf("expected transaction ID %d, got %d", original.TransactionID, decoded.TransactionID)
	}

	if decoded.GNSSPositionResult.Latitude != original.GNSSPositionResult.Latitude {
		t.Errorf("expected latitude %d, got %d", original.GNSSPositionResult.Latitude, decoded.GNSSPositionResult.Latitude)
	}

	if decoded.GNSSPositionResult.Longitude != original.GNSSPositionResult.Longitude {
		t.Errorf("expected longitude %d, got %d", original.GNSSPositionResult.Longitude, decoded.GNSSPositionResult.Longitude)
	}

	if decoded.GNSSPositionResult.Altitude != original.GNSSPositionResult.Altitude {
		t.Errorf("expected altitude %d, got %d", original.GNSSPositionResult.Altitude, decoded.GNSSPositionResult.Altitude)
	}

	if decoded.GNSSPositionResult.HorizontalAccuracy != original.GNSSPositionResult.HorizontalAccuracy {
		t.Errorf("expected horizontal accuracy %d, got %d", original.GNSSPositionResult.HorizontalAccuracy, decoded.GNSSPositionResult.HorizontalAccuracy)
	}

	if decoded.GNSSPositionResult.VerticalAccuracy != original.GNSSPositionResult.VerticalAccuracy {
		t.Errorf("expected vertical accuracy %d, got %d", original.GNSSPositionResult.VerticalAccuracy, decoded.GNSSPositionResult.VerticalAccuracy)
	}
}

func TestParseLPPMessage(t *testing.T) {
	// Test ProvideLocationCapabilities parsing
	capData := []byte{MsgTypeProvideLocationCapabilities, 0x01, 0x03}

	msg, err := ParseLPPMessage(capData)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	capMsg, ok := msg.(*models.ProvideLocationCapabilities)
	if !ok {
		t.Fatalf("expected *ProvideLocationCapabilities, got %T", msg)
	}

	if capMsg.TransactionID != 0x01 {
		t.Errorf("expected transaction ID 0x01, got 0x%02x", capMsg.TransactionID)
	}

	// Test unknown message type
	_, err = ParseLPPMessage([]byte{0xFF})
	if err == nil {
		t.Fatal("expected error for unknown message type")
	}

	// Test empty payload
	_, err = ParseLPPMessage([]byte{})
	if err == nil {
		t.Fatal("expected error for empty payload")
	}
}

func TestBuildRequestCapabilities(t *testing.T) {
	data, err := BuildRequestCapabilities(0x01)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	if data[0] != MsgTypeRequestLocationInformation {
		t.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeRequestLocationInformation, data[0])
	}
}

func TestBuildAssistanceData(t *testing.T) {
	data, err := BuildAssistanceData(0x02)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	if data[0] != MsgTypeProvideAssistanceData {
		t.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeProvideAssistanceData, data[0])
	}
}

func TestBuildLocationInformation(t *testing.T) {
	data, err := BuildLocationInformation(0x03, 48856000, 2352200, 35000, 10, 15)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty encoded data")
	}

	if data[0] != MsgTypeProvideLocationInformation {
		t.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeProvideLocationInformation, data[0])
	}
}
