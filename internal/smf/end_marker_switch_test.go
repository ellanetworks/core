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

func TestModifyEPSSessionRequestsEndMarkersOnlyWhenSwitching(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(3))
	if err != nil {
		t.Fatal(err)
	}

	source := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	target := models.FTEID{TEID: 0x66, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 4})}

	for _, enb := range []models.FTEID{source, source, target} {
		if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI, enb); err != nil {
			t.Fatal(err)
		}
	}

	if len(upf.modifyCalls) != 3 {
		t.Fatalf("expected 3 ModifySession calls, got %d", len(upf.modifyCalls))
	}

	want := []bool{false, false, true}
	for i, req := range upf.modifyCalls {
		if req.SendEndMarkers != want[i] {
			t.Errorf("modify %d SendEndMarkers = %v, want %v", i, req.SendEndMarkers, want[i])
		}
	}
}

func TestModifyEPSSessionSkipsEndMarkersAfterAccessBearerRelease(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(3))
	if err != nil {
		t.Fatal(err)
	}

	source := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	target := models.FTEID{TEID: 0x66, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 4})}

	if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI, source); err != nil {
		t.Fatal(err)
	}

	if err := s.DeactivateEPSSession(context.Background(), bearer.Ref); err != nil {
		t.Fatal(err)
	}

	if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI, target); err != nil {
		t.Fatal(err)
	}

	if len(upf.modifyCalls) != 3 {
		t.Fatalf("expected 3 ModifySession calls, got %d", len(upf.modifyCalls))
	}

	for i, req := range upf.modifyCalls {
		if req.SendEndMarkers {
			t.Errorf("modify %d asked for End Markers: the UE went idle before it came back on another eNB, which is not a handover path switch", i)
		}
	}
}

func TestNGRANBindSkipsEndMarkersAfterUserPlaneDeactivation(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	ctx := context.Background()

	if err := s.DeactivateSmContext(ctx, ref); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x9002, net.ParseIP("10.0.0.9").To4())
	if err != nil {
		t.Fatalf("build setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 2 {
		t.Fatalf("expected 2 ModifySession calls, got %d", len(upf.modifyCalls))
	}

	for i, req := range upf.modifyCalls {
		if req.SendEndMarkers {
			t.Errorf("modify %d asked for End Markers: the UE went idle before it came back on another gNB, which is not an inter NG-RAN handover path switch (TS 23.501 §5.8.2.9.1)", i)
		}
	}
}
