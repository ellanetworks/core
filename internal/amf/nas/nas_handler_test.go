// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
)

func TestHandleNAS_ShortIntegrityProtectedPayload(t *testing.T) {
	shortPayload := []byte{0x7e, 0x01}

	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.UeConn{}

	HandleNAS(context.Background(), amfInstance, ue, shortPayload)

	if ue.UeContext() != nil {
		t.Fatal("short integrity-protected payload bound a UE context")
	}
}

func TestHandleNAS_NilPayload(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.UeConn{}

	HandleNAS(context.Background(), amfInstance, ue, nil)

	if ue.UeContext() != nil {
		t.Fatal("nil payload bound a UE context")
	}
}

func TestHandleNAS_SingleBytePayload(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.UeConn{}

	HandleNAS(context.Background(), amfInstance, ue, []byte{0x7e})

	if ue.UeContext() != nil {
		t.Fatal("single-byte payload bound a UE context")
	}
}

func TestHandleNAS_IntegrityProtectedPayloadExactly6Bytes(t *testing.T) {
	payload := []byte{0x7e, 0x01, 0x00, 0x00, 0x00, 0x00}

	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.UeConn{}

	HandleNAS(context.Background(), amfInstance, ue, payload)

	if ue.UeContext() != nil {
		t.Fatal("6-byte integrity-protected payload bound a UE context")
	}
}
