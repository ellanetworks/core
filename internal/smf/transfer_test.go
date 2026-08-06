// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

// transferTestPDUSessionID is the identity buildPDUSessionEstRequest names.
const transferTestPDUSessionID uint8 = 1

func epsTransferRequest(t *testing.T, requestType eps.RequestType) models.EPSBearerRequest {
	t.Helper()

	req := epsRequest(1)
	req.PDUSessionID = transferTestPDUSessionID
	req.RequestType = requestType
	req.Snssai = &models.Snssai{Sst: 1, Sd: "010203"}

	return req
}

func rejectCause(t *testing.T, raw []byte) fgs.GSMCause {
	t.Helper()

	msg, err := fgs.ParseMessage(raw)
	if err != nil {
		t.Fatalf("parse reject: %v", err)
	}

	reject, ok := msg.(*fgs.PDUSessionEstablishmentReject)
	if !ok {
		t.Fatalf("reject is %T, want *fgs.PDUSessionEstablishmentReject", msg)
	}

	return reject.Cause
}

// TS 23.502 §4.11.2.2 step 13.
func TestTransfer5GSToEPSKeepsSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil || rejectN1 != nil {
		t.Fatalf("CreateSmContext: %v (reject %d bytes)", err, len(rejectN1))
	}

	sc := s.GetSession(ref)
	if sc == nil {
		t.Fatal("5G session is not in the pool")
	}

	sc.Mutex.Lock()
	ip := sc.PDUIPV4Address.String()
	seid := sc.PFCPContext.LocalSEID
	ulTEID := sc.Tunnel.DataPath.UpLinkTunnel.TEID
	sc.Mutex.Unlock()

	upf.mu.Lock()
	establishesBefore := upf.lastEstablish
	upf.mu.Unlock()

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover))
	if err != nil {
		t.Fatalf("CreateEPSSession(handover): %v", err)
	}

	if bearer.Ref != ref {
		t.Errorf("bearer ref = %q, want the 5G session's %q", bearer.Ref, ref)
	}

	if bearer.IPv4.String() != ip {
		t.Errorf("UE address = %s, want the one it held on 5GS, %s", bearer.IPv4, ip)
	}

	if bearer.SGW.TEID != ulTEID {
		t.Errorf("S-GW S1-U TEID = %#x, want the anchor's uplink TEID %#x", bearer.SGW.TEID, ulTEID)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if !sc.IsEPS() {
		t.Error("session is still on 5GS after the transfer")
	}

	if sc.EBI != epsTestEBI || sc.PDUSessionID != transferTestPDUSessionID {
		t.Errorf("identity = ebi %d pdu-session-id %d, want %d and %d", sc.EBI, sc.PDUSessionID, epsTestEBI, transferTestPDUSessionID)
	}

	if sc.PFCPContext.LocalSEID != seid {
		t.Errorf("SEID = %d, want it preserved at %d", sc.PFCPContext.LocalSEID, seid)
	}

	if sc.Tunnel.DataPath.UpLinkTunnel.TEID != ulTEID {
		t.Errorf("uplink TEID = %#x, want it preserved at %#x", sc.Tunnel.DataPath.UpLinkTunnel.TEID, ulTEID)
	}

	if sc.PolicyData == nil || sc.PolicyData.Ambr.Uplink.Bps() == 0 {
		t.Error("transferred session has no policy from the target access")
	}

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the 5GS establishment", ids)
	}

	if moved := amfCb.movedAway(); len(moved) != 1 || moved[0] != transferTestPDUSessionID {
		t.Errorf("AMF told of moved sessions = %v, want [%d]", moved, transferTestPDUSessionID)
	}

	// TS 23.502 §4.11.2.2 step 14.
	amfCb.mu.Lock()
	releases := amfCb.transferReleases
	amfCb.mu.Unlock()

	if releases != 1 {
		t.Errorf("N2 releases for the moved session = %d, want 1", releases)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish != establishesBefore {
		t.Error("the transfer established a second UPF session")
	}
}

