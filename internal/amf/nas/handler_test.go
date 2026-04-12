// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
)

func TestHandleNAS_ShortIntegrityProtectedPayload(t *testing.T) {
	// 0x7e = 5GS Mobility Management EPD
	// 0x01 = SecurityHeaderTypeIntegrityProtected
	// Total length is 2 bytes, well below the 7-byte minimum required
	// for integrity-protected NAS messages. This must return an error,
	// not panic.
	shortPayload := []byte{0x7e, 0x01}

	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.RanUe{} // AmfUe is nil, so HandleNAS enters fetchUeContextWithMobileIdentity

	err := HandleNAS(context.Background(), amfInstance, ue, shortPayload)
	if err == nil {
		t.Fatal("expected error for short integrity-protected payload, got nil")
	}
}

func TestHandleNAS_NilPayload(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.RanUe{}

	err := HandleNAS(context.Background(), amfInstance, ue, nil)
	if err == nil {
		t.Fatal("expected error for nil payload, got nil")
	}
}

func TestHandleNAS_SingleBytePayload(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.RanUe{}

	err := HandleNAS(context.Background(), amfInstance, ue, []byte{0x7e})
	if err == nil {
		t.Fatal("expected error for single-byte payload, got nil")
	}
}

func TestHandleNAS_IntegrityProtectedPayloadExactly6Bytes(t *testing.T) {
	// 6 bytes: still too short for integrity-protected (needs >= 7)
	payload := []byte{0x7e, 0x01, 0x00, 0x00, 0x00, 0x00}

	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.RanUe{}

	err := HandleNAS(context.Background(), amfInstance, ue, payload)
	if err == nil {
		t.Fatal("expected error for 6-byte integrity-protected payload, got nil")
	}
}
