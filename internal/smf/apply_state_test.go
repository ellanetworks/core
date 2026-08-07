// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func farByID(state *models.SessionState, id uint32) *models.FAR {
	for i := range state.FARs {
		if state.FARs[i].FARID == id {
			return &state.FARs[i]
		}
	}

	return nil
}

func pdrByID(state *models.SessionState, id uint16) *models.PDR {
	for i := range state.PDRs {
		if state.PDRs[i].PDRID == id {
			return &state.PDRs[i]
		}
	}

	return nil
}

// Every statement to the UPF names the session's whole user plane, whatever
// prompted it. A statement that named only what changed would leave the rest to
// an "unchanged" reading the UPF has no way to check.
func TestDeactivationStatesTheWholeUserPlane(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	if err := s.DeactivateSmContext(context.Background(), ref); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	applies := upf.applies()
	if len(applies) != 1 {
		t.Fatalf("UPF session statements = %d, want 1", len(applies))
	}

	state := applies[0]

	for _, id := range []uint16{1, 2} {
		if pdrByID(state, id) == nil {
			t.Errorf("the deactivation statement names no PDR %d", id)
		}
	}

	for _, id := range []uint32{1, 2} {
		if farByID(state, id) == nil {
			t.Errorf("the deactivation statement names no FAR %d", id)
		}
	}

	if defaultQER(state) == nil {
		t.Error("the deactivation statement names no QER 1")
	}

	if state.SEID == 0 || state.IMSI != testIMSI {
		t.Errorf("statement SEID/IMSI = %d/%q, want a live SEID and %q", state.SEID, state.IMSI, testIMSI)
	}
}

// A UE going idle withdraws the downlink tunnel endpoint. The statement carries
// the forwarding rule in full, so the withdrawal is the absence of an outer
// header creation and cannot be read as "keep the one you have" — which would
// leave the datapath forwarding to an access the session has left.
func TestDeactivationWithdrawsTheDownlinkEndpoint(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	if err := s.DeactivateSmContext(context.Background(), ref); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	state := upf.applies()[0]

	dl := farByID(state, 2)
	if dl == nil {
		t.Fatal("the deactivation statement names no downlink FAR")
	}

	if dl.ApplyAction.Forw || !dl.ApplyAction.Buff || !dl.ApplyAction.Nocp {
		t.Errorf("downlink apply action = %+v, want buffer with notification", dl.ApplyAction)
	}

	if dl.ForwardingParameters != nil && dl.ForwardingParameters.OuterHeaderCreation != nil {
		t.Errorf("downlink FAR still names a tunnel endpoint: %+v", dl.ForwardingParameters.OuterHeaderCreation)
	}
}

// A dual-stack session states both downlink PDRs on every statement, so the
// IPv6 downlink converges with the rest rather than resting on what an earlier
// statement left behind.
func TestDualStackStatementNamesBothDownlinks(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(3))
	if err != nil {
		t.Fatal(err)
	}

	if err := s.ModifyEPSSession(context.Background(), bearer.Ref,
		models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}); err != nil {
		t.Fatal(err)
	}

	applies := upf.applies()
	if len(applies) != 2 {
		t.Fatalf("UPF session statements = %d, want 2 (the establishment and the bind)", len(applies))
	}

	for i, state := range applies {
		if len(state.PDRs) != 3 {
			t.Errorf("statement %d names %d PDRs, want 3 (uplink and both downlinks)", i, len(state.PDRs))
		}

		if len(state.URRs) != 2 {
			t.Errorf("statement %d names %d URRs, want 2", i, len(state.URRs))
		}
	}
}