// TS 23.502 §4.11.2.3 step 9.
func TestTransferEPSTo5GSKeepsSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	bearer, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeInitialRequest))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	sc := s.GetSession(bearer.Ref)
	if sc == nil {
		t.Fatal("EPS session is not in the pool")
	}

	sc.Mutex.Lock()
	ip := sc.PDUIPV4Address.String()
	seid := sc.PFCPContext.LocalSEID
	ulTEID := sc.Tunnel.DataPath.UpLinkTunnel.TEID
	sc.Mutex.Unlock()

	upf.mu.Lock()
	establishesBefore := upf.lastEstablish
	upf.mu.Unlock()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext(existing PDU session): %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("transfer rejected with 5GSM cause %s", rejectCause(t, rejectN1))
	}

	if ref != bearer.Ref {
		t.Errorf("SM context ref = %q, want the EPS session's %q", ref, bearer.Ref)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.IsEPS() {
		t.Error("session is still on EPS after the transfer")
	}

	if sc.EBI != 0 {
		t.Errorf("EBI = %d, want it given up with the PDN connection", sc.EBI)
	}

	if sc.PDUSessionID != transferTestPDUSessionID {
		t.Errorf("PDU session id = %d, want %d", sc.PDUSessionID, transferTestPDUSessionID)
	}

	if sc.PDUIPV4Address.String() != ip {
		t.Errorf("UE address = %s, want the one it held on EPS, %s", sc.PDUIPV4Address, ip)
	}

	if sc.PFCPContext.LocalSEID != seid || sc.Tunnel.DataPath.UpLinkTunnel.TEID != ulTEID {
		t.Errorf("SEID/uplink TEID = %d/%#x, want them preserved at %d/%#x",
			sc.PFCPContext.LocalSEID, sc.Tunnel.DataPath.UpLinkTunnel.TEID, seid, ulTEID)
	}

	if sc.PolicyData == nil || sc.PolicyData.QosData.QFI == 0 {
		t.Error("transferred session has no 5GS QoS flow identifier")
	}

	if ids := store.allocSessionIDs(); len(ids) != 1 {
		t.Errorf("lease allocations = %v, want the one from the EPS establishment", ids)
	}

	if moved := mmeCb.movedAway(); len(moved) != 1 || moved[0] != epsTestEBI {
		t.Errorf("MME told of moved connections = %v, want [%d]", moved, epsTestEBI)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if upf.lastEstablish != establishesBefore {
		t.Error("the transfer established a second UPF session")
	}
}

// ESM #54 (TS 24.301 §6.5.1.4 b) and 5GSM #54 (TS 24.501 §6.4.1.4 d).
func TestTransferOfUnknownSessionIsRefused(t *testing.T) {
	t.Run("to EPS", func(t *testing.T) {
		pcf, store, upf, amfCb := defaultFakes()
		s := newTestSMF(pcf, store, upf, amfCb)

		bearer, err := s.CreateEPSSession(context.Background(), epsTransferRequest(t, eps.RequestTypeHandover))
		if err == nil {
			t.Fatal("CreateEPSSession(handover) with no session to transfer = nil error")
		}

		if bearer.ESMCause != eps.ESMCausePDNConnectionDoesNotExist {
			t.Errorf("ESM cause = %s, want #54 PDN connection does not exist", bearer.ESMCause)
		}

		if ids := store.allocSessionIDs(); len(ids) != 0 {
			t.Errorf("lease allocations = %v, want none for a refused transfer", ids)
		}
	})

	t.Run("to 5GS", func(t *testing.T) {
		pcf, store, upf, amfCb := defaultFakes()
		s := newTestSMF(pcf, store, upf, amfCb)

		ref, rejectN1, err := s.CreateSmContext(context.Background(), testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeExistingPDUSession, buildPDUSessionEstRequest())
		if err == nil {
			t.Fatal("CreateSmContext(existing PDU session) with no session to transfer = nil error")
		}

		if ref != "" {
			t.Errorf("SM context ref = %q, want none", ref)
		}

		if cause := rejectCause(t, rejectN1); cause != fgs.GSMCausePDUSessionDoesNotExist {
			t.Errorf("5GSM cause = %s, want #54 PDU session does not exist", cause)
		}

		if ids := store.allocSessionIDs(); len(ids) != 0 {
			t.Errorf("lease allocations = %v, want none for a refused transfer", ids)
		}
	})
}

func TestTransferRefusedOnDataNetworkMismatch(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	req := epsTransferRequest(t, eps.RequestTypeHandover)
	req.APN = "ims"

	bearer, err := s.CreateEPSSession(ctx, req)
	if err == nil {
		t.Fatal("CreateEPSSession(handover) for another data network = nil error")
	}

	if bearer.ESMCause != eps.ESMCausePDNConnectionDoesNotExist {
		t.Errorf("ESM cause = %s, want #54 PDN connection does not exist", bearer.ESMCause)
	}

	if !errors.Is(err, smf.ErrSessionNotTransferable) {
		t.Errorf("error = %v, want it to wrap ErrSessionNotTransferable", err)
	}
}

func TestConcurrentTransfersDoNotBothCommit(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), transferTestPDUSessionID, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)

	for range 2 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if _, err := s.CreateEPSSession(ctx, epsTransferRequest(t, eps.RequestTypeHandover)); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if succeeded != 1 {
		t.Errorf("%d of 2 racing transfers committed, want exactly 1", succeeded)
	}

	if moved := amfCb.movedAway(); len(moved) != 1 {
		t.Errorf("AMF told of moved sessions = %v, want exactly one", moved)
	}

	if fourG, _ := s.SessionCountByRAT(); fourG != 1 {
		t.Errorf("EPS session count = %d, want the one transferred session", fourG)
	}
}
