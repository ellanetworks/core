// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

// A default-bearer EBI (5..15, TS 24.007) and a 5G PDU session ID (1..15) share
// the SMF's (SUPI, id) keyspace, so an EPS operation can name the id of a live
// 5G PDU session left by the same subscriber.
func TestModifyEPSSessionRejects5GPDUSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	ref, rejectN1, err := s.CreateSmContext(ctx, testSUPI(), 5, testDNN, testSnssai, buildPDUSessionEstRequest())
	if err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	if rejectN1 != nil {
		t.Fatalf("CreateSmContext rejected the establishment: %d-byte N1 reject", len(rejectN1))
	}

	upf.mu.Lock()
	modifiesBefore := len(upf.modifyCalls)
	upf.mu.Unlock()

	enb := models.FTEID{TEID: 0x6001, Addr: enbS1UAddr}

	if err := s.ModifyEPSSession(ctx, testIMSI, 5, enb); err == nil {
		t.Error("ModifyEPSSession(ebi=5) = nil, want error: the only session with id 5 was established over 5G")
	}

	sc := s.GetSession(ref)
	if sc == nil {
		t.Fatal("5G session with PDU session id 5 is no longer in the pool")
	}

	sc.Mutex.Lock()
	an := sc.Tunnel.ANInformation
	ohc := sc.Tunnel.DataPath.DownLinkTunnel.PDR.FAR.ForwardingParameters
	sc.Mutex.Unlock()

	if an.TEID == enb.TEID && net.IP(enb.Addr.AsSlice()).Equal(an.IPv4Address) {
		t.Errorf("5G session's AN endpoint = eNB S1-U %v/0x%x, want it unchanged", an.IPv4Address, an.TEID)
	}

	if ohc != nil && ohc.OuterHeaderCreation != nil && ohc.OuterHeaderCreation.S1U {
		t.Error("5G session's downlink outer header S1U flag = true, want PSC-bearing N3 encapsulation")
	}

	upf.mu.Lock()
	modifiesAfter := len(upf.modifyCalls)
	upf.mu.Unlock()

	if modifiesAfter != modifiesBefore {
		t.Errorf("UPF ModifySession calls = %d, want %d: an EPS modify must not reprogram a 5G session's data path", modifiesAfter, modifiesBefore)
	}
}

var enbS1UAddr = netip.MustParseAddr("192.168.40.10")
