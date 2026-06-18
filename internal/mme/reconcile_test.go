// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"strings"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
)

// connectedBearerUE returns a secured, registered, ECM-CONNECTED UE with a
// default bearer, ready for data-network reconciliation.
func connectedBearerUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	ue, cc := securedUE(t, m)
	testPDN(ue).apn = "internet"
	ue.ecmState = ECMConnected

	return ue, cc
}

func TestReconcileDataNetworkReactivatesChangedBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	// A fingerprint that differs from the current resolved config simulates a
	// data-network reconfiguration applied while the bearer was up.
	testPDN(ue).dnConfig = "stale|config|0.0.0.0|0"

	m.ReconcileDataNetwork(context.Background())

	defer m.stopNASGuard(ue)

	if !testPDN(ue).deactivating {
		t.Fatal("UE not marked deactivating after a data-network change")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Deactivate EPS Bearer Context Request, got %d", len(cc.sent))
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	req, err := eps.ParseDeactivateEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse Deactivate EPS Bearer Context Request: %v", err)
	}

	if req.ESMCause != eps.ESMCauseReactivationRequested {
		t.Fatalf("ESM cause = %d, want %d (reactivation requested)", req.ESMCause, eps.ESMCauseReactivationRequested)
	}
}

func TestReconcileDataNetworkSkipsUnchanged(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	qos, err := m.resolveQoS(context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	testPDN(ue).dnConfig = qos.dnFingerprint() // matches current → no change

	m.ReconcileDataNetwork(context.Background())

	if testPDN(ue).deactivating {
		t.Fatal("UE reactivated despite an unchanged data-network config")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("expected no signalling for an unchanged config, got %d", len(cc.sent))
	}
}

func TestReconcileDataNetworkSkipsIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	ue.ecmState = ECMIdle // an idle UE picks up the change on its next attach
	testPDN(ue).dnConfig = "stale|config|0.0.0.0|0"

	m.ReconcileDataNetwork(context.Background())

	if testPDN(ue).deactivating || len(cc.sent) != 0 {
		t.Fatalf("idle UE should not be signalled; deactivating=%v sent=%d", testPDN(ue).deactivating, len(cc.sent))
	}
}

// TestDeactivateBearerAcceptReleases drives the uplink DEACTIVATE EPS BEARER
// CONTEXT ACCEPT through handleNAS (exercising ESM routing) and verifies the MME
// releases the session and the S1 context so the UE re-attaches.
func TestDeactivateBearerAcceptReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).deactivating = true

	plain, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: defaultERABID}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, uint8(ue.ulCount)), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, wire)

	if !m.session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released after Deactivate Accept")
	}

	if ue.emmState != EMMDeregistered {
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
func TestReconcileDataNetworkModifiesDNSOnly(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).pdnType = eps.PDNTypeIPv4

	qos, err := m.resolveQoS(context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	// A fingerprint identical to the current one except the DNS field: only DNS
	// changed, so the bearer is modified in place rather than reactivated.
	parts := strings.Split(qos.dnFingerprint(), "|")
	parts[2] = "9.9.9.9"
	testPDN(ue).dnConfig = strings.Join(parts, "|")

	m.ReconcileDataNetwork(context.Background())

	defer m.stopNASGuard(ue)

	if !testPDN(ue).modifying {
		t.Fatal("UE not marked modifying after a DNS-only change")
	}

	if testPDN(ue).deactivating {
		t.Fatal("DNS-only change must not deactivate the bearer")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Modify EPS Bearer Context Request, got %d", len(cc.sent))
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	mt, err := eps.PeekESMMessageType(plain)
	if err != nil || mt != eps.MsgModifyEPSBearerContextRequest {
		t.Fatalf("message type = %#x (err %v), want Modify EPS Bearer Context Request", mt, err)
	}

	if testPDN(ue).dnConfig == qos.dnFingerprint() {
		t.Fatal("dnConfig committed before the UE accepted the modification")
	}
}

// TestModifyBearerAcceptCommitsConfig drives a MODIFY EPS BEARER CONTEXT ACCEPT
// through handleNAS and verifies the pending data-network fingerprint is
// committed and the bearer stays up (no release).
func TestModifyBearerAcceptCommitsConfig(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).modifying = true
	testPDN(ue).pendingDNConfig = "10.45.0.0/16|fd45::/48|9.9.9.9|1500"

	plain, err := (&eps.ModifyEPSBearerContextAccept{EPSBearerIdentity: defaultERABID}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, uint8(ue.ulCount)), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, wire)

	if testPDN(ue).modifying {
		t.Fatal("UE still marked modifying after Modify Accept")
	}

	if testPDN(ue).dnConfig != "10.45.0.0/16|fd45::/48|9.9.9.9|1500" {
		t.Fatalf("dnConfig = %q, want the committed pending fingerprint", testPDN(ue).dnConfig)
	}

	if m.session.(*fakeSessionManager).released {
		t.Fatal("EPS session released on a modification (must stay up)")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("modification accept must not trigger downlink S1AP, got %d", len(cc.sent))
	}
}
