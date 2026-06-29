// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

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
