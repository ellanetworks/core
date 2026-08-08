// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// CONTEXT ACCEPT through handleNAS (exercising ESM routing) and verifies the MME
// releases the session and the S1 context so the UE re-attaches.
func TestDeactivateBearerAcceptReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Deactivating = true

	plain, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: eps.EPSBearerIdentity(mme.DefaultERABID)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released after Deactivate Accept")
	}

	if ue.EMMState() != mme.EMMDeregistered {
		t.Fatal("UE not EMM-DEREGISTERED after Deactivate Accept")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected a UE Context Release Command after Deactivate Accept, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

// TS 23.401 §5.10.3 releases a PDN connection "including the default bearer of
// this PDN" without detaching, as long as another survives — whichever one it is.
func TestDeactivateBearerAcceptKeepsAUEWithAnotherPDN(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Deactivating = true
	ue.EnsurePDN(6)

	plain, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: eps.EPSBearerIdentity(mme.DefaultERABID)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if ue.EMMState() == mme.EMMDeregistered {
		t.Error("the UE was detached though it still has a PDN connection on EBI 6")
	}

	if m.LookupPDN(ue, mme.DefaultERABID) != nil {
		t.Error("the deactivated PDN connection was not released")
	}

	if m.LookupPDN(ue, 6) == nil {
		t.Error("the surviving PDN connection was released too")
	}

	if len(cc.sent) != 0 {
		t.Errorf("sent %d S1AP messages, want 0: releasing one of two PDNs must not release the UE context", len(cc.sent))
	}
}

// TestReconcileDataNetworkModifiesDNSOnly verifies a DNS-only change is applied
// in place with a MODIFY EPS BEARER CONTEXT REQUEST (no deactivation), mirroring
// the 5G PDU Session Modification path, and that dnConfig is committed only when
// the UE accepts.

// through handleNAS and verifies the pending data-network fingerprint is
// committed and the bearer stays up (no release).
func TestModifyBearerAcceptCommitsConfig(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Modifying = true
	testPDN(ue).PendingDNConfig = "10.45.0.0/16|fd45::/48|9.9.9.9|1500"

	plain, err := (&eps.ModifyEPSBearerContextAccept{EPSBearerIdentity: eps.EPSBearerIdentity(mme.DefaultERABID)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if testPDN(ue).Modifying {
		t.Fatal("UE still marked modifying after Modify Accept")
	}

	if testPDN(ue).DnConfig != "10.45.0.0/16|fd45::/48|9.9.9.9|1500" {
		t.Fatalf("dnConfig = %q, want the committed pending fingerprint", testPDN(ue).DnConfig)
	}

	if m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session released on a modification (must stay up)")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("modification accept must not trigger downlink S1AP, got %d", len(cc.sent))
	}
}

// TestReconcileUEIdleNoPanic checks reconciling a UE that has moved to ECM-IDLE
// returns without dereferencing the freed S1 connection.
