// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/s1ap"
)

func connectedBearerUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	ue, cc := securedUE(t, m)
	p := testPDN(ue)
	p.Apn = "internet"

	if qos, err := ResolveQoSByAPN(context.Background(), m, ue.imsiOrEmpty(), p.Apn); err == nil {
		p.SessAmbrDLBps = qos.SessAmbrDL.Bps()
		p.SessAmbrULBps = qos.SessAmbrUL.Bps()
		p.Qci = qos.QCI
		p.Arp = qos.ARP
	}

	return ue, cc
}

func TestReconcileDataNetworkReactivatesChangedBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	testPDN(ue).DnConfig = "stale|config|0.0.0.0|0"

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

	if !testPDN(ue).Deactivating {
		t.Fatal("UE not marked deactivating after a data-network change")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Deactivate EPS Bearer Context Request, got %d", len(cc.sent))
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	req, err := eps.ParseDeactivateEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse Deactivate EPS Bearer Context Request: %v", err)
	}

	if req.Cause != eps.ESMCauseReactivationRequested {
		t.Fatalf("ESM cause = %d, want %d (reactivation requested)", req.Cause, eps.ESMCauseReactivationRequested)
	}
}

func TestReconcileDataNetworkSkipsUnchanged(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	testPDN(ue).DnConfig = qos.DnFingerprint()

	m.ReconcileDataNetwork(context.Background())

	if testPDN(ue).Deactivating {
		t.Fatal("UE reactivated despite an unchanged data-network config")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("expected no signalling for an unchanged config, got %d", len(cc.sent))
	}
}

// TS 23.401 §5.4.4.1
func TestReconcileDataNetworkDeactivatesOnUnknownAPN(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	testPDN(ue).Apn = "removed-apn"

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

	if !testPDN(ue).Deactivating {
		t.Fatal("UE not marked deactivating after its APN was unbound")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Deactivate EPS Bearer Context Request, got %d", len(cc.sent))
	}
}

// TS 23.501 §5.6.14
func TestReconcileDataNetworkReactivatesOnFramedRouteChange(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	testPDN(ue).DnConfig = qos.DnFingerprint()

	m.Session.(*fakeSessionManager).framedChanged = true

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

	if !testPDN(ue).Deactivating {
		t.Fatal("UE not marked deactivating after a framed-route change")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Deactivate EPS Bearer Context Request, got %d", len(cc.sent))
	}
}

// TS 24.301 §6.4.4.2.
func TestReconcileDataNetworkReactivatesOnStaticIPChange(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	testPDN(ue).DnConfig = qos.DnFingerprint()

	m.Session.(*fakeSessionManager).staticIPChanged = true

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

	if !testPDN(ue).Deactivating {
		t.Fatal("UE not marked deactivating after a static IP change")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Deactivate EPS Bearer Context Request, got %d", len(cc.sent))
	}
}

func TestReconcileDataNetworkSkipsIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	m.FreeUeConn(ue)
	testPDN(ue).DnConfig = "stale|config|0.0.0.0|0"

	m.ReconcileDataNetwork(context.Background())

	if testPDN(ue).Deactivating || len(cc.sent) != 0 {
		t.Fatalf("idle UE should not be signalled; deactivating=%v sent=%d", testPDN(ue).Deactivating, len(cc.sent))
	}
}

