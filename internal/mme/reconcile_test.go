// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func connectedBearerUE(t *testing.T, m *MME) (*UeContext, *captureConn) {
	t.Helper()

	ue, cc := securedUE(t, m)
	p := testPDN(ue)
	p.Apn = "internet"
	p.SessionRef = "ref-internet"
	p.PdnType = eps.PDNTypeIPv4
	p.Qci = 9
	p.Arp = 1
	p.SessAmbrDLBps = 200_000_000
	p.SessAmbrULBps = 100_000_000

	return ue, cc
}

func testAmbr(ul, dl string) *models.Ambr {
	return &models.Ambr{Uplink: models.MustParseBitRate(ul), Downlink: models.MustParseBitRate(dl)}
}

func sentModifyRequest(t *testing.T, ue *UeContext, wire []byte) *eps.ModifyEPSBearerContextRequest {
	t.Helper()

	plain, err := unprotected(eps.Unprotect(wire, nas.MakeCount(0, wire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, ue.knasInt, ue.knasEnc)))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	req, err := eps.ParseModifyEPSBearerContextRequest(plain)
	if err != nil {
		t.Fatalf("parse Modify EPS Bearer Context Request: %v", err)
	}

	return req
}

func sentERABModify(t *testing.T, pdu []byte) *s1ap.ERABModifyRequest {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal S1AP: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcERABModify {
		t.Fatalf("got %T, want E-RAB Modify Request", msg)
	}

	req, err := s1ap.ParseERABModifyRequest(im.Value)
	if err != nil {
		t.Fatalf("parse E-RAB Modify Request: %v", err)
	}

	return req
}

func TestReactivateEPSBearerRequestsReactivation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	if err := m.ReactivateEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	if !testPDN(ue).Deactivating {
		t.Fatal("bearer not marked deactivating")
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

func TestModifyEPSBearerRefusesAnIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	m.FreeUeConn(t.Context(), ue)

	err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("1 Gbps", "1 Gbps")})
	if !errors.Is(err, ErrUENotReachable) {
		t.Fatalf("ModifyEPSBearer error = %v, want ErrUENotReachable", err)
	}

	if cc.count() != 0 {
		t.Fatalf("an idle UE was sent %d messages", cc.count())
	}
}

func TestModifyEPSBearerRefusesABusyBearer(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Deactivating = true

	err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("1 Gbps", "1 Gbps")})
	if !errors.Is(err, ErrBearerBusy) {
		t.Fatalf("ModifyEPSBearer error = %v, want ErrBearerBusy", err)
	}

	if cc.count() != 0 {
		t.Fatalf("a bearer under deactivation was sent %d messages", cc.count())
	}
}

func TestModifyEPSBearerDNSOnly(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	dns := netip.MustParseAddr("9.9.9.9")

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{DNS: dns, MTU: 1400}); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Modify EPS Bearer Context Request, got %d", len(cc.sent))
	}

	req := sentModifyRequest(t, ue, decodeDownlinkNAS(t, cc.sent[0]))

	want := nas.NewProtocolConfigurationOptions(nas.DNSServers(dns), 1400)
	if req.ProtocolConfigurationOptions == nil || !slices.EqualFunc(req.ProtocolConfigurationOptions.Containers, want.Containers, func(a, b nas.PCOContainer) bool {
		return a.ID == b.ID && string(a.Content) == string(b.Content)
	}) {
		t.Fatalf("PCO = %+v, want %+v", req.ProtocolConfigurationOptions, want)
	}

	if req.NewEPSQoS != nil || req.APNAMBR != nil {
		t.Fatalf("a DNS-only change carried QoS %+v / APN-AMBR %+v", req.NewEPSQoS, req.APNAMBR)
	}

	if p.Dns == dns {
		t.Fatal("DNS committed before the UE accepted the modification")
	}
}

func TestModifyEPSBearerSessionAMBR(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	ambr := testAmbr("300 Mbps", "400 Mbps")

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: ambr}); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	if len(cc.sent) != 1 {
		t.Fatalf("expected one Modify EPS Bearer Context Request, got %d", len(cc.sent))
	}

	req := sentModifyRequest(t, ue, decodeDownlinkNAS(t, cc.sent[0]))

	if req.APNAMBR == nil {
		t.Fatal("Modify request missing APN-AMBR")
	}

	if dl, ul, ok := req.APNAMBR.Kbps(); !ok || dl*1000 != ambr.Downlink.Bps() || ul*1000 != ambr.Uplink.Bps() {
		t.Fatalf("APN-AMBR = %d/%d kbit/s, want %s/%s", dl, ul, ambr.Downlink, ambr.Uplink)
	}

	if p.SessAmbrDLBps == ambr.Downlink.Bps() {
		t.Fatal("Session-AMBR committed before the UE accepted the modification")
	}
}

