// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

func esmStatus(t *testing.T, ebi, pti, cause uint8) []byte {
	t.Helper()

	b, err := (&eps.ESMStatus{EPSBearerIdentity: ebi, ProcedureTransactionIdentity: pti, ESMCause: cause}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TestESMStatus_InvalidEPSBearerIdentityOnDefaultBearerDetaches verifies TS 24.301 §6.7:
// ESM cause #43 deactivates the named bearer locally, and for the default bearer that
// releases the UE context so the UE re-attaches (§6.4.4).
func TestESMStatus_InvalidEPSBearerIdentityOnDefaultBearerDetaches(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue)

	d := handleESMStatus(context.Background(), m, ue, esmStatus(t, mme.DefaultERABID, 0, esmCauseInvalidEPSBearerIdentity))

	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want handled (an ESM STATUS is never answered with another STATUS)", d)
	}

	if got := ue.PDNCount(); got != 0 {
		t.Fatalf("PDNCount = %d after ESM STATUS #43 on the default bearer, want 0", got)
	}

	if ue.EMMState() != mme.EMMDeregistered {
		t.Fatalf("emmState = %v after ESM STATUS #43 on the default bearer, want mme.EMMDeregistered", ue.EMMState())
	}
}

// TestESMStatus_InvalidEPSBearerIdentityOnAdditionalPDNReleasesOnlyThatPDN verifies that
// ESM cause #43 naming an additional PDN's bearer releases only that connection and leaves
// the UE connected (TS 24.301 §6.4.4.2).
func TestESMStatus_InvalidEPSBearerIdentityOnAdditionalPDNReleasesOnlyThatPDN(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue)
	ue.EnsurePDN(6)

	handleESMStatus(context.Background(), m, ue, esmStatus(t, 6, 0, esmCauseInvalidEPSBearerIdentity))

	if _, ok := ue.Pdns[6]; ok {
		t.Fatal("additional PDN retained after ESM STATUS #43 named its bearer")
	}

	if _, ok := ue.Pdns[mme.DefaultERABID]; !ok {
		t.Fatal("default PDN released by an ESM STATUS #43 naming an additional PDN's bearer")
	}

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatalf("emmState = %v after ESM STATUS #43 on an additional PDN, want mme.EMMRegistered", ue.EMMState())
	}
}

// TestESMStatus_UnknownEPSBearerIdentityIgnored verifies TS 24.301 §7.3.2 g): an EPS bearer
// identity matching no bearer context is ignored.
func TestESMStatus_UnknownEPSBearerIdentityIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue)

	d := handleESMStatus(context.Background(), m, ue, esmStatus(t, 9, 1, esmCauseInvalidEPSBearerIdentity))

	if d.Action != nasreply.ActionSilent || d.Reason != nasreply.ReasonNoContext {
		t.Fatalf("disposition = %+v, want a silent discard for an EPS bearer identity with no context", d)
	}

	if got := ue.PDNCount(); got != 1 {
		t.Fatalf("PDNCount = %d after ESM STATUS naming an unknown EPS bearer identity, want 1", got)
	}
}

// TestESMStatus_ReservedPTIIgnored verifies TS 24.301 §7.3.1 f): a reserved PTI names no
// transaction, so the message is ignored. Clause 7 applies ahead of the §6.7 cause handling,
// so even #43 takes no action.
func TestESMStatus_ReservedPTIIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue)

	d := handleESMStatus(context.Background(), m, ue, esmStatus(t, mme.DefaultERABID, esmPTIReserved, esmCauseInvalidEPSBearerIdentity))

	if d.Action != nasreply.ActionSilent || d.Reason != nasreply.ReasonOutOfState {
		t.Fatalf("disposition = %+v, want a silent discard for a reserved PTI", d)
	}

	if got := ue.PDNCount(); got != 1 {
		t.Fatalf("PDNCount = %d after ESM STATUS with a reserved PTI, want 1", got)
	}
}

// TestESMStatus_AbortingAnInFlightDeactivationReleasesPDN verifies that an ESM STATUS
// carrying a cause other than #43 still tears the PDN connection down when it aborts an
// in-flight deactivation (TS 24.301 §6.7: for #97 the MME aborts the procedure, and the
// local action for any other cause is implementation dependent). The user plane is released
// when the deactivation starts (TS 23.401 §5.4.4) and no reconcile sweep re-derives a
// UE-requested disconnect, so the connection is released here or never.
func TestESMStatus_AbortingAnInFlightDeactivationReleasesPDN(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue)
	p := ue.EnsurePDN(6)

	m.DisconnectBearer(context.Background(), ue, p, esmCauseRegularDeactivation, 3)

	if !ue.BearerDeactivating(p) {
		t.Fatal("deactivation not in flight after DisconnectBearer")
	}

	d := handleESMStatus(context.Background(), m, ue, esmStatus(t, 6, 3, nasreply.CauseMessageTypeNotImplemented))

	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want handled", d)
	}

	if _, ok := ue.Pdns[6]; ok {
		t.Fatal("PDN connection retained after an ESM STATUS aborted its deactivation; it holds an EPS bearer identity with no user plane and no radio bearer, and nothing retries the teardown")
	}

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatalf("emmState = %v after an additional PDN's deactivation was aborted, want mme.EMMRegistered", ue.EMMState())
	}
}

// TestESMStatus_UnrelatedCauseKeepsPDNAndClearsPendingModify verifies that an ESM STATUS
// with no deactivation in flight leaves the PDN connection up and only abandons the
// in-flight modification, leaving the stored config stale so the backstop retries.
func TestESMStatus_UnrelatedCauseKeepsPDNAndClearsPendingModify(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	p := testPDN(ue)
	p.Modifying = true
	p.PendingQCI = 7

	handleESMStatus(context.Background(), m, ue, esmStatus(t, mme.DefaultERABID, 0, nasreply.CauseProtocolErrorUnspecified))

	if got := ue.PDNCount(); got != 1 {
		t.Fatalf("PDNCount = %d after ESM STATUS #111 with no procedure in flight, want 1", got)
	}

	if p.Modifying || p.PendingQCI != 0 {
		t.Fatalf("pending modification not abandoned: Modifying = %v, PendingQCI = %d, want false and 0", p.Modifying, p.PendingQCI)
	}
}
