// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppa

import (
	"testing"
)

// TestRoundTrip_ECIDMeasurementInitiationFailure is Stage A: it exercises the
// whole envelope / open-type / CHOICE machinery by building a Failure PDU
// (LMF-UE-Measurement-ID + Cause radioNetwork=unspecified), encoding it,
// decoding it, and asserting the fields survive the round trip.
func TestRoundTrip_ECIDMeasurementInitiationFailure(t *testing.T) {
	const lmfMeasID = int64(7)

	cause := Cause{Group: CauseGroupRadioNetwork, Value: 0} // unspecified

	encoded, err := BuildECIDMeasurementInitiationFailure(lmfMeasID, cause)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationFailure: %v", err)
	}

	if len(encoded) == 0 {
		t.Fatal("encoded PDU is empty")
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationFailure {
		t.Fatalf("kind: got %d, want KindECIDMeasurementInitiationFailure", parsed.Kind)
	}

	if parsed.Failure == nil {
		t.Fatal("parsed.Failure is nil")
	}

	if parsed.Failure.LMFUEMeasurementID != lmfMeasID {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want %d", parsed.Failure.LMFUEMeasurementID, lmfMeasID)
	}

	if parsed.Failure.Cause.Group != CauseGroupRadioNetwork {
		t.Errorf("cause group: got %d, want CauseGroupRadioNetwork", parsed.Failure.Cause.Group)
	}

	if parsed.Failure.Cause.Value != 0 {
		t.Errorf("cause value: got %d, want 0 (unspecified)", parsed.Failure.Cause.Value)
	}
}

// TestRoundTrip_ECIDFailure_CauseProtocol checks an extensible-ENUMERATED Cause
// value (protocol/semantic-error) survives the round trip.
func TestRoundTrip_ECIDFailure_CauseProtocol(t *testing.T) {
	const lmfMeasID = int64(3)

	cause := Cause{Group: CauseGroupProtocol, Value: 4} // semantic-error

	encoded, err := BuildECIDMeasurementInitiationFailure(lmfMeasID, cause)
	if err != nil {
		t.Fatalf("BuildECIDMeasurementInitiationFailure: %v", err)
	}

	parsed, err := ParsePDU(encoded)
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}

	if parsed.Kind != KindECIDMeasurementInitiationFailure || parsed.Failure == nil {
		t.Fatalf("unexpected parse result: %+v", parsed)
	}

	if parsed.Failure.LMFUEMeasurementID != lmfMeasID {
		t.Errorf("LMF-UE-Measurement-ID: got %d, want %d", parsed.Failure.LMFUEMeasurementID, lmfMeasID)
	}

	if parsed.Failure.Cause.Group != CauseGroupProtocol || parsed.Failure.Cause.Value != 4 {
		t.Errorf("cause: got group=%d value=%d, want group=protocol value=4",
			parsed.Failure.Cause.Group, parsed.Failure.Cause.Value)
	}
}
