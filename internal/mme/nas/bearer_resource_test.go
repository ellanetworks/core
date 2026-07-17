// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

// TestBearerResourceAllocationRejected confirms the MME answers a UE-requested
// dedicated-bearer allocation with a BEARER RESOURCE ALLOCATION REJECT #31 rather
// than the ESM STATUS #97 an unhandled message type would draw (TS 24.301
// §6.5.3.4).
func TestBearerResourceAllocationRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req, err := (&eps.BearerResourceAllocationRequest{ProcedureTransactionIdentity: 3}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	d := HandleEmmMessage(context.Background(), m, ue, req, true)
	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want ActionHandled (a reject was already sent)", d)
	}

	reject, err := eps.ParseBearerResourceAllocationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Allocation Reject: %v", err)
	}

	if reject.ProcedureTransactionIdentity != 3 {
		t.Fatalf("PTI = %d, want 3 (echoed from the request)", reject.ProcedureTransactionIdentity)
	}

	if reject.ESMCause != esmCauseRequestRejectedUnspecified {
		t.Fatalf("ESM cause = %d, want %d (request rejected, unspecified)", reject.ESMCause, esmCauseRequestRejectedUnspecified)
	}
}

// TestBearerResourceModificationRejected confirms the MME answers a UE-requested
// dedicated-bearer modification with a BEARER RESOURCE MODIFICATION REJECT #31
// rather than the ESM STATUS #97 an unhandled message type would draw (TS 24.301
// §6.5.4.4).
func TestBearerResourceModificationRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req, err := (&eps.BearerResourceModificationRequest{ProcedureTransactionIdentity: 7}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	d := HandleEmmMessage(context.Background(), m, ue, req, true)
	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want ActionHandled (a reject was already sent)", d)
	}

	reject, err := eps.ParseBearerResourceModificationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Modification Reject: %v", err)
	}

	if reject.ProcedureTransactionIdentity != 7 {
		t.Fatalf("PTI = %d, want 7 (echoed from the request)", reject.ProcedureTransactionIdentity)
	}

	if reject.ESMCause != esmCauseRequestRejectedUnspecified {
		t.Fatalf("ESM cause = %d, want %d (request rejected, unspecified)", reject.ESMCause, esmCauseRequestRejectedUnspecified)
	}
}

// TestBearerResourceModificationInvalidPTI confirms the ESM header is validated
// before the unconditional refusal: an unassigned PTI draws ESM cause #81
// (TS 24.301 §7.3), as it does for the other UE-requested ESM procedures.
func TestBearerResourceModificationInvalidPTI(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req, err := (&eps.BearerResourceModificationRequest{ProcedureTransactionIdentity: 0}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(context.Background(), m, ue, req, true)

	reject, err := eps.ParseBearerResourceModificationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Modification Reject: %v", err)
	}

	if reject.ESMCause != esmCauseInvalidPTIValue {
		t.Fatalf("ESM cause = %d, want %d (invalid PTI value)", reject.ESMCause, esmCauseInvalidPTIValue)
	}
}
