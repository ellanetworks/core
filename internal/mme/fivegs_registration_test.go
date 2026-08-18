// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

func supersedeUE(t *testing.T) (*MME, *UeContext, *fakeFiveGSPeer) {
	t.Helper()

	m := newTestMME(t)
	peer := &fakeFiveGSPeer{}
	m.FiveGS = peer

	ue := NewUeContext()
	m.SetIMSI(ue, "001010000000001")

	return m, ue, peer
}

// TS 23.502 §4.11.1.5.2 step 2
func TestSupersedeFiveGSRegistrationTellsFiveGSTheUEAttachedHere(t *testing.T) {
	m, ue, peer := supersedeUE(t)

	m.SupersedeFiveGSRegistration(context.Background(), ue)

	if len(peer.cancelled) != 1 || peer.cancelled[0] != "001010000000001" {
		t.Errorf("cancelled %v, want [001010000000001]: 5GS was never told the UE attached in EPS, so the subscriber reports on both 4G and 5G", peer.cancelled)
	}
}

func TestSupersedeFiveGSRegistrationDefersToAHandoverToFiveGS(t *testing.T) {
	m, ue, peer := supersedeUE(t)

	if _, ok := m.stageRelocationToFiveGS(ue, nil, nil); !ok {
		t.Fatal("stageRelocationToFiveGS refused a fresh handover")
	}

	m.SupersedeFiveGSRegistration(context.Background(), ue)

	if len(peer.cancelled) != 0 {
		t.Errorf("cancelled %v during a handover to 5GS: the cancel pre-empted the relocation", peer.cancelled)
	}
}

func TestSupersedeFiveGSRegistrationDefersToAnIdleMoveToFiveGS(t *testing.T) {
	m, ue, peer := supersedeUE(t)

	ue.BeginIdleMobilityTo5GS()

	m.SupersedeFiveGSRegistration(context.Background(), ue)

	if len(peer.cancelled) != 0 {
		t.Errorf("cancelled %v during an idle move to 5GS: the cancel pre-empted the context transfer", peer.cancelled)
	}
}

func TestSupersedeFiveGSRegistrationWithoutAFiveGSPeerIsANoOp(t *testing.T) {
	m, ue, _ := supersedeUE(t)
	m.FiveGS = nil

	m.SupersedeFiveGSRegistration(context.Background(), ue)
}

func TestSupersedeFiveGSRegistrationWithoutAnIdentityIsANoOp(t *testing.T) {
	m, _, peer := supersedeUE(t)

	m.SupersedeFiveGSRegistration(context.Background(), NewUeContext())

	if len(peer.cancelled) != 0 {
		t.Errorf("cancelled %v for a UE with no IMSI", peer.cancelled)
	}
}
