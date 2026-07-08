// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

// TestIngestAttachRequestStoresDRX verifies the UE's requested DRX parameter is
// parsed and stored, mirroring the AMF's UESpecificDRX (parity; the value is
// otherwise unused today — 4G echoes it only in NB-IoT and neither stack pages
// with it yet).
func TestIngestAttachRequestStoresDRX(t *testing.T) {
	ue := &mme.UeContext{}
	drx := []byte{0x00, 0x08}

	ingestAttachRequest(ue, &eps.AttachRequest{DRXParameter: drx})

	if !bytes.Equal(ue.DRXParameter, drx) {
		t.Fatalf("DRXParameter = %x, want %x", ue.DRXParameter, drx)
	}

	// Omitted DRX parameter leaves it nil.
	ue2 := &mme.UeContext{}
	ingestAttachRequest(ue2, &eps.AttachRequest{})

	if ue2.DRXParameter != nil {
		t.Fatalf("DRXParameter = %x, want nil when omitted", ue2.DRXParameter)
	}
}

func TestIngestAttachRequestExtractsAPN(t *testing.T) {
	apnIE, err := eps.MarshalAPN("ims")
	if err != nil {
		t.Fatalf("MarshalAPN: %v", err)
	}

	esm, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE,
	}).Marshal()
	if err != nil {
		t.Fatalf("marshal PDN Connectivity Request: %v", err)
	}

	ue := &mme.UeContext{}
	ingestAttachRequest(ue, &eps.AttachRequest{ESMMessageContainer: esm})

	if ue.RequestedAPN != "ims" {
		t.Errorf("requestedAPN = %q, want %q", ue.RequestedAPN, "ims")
	}

	// No APN IE → empty (use the default policy).
	esm2, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).Marshal()
	if err != nil {
		t.Fatalf("marshal PDN Connectivity Request (no APN): %v", err)
	}

	ue2 := &mme.UeContext{}
	ingestAttachRequest(ue2, &eps.AttachRequest{ESMMessageContainer: esm2})

	if ue2.RequestedAPN != "" {
		t.Errorf("requestedAPN = %q, want empty for an attach without an APN", ue2.RequestedAPN)
	}
}
