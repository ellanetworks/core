// Copyright 2025 Ella Networks

package context_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf/context"
)

func TestDecodePayloadTooShort(t *testing.T) {
	ue := &context.AmfUe{}
	payload := []byte{0x00, 0x01, 0x02}

	_, err := ue.DecodeNASMessage(payload)
	if err == nil {
		t.Fatal("expected error when payload is too short, got nil")
	}

	expectedError := "nas payload is too short"
	if err.Error() != expectedError {
		t.Fatalf("expected error %q, got %q", expectedError, err.Error())
	}
}
