// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

const (
	farIDDownlink   = 2
	farIDForwarding = 3
)

func establishSecondEPSSession(t *testing.T, s *smf.SMF) *smf.SMContext {
	t.Helper()

	req := epsRequest(3)
	req.APN = testDNN
	req.PDUSessionID = arrivingPDUSessionID + 1
	req.EPSBearerIdentity = epsTestEBI + 1
	req.Snssai = testSnssai

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEPSSession (second PDN connection): %v", err)
	}

	if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI+1, sourceENB); err != nil {
		t.Fatalf("ModifyEPSSession (second PDN connection): %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("the second EPS session is not in the pool")
	}

	return sc
}

func reportFor(seid uint64, an smf.AnchorBinding) *models.ErrorIndicationReport {
	addr, _ := netip.AddrFromSlice(an.IPv4.To4())

	return &models.ErrorIndicationReport{
		SEID:        seid,
		FARID:       farIDDownlink,
		RemoteFTEID: models.FTEID{TEID: an.TEID, Addr: addr},
	}
}

// TS 23.527 §5.3.2 step 4: the SMF modifies the session to buffer the downlink,
// then re-establishes the user plane.
func TestErrorIndicationBuffersAndRepagesA5GSSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	an := smCtx.Tunnel.AN

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(smCtx.PFCPContext.SEID, an)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if smCtx.Tunnel.Downlink != smf.DownlinkBuffering {
		t.Errorf("downlink state = %v, want buffering", smCtx.Tunnel.Downlink)
	}

	if smCtx.Tunnel.AN.IPv4 != nil || smCtx.Tunnel.AN.TEID != 0 {
		t.Errorf("the dead access endpoint is still bound: %+v", smCtx.Tunnel.AN)
	}

	if got := amfCb.releasedAccess(); len(got) != 1 || got[0] != smCtx.PDUSessionID {
		t.Errorf("access resources released for %v, want one release of PDU session %d (TS 23.527 §5.3.2 step 5)",
			got, smCtx.PDUSessionID)
	}

	// TS 38.413 §8.2.2.2: the 5G-AN only has to accept a Setup for this PDU
	// session ID once it has answered the Release Command.
	if len(amfCb.pageCalls) != 0 {
		t.Errorf("the user plane was re-activated before the release was acknowledged: %d transfers", len(amfCb.pageCalls))
	}

	if _, err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), ref); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp: %v", err)
	}

	if len(amfCb.pageCalls) != 1 {
		t.Errorf("AMF got %d N2-transfer-or-page calls after the release response, want 1 (TS 23.527 §5.3.2 step 8)",
			len(amfCb.pageCalls))
	}
}

// TS 23.502 §4.3.7 deactivates the UP connection of an *existing* PDU session, so
// the release response must not tear the session down.
func TestAccessReleaseKeepsThePDUSessionEstablished(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)
	ueIP := smCtx.PDUIPV4Address

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(smCtx.PFCPContext.SEID, smCtx.Tunnel.AN)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if _, err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), ref); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp: %v", err)
	}

	if s.GetSession(ref) == nil {
		t.Fatal("the PDU session was removed by a release that only deactivates the UP connection")
	}

	if smCtx.Tunnel == nil || smCtx.PFCPContext == nil {
		t.Fatal("the user plane was torn down by a release that only deactivates the UP connection")
	}

	if !smCtx.PDUIPV4Address.Equal(ueIP) {
		t.Errorf("the UE address was released: %v, want %v", smCtx.PDUIPV4Address, ueIP)
	}
}

// TS 23.527 §5.3.2 step 5 releases the AN resources before step 8 re-activates
// them; a CM-IDLE UE holds none, which is not a failure (step 6).
func TestErrorIndicationStillRepagesWhenTheUEIsUnreachable(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	amfCb.accessReleaseErr = smf.ErrUENotReachable

	smCtx, _ := setupSessionWithTunnel(t, s)

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(smCtx.PFCPContext.SEID, smCtx.Tunnel.AN)); err != nil {
		t.Fatalf("a CM-IDLE UE made the Error Indication fail: %v", err)
	}

	if len(amfCb.pageCalls) != 1 {
		t.Errorf("AMF got %d pages, want 1", len(amfCb.pageCalls))
	}
}

