// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
)

func testGNBID(value uint32) ngap.GlobalRANNodeID {
	return ngap.GlobalRANNodeID{
		Kind:         ngap.RANNodeIDGNB,
		PLMNIdentity: ngap.PLMNIdentity{0x00, 0xf1, 0x10},
		Value:        value,
		Bits:         24,
	}
}

func newIdentityTestRadio(t *testing.T, a *AMF, name string) (*Radio, *sctp.SCTPConn) {
	t.Helper()

	conn := &sctp.SCTPConn{}
	radio := &Radio{Conn: conn, amf: a}
	a.UpdateRadioName(radio, name)
	a.SetRadioForTest(conn, radio)

	return radio, conn
}

func TestRebindRanIDMovesConnectedUEsAndLastSeenToTheNewRef(t *testing.T) {
	a := New(nil, nil, nil)
	radio, _ := newIdentityTestRadio(t, a, "gNB-A")

	if _, err := a.ClaimRanID(t.Context(), radio, testGNBID(1), DefaultRelativeCapacity); err != nil {
		t.Fatalf("ClaimRanID: %v", err)
	}

	oldRef := radioIDOf(radio)
	ue := NewUeConnForTest(radio, 1, 1)

	const imsi = "001010000000001"

	a.lastSeen.record(imsi, oldRef, "gNB-A", time.Now())

	rebound, err := a.RebindRanID(radio, testGNBID(2))
	if err != nil || !rebound {
		t.Fatalf("RebindRanID = %v, %v; want true", rebound, err)
	}

	newRef := radioIDOf(radio)
	if newRef == oldRef {
		t.Fatalf("ref did not change on rebind: %q", newRef)
	}

	if id, name := ue.radioIDName(); id != newRef || name != "gNB-A" {
		t.Errorf("UE radio = %q/%q, want %q/gNB-A", id, name, newRef)
	}

	if seen, _ := a.LastSeen(imsi); seen.RadioID != newRef || seen.RadioName != "gNB-A" {
		t.Errorf("last seen = %q/%q, want %q/gNB-A", seen.RadioID, seen.RadioName, newRef)
	}
}

func TestRenamingARadioRenamesItOnConnectedUEs(t *testing.T) {
	a := New(nil, nil, nil)
	radio, _ := newIdentityTestRadio(t, a, "gNB-A")

	if _, err := a.ClaimRanID(t.Context(), radio, testGNBID(1), DefaultRelativeCapacity); err != nil {
		t.Fatalf("ClaimRanID: %v", err)
	}

	ue := NewUeConnForTest(radio, 1, 1)

	a.UpdateRadioName(radio, "gNB-B")

	if id, name := ue.radioIDName(); id != radioIDOf(radio) || name != "gNB-B" {
		t.Errorf("UE radio = %q/%q, want %q/gNB-B", id, name, radioIDOf(radio))
	}
}

func TestNGSetupOutcomeCarriesTheRefOfItsRequest(t *testing.T) {
	a := New(nil, nil, nil)
	_, conn := newIdentityTestRadio(t, a, "")

	const ref = "gNB:001-01:000001@24"

	a.setSetupRef(conn, ref)

	for _, procedure := range []NGAPProcedure{NGAPProcedureNGSetupFailure, NGAPProcedureNGSetupResponse} {
		if _, got := a.radioIdentityByConn(conn, procedure); got != ref {
			t.Errorf("%s ref = %q, want %q", procedure, got, ref)
		}
	}

	if _, got := a.radioIdentityByConn(conn, NGAPProcedureErrorIndication); got != "" {
		t.Errorf("ErrorIndication ref = %q, want empty: the radio has not claimed an identity", got)
	}
}
