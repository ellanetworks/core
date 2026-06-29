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
	p.Apn = "internet"

	// Record the QoS a real activation would, so a reconcile against an unchanged
	// policy is a no-op.
	if qos, err := ResolveQoSByAPN(m, context.Background(), ue.imsi, p.Apn); err == nil {
		p.SessAmbrDLBps = BitRateToBps(qos.SessAmbrDLStr)
		p.SessAmbrULBps = BitRateToBps(qos.SessAmbrULStr)
		p.Qci = qos.QCI
		p.Arp = qos.ARP
	}

	return ue, cc
}

func TestReconcileDataNetworkReactivatesChangedBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	// A fingerprint that differs from the current resolved config simulates a
	// data-network reconfiguration applied while the bearer was up.
	testPDN(ue).DnConfig = "stale|config|0.0.0.0|0"

	m.ReconcileDataNetwork(context.Background())

	defer m.StopNASGuard(ue)

	if !testPDN(ue).Deactivating {
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

	qos, err := ResolveQoS(m, context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	testPDN(ue).DnConfig = qos.DnFingerprint() // matches current → no change

	m.ReconcileDataNetwork(context.Background())

	if testPDN(ue).Deactivating {
		t.Fatal("UE reactivated despite an unchanged data-network config")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("expected no signalling for an unchanged config, got %d", len(cc.sent))
	}
}

func TestReconcileDataNetworkSkipsIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	m.FreeS1Conn(ue) // an idle UE picks up the change on its next attach
	testPDN(ue).DnConfig = "stale|config|0.0.0.0|0"

	m.ReconcileDataNetwork(context.Background())

	if testPDN(ue).Deactivating || len(cc.sent) != 0 {
		t.Fatalf("idle UE should not be signalled; deactivating=%v sent=%d", testPDN(ue).Deactivating, len(cc.sent))
	}
}

// TestDeactivateBearerAcceptReleases drives the uplink DEACTIVATE EPS BEARER
func TestReconcileDataNetworkModifiesDNSOnly(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(m, context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	// A fingerprint identical to the current one except the DNS field: only DNS
	// changed, so the bearer is modified in place rather than reactivated.
	parts := strings.Split(qos.DnFingerprint(), "|")
	parts[2] = "9.9.9.9"
	testPDN(ue).DnConfig = strings.Join(parts, "|")

	m.ReconcileDataNetwork(context.Background())

	defer m.StopNASGuard(ue)

	if !testPDN(ue).Modifying {
		t.Fatal("UE not marked modifying after a DNS-only change")
	}

	if testPDN(ue).Deactivating {
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

	if testPDN(ue).DnConfig == qos.DnFingerprint() {
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
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(m, context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	wantDL := BitRateToBps(qos.SessAmbrDLStr)
	wantUL := BitRateToBps(qos.SessAmbrULStr)

	// DN config unchanged; only the stored Session-AMBR differs from the policy.
	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = wantDL / 2
	p.SessAmbrULBps = wantUL / 2

	m.ReconcileDataNetwork(context.Background())

	defer m.StopNASGuard(ue)

	if !p.Modifying {
		t.Fatal("UE not marked modifying after a Session-AMBR change")
	}

	if p.Deactivating {
		t.Fatal("Session-AMBR change must not deactivate the bearer")
	}

	fsm := m.Session.(*fakeSessionManager)
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

	if p.SessAmbrDLBps == wantDL {
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
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(m, context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	staleDL := BitRateToBps(qos.SessAmbrDLStr) / 2
	staleUL := BitRateToBps(qos.SessAmbrULStr) / 2

	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = staleDL
	p.SessAmbrULBps = staleUL

	m.Session.(*fakeSessionManager).ambrErr = errors.New("upf unavailable")

	m.ReconcileDataNetwork(context.Background())

	if p.Modifying {
		t.Fatal("modification marked in-flight despite the QER update failing")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("UE signalled a Session-AMBR the data plane rejected: %d message(s) sent", len(cc.sent))
	}

	if p.SessAmbrDLBps != staleDL || p.SessAmbrULBps != staleUL {
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
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(m, context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	// DN and Session-AMBR unchanged; only the QCI/ARP differ from the stored values.
	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = BitRateToBps(qos.SessAmbrDLStr)
	p.SessAmbrULBps = BitRateToBps(qos.SessAmbrULStr)
	p.Qci = qos.QCI + 1
	p.Arp = qos.ARP + 1

	m.ReconcileDataNetwork(context.Background())

	defer m.StopNASGuard(ue)

	if !p.Modifying {
		t.Fatal("UE not marked modifying after a QoS change")
	}

	if p.Deactivating {
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

	if p.Qci == qos.QCI {
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
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(m, context.Background(), ue.imsi)
	if err != nil {
		t.Fatal(err)
	}

	wantDL := BitRateToBps(qos.SessAmbrDLStr)
	wantUL := BitRateToBps(qos.SessAmbrULStr)

	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = wantDL / 2 // Session-AMBR changed
	p.SessAmbrULBps = wantUL / 2
	p.Qci = qos.QCI + 1 // QoS changed
	p.Arp = qos.ARP + 1

	m.ReconcileDataNetwork(context.Background())

	defer m.StopNASGuard(ue)

	fsm := m.Session.(*fakeSessionManager)
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
func TestReconcileUEIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	m.FreeS1Conn(ue)

	m.ReconcileUE(context.Background(), ue)
}