func TestReconcileDataNetworkModifiesDNSOnly(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(qos.DnFingerprint(), "|")
	parts[2] = "9.9.9.9"
	testPDN(ue).DnConfig = strings.Join(parts, "|")

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

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

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
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

func TestReconcileDataNetworkModifiesSessionAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	wantDL := qos.SessAmbrDL.Bps()
	wantUL := qos.SessAmbrUL.Bps()

	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = wantDL / 2
	p.SessAmbrULBps = wantUL / 2

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

	if !p.Modifying {
		t.Fatal("UE not marked modifying after a Session-AMBR change")
	}

	if p.Deactivating {
		t.Fatal("Session-AMBR change must not deactivate the bearer")
	}

	fsm := m.Session.(*fakeSessionManager)
	if !fsm.ambrUpdated || fsm.ambrUplink != qos.SessAmbrUL || fsm.ambrDownlink != qos.SessAmbrDL {
		t.Fatalf("UPF Session-AMBR not updated to %s/%s, got %+v", qos.SessAmbrUL, qos.SessAmbrDL, fsm)
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Modify EPS Bearer Context Request, got %d", len(cc.sent))
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	req, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse Modify request: %v", err)
	}

	if req.APNAMBR == nil {
		t.Fatal("Modify request missing APN-AMBR")
	}

	ambr := *req.APNAMBR

	if dl, ul, ok := ambr.Kbps(); !ok || dl*1000 != wantDL || ul*1000 != wantUL {
		t.Fatalf("APN-AMBR = %d/%d kbit/s, want %d/%d bit/s", dl, ul, wantDL, wantUL)
	}

	if p.SessAmbrDLBps == wantDL {
		t.Fatal("Session-AMBR committed before the UE accepted the modification")
	}
}