// TS 23.007 §21.7: the SGW drops the eNodeB TEIDs, notifies the MME and buffers.
func TestErrorIndicationBuffersAndRepagesAnEPSSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	mme := &fakeMME{}
	s.SetMME(mme)

	smCtx := establishEPSForArrival(t, s)
	smCtx.Tunnel.AN = smf.AnchorBinding{TEID: 7000, IPv4: net.ParseIP("10.0.0.200").To4()}
	smCtx.Tunnel.Downlink = smf.DownlinkForwarding

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(smCtx.PFCPContext.SEID, smCtx.Tunnel.AN)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if smCtx.Tunnel.Downlink != smf.DownlinkBuffering {
		t.Errorf("downlink state = %v, want buffering", smCtx.Tunnel.Downlink)
	}

	if len(mme.pagedIMSI) != 1 {
		t.Errorf("MME got %d pages, want 1 (TS 29.274 §7.2.11 Downlink Data Notification)", len(mme.pagedIMSI))
	}

	if len(mme.notifyCauses) != 1 || mme.notifyCauses[0] != models.DownlinkDataErrorIndication {
		t.Errorf("Downlink Data Notification causes = %v, want one %d (TS 29.274 §8.4 cause 6)",
			mme.notifyCauses, models.DownlinkDataErrorIndication)
	}

	if len(amfCb.releasedAccess()) != 0 {
		t.Error("an EPS session released 5G access resources")
	}
}

// TS 23.007 §21.7 clears every eNodeB GTP-U tunnel of the UE, not just the one
// the Error Indication named.
func TestErrorIndicationClearsEveryEPSTunnelOfTheUE(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(&fakeMME{})

	reported := establishEPSForArrival(t, s)
	reported.Tunnel.AN = smf.AnchorBinding{TEID: 7000, IPv4: net.ParseIP("10.0.0.200").To4()}
	reported.Tunnel.Downlink = smf.DownlinkForwarding

	other := establishSecondEPSSession(t, s)
	other.Tunnel.AN = smf.AnchorBinding{TEID: 7001, IPv4: net.ParseIP("10.0.0.200").To4()}
	other.Tunnel.Downlink = smf.DownlinkForwarding

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(reported.PFCPContext.SEID, reported.Tunnel.AN)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if other.Tunnel.Downlink != smf.DownlinkBuffering {
		t.Errorf("the UE's other PDN connection still forwards into the dead eNB: state = %v", other.Tunnel.Downlink)
	}

	if other.Tunnel.AN.IPv4 != nil {
		t.Errorf("the UE's other eNB tunnel is still bound: %+v", other.Tunnel.AN)
	}
}

// TS 23.007 §21.7 drops every eNodeB tunnel of the UE and then notifies the MME,
// so one PDN connection failing must not strand the others or skip the
// notification.
func TestErrorIndicationClearsTheOtherEPSTunnelsWhenOneFails(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	mme := &fakeMME{}
	s.SetMME(mme)

	reported := establishEPSForArrival(t, s)
	reported.Tunnel.AN = smf.AnchorBinding{TEID: 7000, IPv4: net.ParseIP("10.0.0.200").To4()}
	reported.Tunnel.Downlink = smf.DownlinkForwarding

	failing := establishSecondEPSSession(t, s)
	failing.Tunnel.AN = smf.AnchorBinding{TEID: 7001, IPv4: net.ParseIP("10.0.0.200").To4()}
	failing.Tunnel.Downlink = smf.DownlinkForwarding

	upf.modifyErrBySEID = map[uint64]error{failing.PFCPContext.SEID: errors.New("pfcp modification failed")}

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(reported.PFCPContext.SEID, reported.Tunnel.AN)); err != nil {
		t.Fatalf("one failing PDN connection failed the whole Error Indication: %v", err)
	}

	if reported.Tunnel.Downlink != smf.DownlinkBuffering {
		t.Errorf("the reported tunnel still forwards into the dead eNB: state = %v", reported.Tunnel.Downlink)
	}

	if len(mme.pagedIMSI) != 1 {
		t.Errorf("MME got %d Downlink Data Notifications, want 1", len(mme.pagedIMSI))
	}
}

// A failure on the reported session withholds the notification: its downlink is
// the one known to be flowing into a dead tunnel.
func TestErrorIndicationReportsAFailureOnTheBrokenSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	mme := &fakeMME{}
	s.SetMME(mme)

	reported := establishEPSForArrival(t, s)
	reported.Tunnel.AN = smf.AnchorBinding{TEID: 7000, IPv4: net.ParseIP("10.0.0.200").To4()}
	reported.Tunnel.Downlink = smf.DownlinkForwarding

	upf.modifyErrBySEID = map[uint64]error{reported.PFCPContext.SEID: errors.New("pfcp modification failed")}

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(reported.PFCPContext.SEID, reported.Tunnel.AN)); err == nil {
		t.Fatal("a failed deactivation of the broken tunnel was reported as success")
	}

	if len(mme.pagedIMSI) != 0 {
		t.Errorf("the MME was notified though the broken tunnel is still forwarding: %d notifications", len(mme.pagedIMSI))
	}
}