func TestModifyEPSBearerQoSViaERABModify(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	p := testPDN(ue)
	ambr := testAmbr("300 Mbps", "400 Mbps")

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{
		QoS:     &models.EPSBearerQoS{QCI: 8, ARP: 2},
		APNAMBR: ambr,
	}); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	if len(cc.sent) != 1 {
		t.Fatalf("expected one E-RAB Modify Request, got %d", len(cc.sent))
	}

	req := sentERABModify(t, cc.sent[0])

	if len(req.ERABToBeModified) != 1 {
		t.Fatalf("expected one E-RAB, got %d", len(req.ERABToBeModified))
	}

	item := req.ERABToBeModified[0]
	if uint8(item.QoS.QCI) != 8 || item.QoS.ARP.PriorityLevel != 2 {
		t.Fatalf("E-RAB QoS = QCI %d ARP %d, want 8/2", item.QoS.QCI, item.QoS.ARP.PriorityLevel)
	}

	nasReq := sentModifyRequest(t, ue, []byte(item.NASPDU))

	if nasReq.NewEPSQoS == nil || nasReq.NewEPSQoS.QCI != 8 {
		t.Fatalf("NAS New EPS QoS = %+v, want QCI 8", nasReq.NewEPSQoS)
	}

	if nasReq.APNAMBR == nil {
		t.Fatal("piggybacked NAS missing APN-AMBR")
	}

	if p.Qci == 8 {
		t.Fatal("QCI committed before the UE accepted the modification")
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

			testPDN(ue).Transferred = tc.transferred

			before := cc.count()

			if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{DNS: netip.MustParseAddr("9.9.9.9"), MTU: 1400}); err != nil {
				t.Fatal(err)
			}

			defer ue.Conn().StopNASGuard(t.Context())

			if cc.count() == before {
				t.Fatal("no MODIFY EPS BEARER CONTEXT REQUEST was sent")
			}

			req := sentModifyRequest(t, ue, decodeDownlinkNAS(t, cc.sent[len(cc.sent)-1]))

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

func TestModifyEPSBearerCarriesTheMappedFiveGSQoS(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)

	mapped := []nas.PCOContainer{
		{ID: nas.PCOContainerSessionAMBR, Content: []byte{0x06, 0x00, 0x01, 0x06, 0x00, 0x01}},
		{ID: nas.PCOContainerQoSFlowDescriptions, Content: []byte{0x01, 0x20, 0x41, 0x01, 0x01, 0x09}},
	}

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{
		APNAMBR:         testAmbr("300 Mbps", "400 Mbps"),
		MappedFiveGSQoS: mapped,
	}); err != nil {
		t.Fatal(err)
	}

	defer ue.Conn().StopNASGuard(t.Context())

	req := sentModifyRequest(t, ue, decodeDownlinkNAS(t, cc.sent[0]))

	if req.ProtocolConfigurationOptions == nil {
		t.Fatal("the modification carries no protocol configuration options, so the UE keeps its stale mapped 5GS QoS")
	}

	for _, want := range mapped {
		if !slices.ContainsFunc(req.ProtocolConfigurationOptions.Containers, func(c nas.PCOContainer) bool {
			return c.ID == want.ID && string(c.Content) == string(want.Content)
		}) {
			t.Errorf("container %#x missing from the modification", want.ID)
		}
	}
}

func TestReconcileUEAsksTheSMFPerPDN(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)

	ims := ue.EnsurePDN(6)
	ims.Apn = "ims"
	ims.SessionRef = "ref-ims"

	m.ReconcileUE(context.Background(), ue)

	got := m.Session.(*fakeSessionManager).reconciled
	slices.Sort(got)

	if !slices.Equal(got, []string{"ref-ims", "ref-internet"}) {
		t.Fatalf("reconciled sessions = %v, want both PDN connections", got)
	}
}

func TestReconcileUEIdleNoPanic(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	m.FreeUeConn(t.Context(), ue)

	m.ReconcileUE(context.Background(), ue)

	if cc.count() != 0 {
		t.Fatalf("an idle UE cannot be sent a reconfiguration, got %d messages", cc.count())
	}

	if got := m.Session.(*fakeSessionManager).reconciled; len(got) != 0 {
		t.Fatalf("an idle UE's sessions were reconciled: %v", got)
	}

	if ue.Conn() != nil {
		t.Fatal("the freed connection was resurrected")
	}
}

func TestS1ReleaseEndsAnInFlightModification(t *testing.T) {
	m := newTestMME(t)
	ue, _ := connectedBearerUE(t, m)
	p := testPDN(ue)

	if err := m.ModifyEPSBearer(context.Background(), ue.imsiOrEmpty(), DefaultERABID, models.EPSBearerModification{APNAMBR: testAmbr("300 Mbps", "400 Mbps")}); err != nil {
		t.Fatal(err)
	}

	m.FreeUeConn(t.Context(), ue)

	ue.mu.Lock()
	modifying := p.Modifying
	ue.mu.Unlock()

	if modifying != nil {
		t.Fatal("the modification outlived the S1 connection, so every later reconcile finds the bearer busy")
	}
}
