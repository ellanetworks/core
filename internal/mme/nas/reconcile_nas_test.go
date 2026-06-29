// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// CONTEXT ACCEPT through handleNAS (exercising ESM routing) and verifies the MME
// releases the session and the S1 context so the UE re-attaches.
func TestDeactivateBearerAcceptReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Deactivating = true

	plain, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: mme.DefaultERABID}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, uint8(ue.ULCount())), nascommon.DirectionUplink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue, wire)

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

	plain, err := (&eps.ModifyEPSBearerContextAccept{EPSBearerIdentity: mme.DefaultERABID}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, uint8(ue.ULCount())), nascommon.DirectionUplink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue, wire)

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