// An access-only release that the 5G-AN never answers must not make a later
// session release skip its teardown: the UE address, the N4 session and the SEID
// would all leak.
func TestUnansweredAccessReleaseDoesNotSwallowASessionRelease(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(smCtx.PFCPContext.SEID, smCtx.Tunnel.AN)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	// The 5G-AN never answers; a UE-requested release follows, which runs the
	// network-requested release procedure and waits for its own response.
	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionReleaseRequest(smCtx.PDUSessionID, 5)); err != nil {
		t.Fatalf("UpdateSmContextN1Msg (release request): %v", err)
	}

	before := len(amfCb.pageCalls)

	if _, err := s.UpdateSmContextN2InfoPduResRelRsp(context.Background(), ref); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResRelRsp: %v", err)
	}

	if len(amfCb.pageCalls) != before {
		t.Error("a stale access-release purpose swallowed the session release's response and re-activated the session instead")
	}

	if !smCtx.N2ReleasedForTest() {
		t.Error("the session release's N2 leg was never recorded, so the session can never complete its release")
	}
}

// An Error Indication for an endpoint the session has already moved off must not
// tear down the endpoint that replaced it.
func TestErrorIndicationForASupersededEndpointIsIgnored(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, _ := setupSessionWithTunnel(t, s)

	stale := smf.AnchorBinding{TEID: smCtx.Tunnel.AN.TEID + 1, IPv4: smCtx.Tunnel.AN.IPv4}

	if err := s.HandleErrorIndicationReport(context.Background(), reportFor(smCtx.PFCPContext.SEID, stale)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if smCtx.Tunnel.Downlink != smf.DownlinkForwarding {
		t.Errorf("a superseded Error Indication stopped the downlink: state = %v", smCtx.Tunnel.Downlink)
	}

	if len(amfCb.pageCalls) != 0 {
		t.Errorf("a superseded Error Indication paged the UE %d times", len(amfCb.pageCalls))
	}
}

func TestRepeatedErrorIndicationsActOnce(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, _ := setupSessionWithTunnel(t, s)
	report := reportFor(smCtx.PFCPContext.SEID, smCtx.Tunnel.AN)

	for range 5 {
		if err := s.HandleErrorIndicationReport(context.Background(), report); err != nil {
			t.Fatalf("HandleErrorIndicationReport: %v", err)
		}
	}

	if got := amfCb.releasedAccess(); len(got) != 1 {
		t.Errorf("a peer answering every downlink packet released the access resources %d times, want 1", len(got))
	}
}

func forwardingReport(seid uint64, target models.FTEID) *models.ErrorIndicationReport {
	return &models.ErrorIndicationReport{SEID: seid, FARID: farIDForwarding, RemoteFTEID: target}
}

func epsSessionForwardingTo(t *testing.T, s *smf.SMF, target models.FTEID) *smf.SMContext {
	t.Helper()

	sc := establishEPSForArrival(t, s)
	sc.Tunnel.AN = smf.AnchorBinding{TEID: 7000, IPv4: net.ParseIP("10.0.0.200").To4()}
	sc.Tunnel.Downlink = smf.DownlinkForwarding

	if _, err := s.OpenEPSForwardingTunnel(context.Background(), sc.Ref, target); err != nil {
		t.Fatalf("OpenEPSForwardingTunnel: %v", err)
	}

	return sc
}

// The handover target has discarded the tunnel, so relaying into it for the rest
// of the indirect data forwarding timer (TS 23.502 §4.9.1.3.3) achieves nothing.
func TestErrorIndicationReleasesAForwardingTunnelEarly(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	upf.forwardingTEID = 4242

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(&fakeMME{})

	target := models.FTEID{TEID: 0x5150, Addr: netip.MustParseAddr("10.0.0.77")}
	sc := epsSessionForwardingTo(t, s, target)

	if sc.ForwardingTEIDForTest() == 0 {
		t.Fatal("no forwarding tunnel to release")
	}

	if err := s.HandleErrorIndicationReport(context.Background(), forwardingReport(sc.PFCPContext.SEID, target)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if sc.ForwardingTEIDForTest() != 0 {
		t.Error("the forwarding tunnel still relays into a target that discarded it")
	}

	if sc.Tunnel.Downlink != smf.DownlinkForwarding {
		t.Errorf("releasing the forwarding tunnel disturbed the downlink: state = %v", sc.Tunnel.Downlink)
	}
}

// A report naming a forwarding endpoint the session has moved off must not tear
// down the tunnel that replaced it.
func TestErrorIndicationForASupersededForwardingTunnelIsIgnored(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	upf.forwardingTEID = 4242

	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(&fakeMME{})

	target := models.FTEID{TEID: 0x5150, Addr: netip.MustParseAddr("10.0.0.77")}
	sc := epsSessionForwardingTo(t, s, target)

	stale := models.FTEID{TEID: target.TEID + 1, Addr: target.Addr}

	if err := s.HandleErrorIndicationReport(context.Background(), forwardingReport(sc.PFCPContext.SEID, stale)); err != nil {
		t.Fatalf("HandleErrorIndicationReport: %v", err)
	}

	if sc.ForwardingTEIDForTest() == 0 {
		t.Error("a superseded Error Indication released the current forwarding tunnel")
	}
}