func TestReconcileDataNetworkDefersAMBROnQERFailure(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	staleDL := qos.SessAmbrDL.Bps() / 2
	staleUL := qos.SessAmbrUL.Bps() / 2

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

func TestReconcileDataNetworkModifiesQoSViaERABModify(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = qos.SessAmbrDL.Bps()
	p.SessAmbrULBps = qos.SessAmbrUL.Bps()
	p.Qci = qos.QCI + 1
	p.Arp = qos.ARP + 1

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

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

	plain, err := unprotected(eps.Unprotect(nasWire, nas.MakeCount(0, nasWire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect piggybacked NAS: %v", err)
	}

	nasReq, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse piggybacked Modify request: %v", err)
	}

	if nasReq.NewEPSQoS == nil || nasReq.NewEPSQoS.QCI != qos.QCI {
		t.Fatalf("NAS New-EPS-QoS = %+v, want QCI %d", nasReq.NewEPSQoS, qos.QCI)
	}

	if p.Qci == qos.QCI {
		t.Fatal("QCI committed before the UE accepted the modification")
	}
}

func TestReconcileDataNetworkModifiesQoSAndAMBRTogether(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.PdnType = eps.PDNTypeIPv4

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	wantDL := qos.SessAmbrDL.Bps()
	wantUL := qos.SessAmbrUL.Bps()

	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = wantDL / 2
	p.SessAmbrULBps = wantUL / 2
	p.Qci = qos.QCI + 1
	p.Arp = qos.ARP + 1

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

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

	plain, err := unprotected(eps.Unprotect(nasWire, nas.MakeCount(0, nasWire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatal(err)
	}

	nasReq, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatal(err)
	}

	if nasReq.NewEPSQoS == nil || nasReq.NewEPSQoS.QCI != qos.QCI {
		t.Fatalf("piggybacked NAS missing New-EPS-QoS: %+v", nasReq.NewEPSQoS)
	}

	if nasReq.APNAMBR == nil {
		t.Fatal("piggybacked NAS missing APN-AMBR")
	}

	ambr := *nasReq.APNAMBR

	if dl, ul, ok := ambr.Kbps(); !ok || dl*1000 != wantDL || ul*1000 != wantUL {
		t.Fatalf("piggybacked APN-AMBR = %d/%d, want %d/%d", dl, ul, wantDL, wantUL)
	}
}

func TestReconcileUEIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	m.FreeUeConn(ue)

	m.ReconcileUE(context.Background(), ue)

	if cc.count() != 0 {
		t.Fatalf("an idle UE cannot be sent a reconfiguration, got %d messages", cc.count())
	}

	if ue.Conn() != nil {
		t.Fatal("the freed connection was resurrected")
	}
}

// TS 24.301 §8.3.18.9
func TestModifyBearerFollowsTheConnectionsPCOElement(t *testing.T) {
	for _, tc := range []struct {
		name         string
		transferred  bool
		wantExtended bool
	}{
		{"transferred from a PDU session", true, true},
		{"ordinary PDN connection", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, cc := connectedBearerUE(t, m)

			ue.SetUESecurityCapability(eps.UENetworkCapability{HasUMTS: true, Rest: []byte{0x00, 0x80, 0x20}}, nil, MintAuthProofForTrackingAreaUpdate())

			p := testPDN(ue)
			p.Transferred = tc.transferred

			qos, err := ResolveQoSByAPN(context.Background(), m, ue.imsiOrEmpty(), p.Apn)
			if err != nil {
				t.Fatalf("resolve QoS: %v", err)
			}

			before := cc.count()

			m.modifyBearer(context.Background(), ue, ue.Conn(), p, qos, true, false, false)

			if cc.count() == before {
				t.Fatal("no MODIFY EPS BEARER CONTEXT REQUEST was sent")
			}

			wire := decodeDownlinkNAS(t, cc.sent[len(cc.sent)-1])

			plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
			if err != nil {
				t.Fatalf("unprotect downlink: %v", err)
			}

			req, err := eps.ParseModifyEPSBearerContextRequest(plain)
			if err != nil {
				t.Fatalf("parse Modify request: %v", err)
			}

			if tc.wantExtended {
				if req.ExtendedProtocolConfigurationOptions == nil {
					t.Error("the modification sent the classic element to a connection that uses the extended one")
				}

				if req.ProtocolConfigurationOptions != nil {
					t.Error("both elements were sent; §8.3.18.9/.13 make them exclusive")
				}

				return
			}

			if req.ProtocolConfigurationOptions == nil {
				t.Error("the modification sent no classic element to a connection that uses it")
			}

			if req.ExtendedProtocolConfigurationOptions != nil {
				t.Error("the extended element went to a connection that never took it")
			}
		})
	}
}

func TestReconcileSessionAMBRRefreshesTheMappedFiveGSQoS(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	p.PdnType = eps.PDNTypeIPv4
	p.Snssai = &models.Snssai{Sst: 1}
	p.PDUSessionID = 5

	qos, err := ResolveQoS(context.Background(), m, ue.imsiOrEmpty())
	if err != nil {
		t.Fatal(err)
	}

	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = qos.SessAmbrDL.Bps() / 2
	p.SessAmbrULBps = qos.SessAmbrUL.Bps() / 2

	m.ReconcileDataNetwork(context.Background())

	defer ue.Conn().StopNASGuard()

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Modify EPS Bearer Context Request, got %d", len(cc.sent))
	}

	wire := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	req, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse Modify request: %v", err)
	}

	if req.ProtocolConfigurationOptions == nil {
		t.Fatal("the modification carries no protocol configuration options, so the UE keeps its stale mapped 5GS QoS")
	}

	var ambrValue, flowsValue []byte

	for _, c := range req.ProtocolConfigurationOptions.Containers {
		switch c.ID {
		case nas.PCOContainerSessionAMBR:
			ambrValue = c.Content
		case nas.PCOContainerQoSFlowDescriptions:
			flowsValue = c.Content
		case nas.PCOContainerQoSRules:
			t.Error("the modification re-sends the default QoS rule, which the UE rejects with 5GSM cause #83 (TS 24.501 §6.1.4.1 case a)7)")
		}
	}

	if ambrValue == nil {
		t.Fatal("no mapped Session-AMBR container in the modification")
	}

	if flowsValue == nil {
		t.Fatal("no mapped QoS flow descriptions container in the modification")
	}

	flows, err := fgs.ParseQoSFlowDescriptions(flowsValue)
	if err != nil {
		t.Fatalf("parse the mapped QoS flow descriptions: %v", err)
	}

	if len(flows) != 1 || flows[0].QFI != models.DefaultQFI || flows[0].OperationCode != fgs.QoSFlowOpCreate {
		t.Errorf("mapped QoS flow descriptions = %+v, want one create for QFI %d", flows, models.DefaultQFI)
	}

	ambr, err := fgs.ParseSessionAMBR(ambrValue)
	if err != nil {
		t.Fatalf("parse the mapped Session-AMBR: %v", err)
	}

	dl, ul, ok := ambr.Kbps()
	if !ok || dl != qos.SessAmbrDL.Kbps() || ul != qos.SessAmbrUL.Kbps() {
		t.Errorf("mapped Session-AMBR = %d/%d kbps, want the policy's %d/%d", dl, ul, qos.SessAmbrDL.Kbps(), qos.SessAmbrUL.Kbps())
	}
}
