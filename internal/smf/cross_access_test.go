// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestModifyEPSSessionRejectsSupersededRef(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	superseded, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	live, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession (re-establish): %v", err)
	}

	if superseded.Ref == live.Ref {
		t.Fatalf("two sessions for EBI %d share ref %q", epsTestEBI, live.Ref)
	}

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}

	if err := s.ModifyEPSSession(ctx, superseded.Ref, enb); err == nil {
		t.Error("ModifyEPSSession(superseded ref) = nil, want error")
	}

	sc := s.GetSession(live.Ref)
	if sc == nil {
		t.Fatal("live EPS session is not in the pool")
	}

	sc.Mutex.Lock()
	an := sc.Tunnel.ANInformation
	sc.Mutex.Unlock()

	if an.TEID == enb.TEID && net.IP(enb.Addr.AsSlice()).Equal(an.IPv4Address) {
		t.Errorf("live session's AN endpoint = eNB S1-U %v/0x%x, want it unchanged", an.IPv4Address, an.TEID)
	}

	upf.mu.Lock()
	modifiesAfter := len(upf.modifyCalls)
	upf.mu.Unlock()

	if modifiesAfter != modifiesBefore {
		t.Errorf("UPF ModifySession calls = %d, want %d", modifiesAfter, modifiesBefore)
	}
}

func TestCreateEPSSessionKeeps5GSessionWithSameID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref5g, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("CreateSmContext rejected the establishment: %d-byte N1 reject", len(rejectN1))
	}

	bearer, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if s.GetSession(ref5g) == nil {
		t.Error("5G session with PDU session id 5 released by an EPS create for EBI 5")
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("EPS session for EBI 5 is not live")
	}

	if bearer.Ref == ref5g {
		t.Errorf("EPS create returned the 5G session's ref %q", ref5g)
	}
}

func TestCreateSmContextKeepsEPSSessionWithSameID(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	bearer, err := s.CreateEPSSession(ctx, epsRequest(1))
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	ref5g, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("CreateSmContext rejected the establishment: %d-byte N1 reject", len(rejectN1))
	}

	if s.GetSession(bearer.Ref) == nil {
		t.Error("EPS session for EBI 5 released by a 5G establishment for PDU session id 5")
	}

	if s.GetSession(ref5g) == nil {
		t.Error("5G session with PDU session id 5 is not live")
	}
}

// Leases must be keyed in the converged id space too, so coexisting sessions
// with the same wire id cannot share an address (TS 29.571 core-allocated
// range keeps them distinct).
func TestLeaseKeysDistinctAcrossAccesses(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	if _, _, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, fgs.RequestTypeInitialRequest, buildPDUSessionEstRequest()); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if _, err := s.CreateEPSSession(ctx, epsRequest(1)); err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	ids := store.allocSessionIDs()
	if len(ids) != 2 {
		t.Fatalf("lease allocations = %d, want 2", len(ids))
	}

	if ids[0] == ids[1] {
		t.Errorf("both accesses allocated under lease session id %d, want distinct ids", ids[0])
	}
}
