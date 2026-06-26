// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"strings"
	"testing"

	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// connectedBearerUE returns a secured, registered, ECM-CONNECTED UE with a
// default bearer, ready for data-network reconciliation.
func connectedBearerUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	ue, cc := securedUE(t, m)
	p := testPDN(ue)
	p.apn = "internet"

	// Record the QoS a real activation would, so a reconcile against an unchanged
	// policy is a no-op.
	if qos, err := m.resolveQoSByAPN(context.Background(), ue.imsi, p.apn); err == nil {
		p.sessAmbrDLBps = bitRateToBps(qos.SessAmbrDLStr)
		p.sessAmbrULBps = bitRateToBps(qos.SessAmbrULStr)
		p.qci = qos.QCI
		p.arp = qos.ARP
	}

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
	m.freeS1Conn(ue) // an idle UE picks up the change on its next attach
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

	m.handleNAS(context.Background(), ue, wire)

	if !m.session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released after Deactivate Accept")
	}

	if ue.emmState.load() != EMMDeregistered {
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

// TestReconcileDataNetworkModifiesSessionAMBR verifies a Session-AMBR change is
// applied in place with a MODIFY EPS BEARER CONTEXT REQUEST carrying the new
// APN-AMBR, that the UPF QER is updated, and that the stored Session-AMBR is
// committed only when the UE accepts.
func TestReconcileDataNetworkModifiesSessionAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.pdnType = eps.PDNTypeIPv4

	qos, err := m.resolveQoS(context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	wantDL := bitRateToBps(qos.SessAmbrDLStr)
	wantUL := bitRateToBps(qos.SessAmbrULStr)

	// DN config unchanged; only the stored Session-AMBR differs from the policy.
	p.dnConfig = qos.dnFingerprint()
	p.sessAmbrDLBps = wantDL / 2
	p.sessAmbrULBps = wantUL / 2

	m.ReconcileDataNetwork(context.Background())

	defer m.stopNASGuard(ue)

	if !p.modifying {
		t.Fatal("UE not marked modifying after a Session-AMBR change")
	}

	if p.deactivating {
		t.Fatal("Session-AMBR change must not deactivate the bearer")
	}

	fsm := m.session.(*fakeSessionManager)
	if !fsm.ambrUpdated || fsm.ambrUplink != qos.SessAmbrULStr || fsm.ambrDownlink != qos.SessAmbrDLStr {
		t.Fatalf("UPF Session-AMBR not updated to %s/%s, got %+v", qos.SessAmbrULStr, qos.SessAmbrDLStr, fsm)
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

	req, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse Modify request: %v", err)
	}

	ambr, err := eps.ParseAPNAMBR(req.APNAMBR)
	if err != nil {
		t.Fatalf("Modify request missing APN-AMBR: %v", err)
	}

	if dl, ul := ambr.BitsPerSecond(); dl != wantDL || ul != wantUL {
		t.Fatalf("APN-AMBR = %d/%d bps, want %d/%d", dl, ul, wantDL, wantUL)
	}

	if p.sessAmbrDLBps == wantDL {
		t.Fatal("Session-AMBR committed before the UE accepted the modification")
	}
}

// TestReconcileDataNetworkDefersAMBROnQERFailure verifies that when the UPF QER
// update fails the modification is aborted rather than signalled to the UE: the
// stored Session-AMBR is left stale so the next reconcile retries, avoiding a
// silent UE/UPF divergence (AC4).
func TestReconcileDataNetworkDefersAMBROnQERFailure(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.pdnType = eps.PDNTypeIPv4

	qos, err := m.resolveQoS(context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	staleDL := bitRateToBps(qos.SessAmbrDLStr) / 2
	staleUL := bitRateToBps(qos.SessAmbrULStr) / 2

	p.dnConfig = qos.dnFingerprint()
	p.sessAmbrDLBps = staleDL
	p.sessAmbrULBps = staleUL

	m.session.(*fakeSessionManager).ambrErr = errors.New("upf unavailable")

	m.ReconcileDataNetwork(context.Background())

	if p.modifying {
		t.Fatal("modification marked in-flight despite the QER update failing")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("UE signalled a Session-AMBR the data plane rejected: %d message(s) sent", len(cc.sent))
	}

	if p.sessAmbrDLBps != staleDL || p.sessAmbrULBps != staleUL {
		t.Fatal("stored Session-AMBR changed; the next reconcile would not retry")
	}
}

// TestReconcileDataNetworkModifiesQoSViaERABModify verifies a QCI/ARP change is
// carried in an S1AP E-RAB Modify Request (not a Downlink NAS Transport) with the
// new E-RAB QoS, that the piggybacked NAS-PDU is a Modify EPS Bearer Context
// Request with the new EPS QoS, and that the stored QoS commits only on accept.
func TestReconcileDataNetworkModifiesQoSViaERABModify(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.pdnType = eps.PDNTypeIPv4

	qos, err := m.resolveQoS(context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	// DN and Session-AMBR unchanged; only the QCI/ARP differ from the stored values.
	p.dnConfig = qos.dnFingerprint()
	p.sessAmbrDLBps = bitRateToBps(qos.SessAmbrDLStr)
	p.sessAmbrULBps = bitRateToBps(qos.SessAmbrULStr)
	p.qci = qos.QCI + 1
	p.arp = qos.ARP + 1

	m.ReconcileDataNetwork(context.Background())

	defer m.stopNASGuard(ue)

	if !p.modifying {
		t.Fatal("UE not marked modifying after a QoS change")
	}

	if p.deactivating {
		t.Fatal("QoS change must not deactivate the bearer")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one E-RAB Modify Request, got %d", len(cc.sent))
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatalf("unmarshal S1AP: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcERABModify {
		t.Fatalf("got %T, want E-RAB Modify Request", pdu)
	}

	req, err := s1ap.ParseERABModifyRequest(im.Value)
	if err != nil {
		t.Fatalf("parse E-RAB Modify Request: %v", err)
	}

	if len(req.ERABToBeModified) != 1 {
		t.Fatalf("expected one E-RAB, got %d", len(req.ERABToBeModified))
	}

	item := req.ERABToBeModified[0]
	if uint8(item.QoS.QCI) != qos.QCI || item.QoS.ARP.PriorityLevel != qos.ARP {
		t.Fatalf("E-RAB QoS = QCI %d ARP %d, want %d/%d", item.QoS.QCI, item.QoS.ARP.PriorityLevel, qos.QCI, qos.ARP)
	}

	nasWire := []byte(item.NASPDU)

	plain, err := eps.Unprotect(nasWire, nascommon.NASCount(0, nasWire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect piggybacked NAS: %v", err)
	}

	nasReq, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse piggybacked Modify request: %v", err)
	}

	if len(nasReq.NewEPSQoS) == 0 || nasReq.NewEPSQoS[0] != qos.QCI {
		t.Fatalf("NAS New-EPS-QoS = % x, want QCI %d", nasReq.NewEPSQoS, qos.QCI)
	}

	if p.qci == qos.QCI {
		t.Fatal("QCI committed before the UE accepted the modification")
	}
}

// TestReconcileDataNetworkModifiesQoSAndAMBRTogether verifies a combined QCI/ARP
// and Session-AMBR change is carried in a single E-RAB Modify Request whose
// piggybacked NAS-PDU contains both the new EPS QoS and the new APN-AMBR, and that
// the UPF QER is updated for the new Session-AMBR.
func TestReconcileDataNetworkModifiesQoSAndAMBRTogether(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.pdnType = eps.PDNTypeIPv4

	qos, err := m.resolveQoS(context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	wantDL := bitRateToBps(qos.SessAmbrDLStr)
	wantUL := bitRateToBps(qos.SessAmbrULStr)

	p.dnConfig = qos.dnFingerprint()
	p.sessAmbrDLBps = wantDL / 2 // Session-AMBR changed
	p.sessAmbrULBps = wantUL / 2
	p.qci = qos.QCI + 1 // QoS changed
	p.arp = qos.ARP + 1

	m.ReconcileDataNetwork(context.Background())

	defer m.stopNASGuard(ue)

	fsm := m.session.(*fakeSessionManager)
	if !fsm.ambrUpdated {
		t.Fatal("UPF Session-AMBR not updated on a combined QoS+AMBR change")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one E-RAB Modify Request, got %d", len(cc.sent))
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcERABModify {
		t.Fatalf("got %T, want E-RAB Modify Request", pdu)
	}

	req, err := s1ap.ParseERABModifyRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	nasWire := []byte(req.ERABToBeModified[0].NASPDU)

	plain, err := eps.Unprotect(nasWire, nascommon.NASCount(0, nasWire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	nasReq, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatal(err)
	}

	if len(nasReq.NewEPSQoS) == 0 || nasReq.NewEPSQoS[0] != qos.QCI {
		t.Fatalf("piggybacked NAS missing New-EPS-QoS: % x", nasReq.NewEPSQoS)
	}

	ambr, err := eps.ParseAPNAMBR(nasReq.APNAMBR)
	if err != nil {
		t.Fatalf("piggybacked NAS missing APN-AMBR: %v", err)
	}

	if dl, ul := ambr.BitsPerSecond(); dl != wantDL || ul != wantUL {
		t.Fatalf("piggybacked APN-AMBR = %d/%d, want %d/%d", dl, ul, wantDL, wantUL)
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

	m.handleNAS(context.Background(), ue, wire)

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

// TestReconcileUEIdleNoPanic checks reconciling a UE that has moved to ECM-IDLE
// returns without dereferencing the freed S1 connection.
func TestReconcileUEIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).apn = "internet"
	m.freeS1Conn(ue)

	m.reconcileUE(context.Background(), ue)
}
