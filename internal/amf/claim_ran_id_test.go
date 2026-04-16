// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func gnbGlobalRANNodeID(t *testing.T, hexID string) *ngapType.GlobalRANNodeID {
	t.Helper()

	id := &ngapType.GlobalRANNodeID{}
	id.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
	id.GlobalGNBID = &ngapType.GlobalGNBID{}
	id.GlobalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	id.GlobalGNBID.GNBID.GNBID = new(aper.BitString)
	*id.GlobalGNBID.GNBID.GNBID = ngapConvert.HexToBitString(hexID, 24)

	return id
}

func newRadioForTest(conn *sctp.SCTPConn, name string) *amf.Radio {
	return &amf.Radio{
		Name:   name,
		Conn:   conn,
		RanUEs: make(map[int64]*amf.RanUe),
		Log:    zap.NewNop(),
	}
}

func TestClaimRanID_NoExistingRadio(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	conn := &sctp.SCTPConn{}

	radio := newRadioForTest(conn, "gNB-A")
	amfInstance.Radios[conn] = radio

	evicted := amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1"))
	if evicted != nil {
		t.Fatalf("expected no eviction, got radio %q", evicted.Name)
	}

	if radio.RanID == nil || radio.RanID.GNbID == nil {
		t.Fatal("expected radio.RanID and RanID.GNbID to be populated")
	}

	if radio.RanPresent != amf.RanPresentGNbID {
		t.Errorf("expected RanPresent=%d, got %d", amf.RanPresentGNbID, radio.RanPresent)
	}

	if len(amfInstance.Radios) != 1 {
		t.Errorf("expected 1 radio in pool, got %d", len(amfInstance.Radios))
	}
}

func TestClaimRanID_EvictsDuplicateGNB(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	existingConn := &sctp.SCTPConn{}
	existing := newRadioForTest(existingConn, "gNB-old")
	amfInstance.Radios[existingConn] = existing

	if evicted := amfInstance.ClaimRanID(existing, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Fatalf("setup: unexpected eviction of %q", evicted.Name)
	}

	newConn := &sctp.SCTPConn{}
	newRadio := newRadioForTest(newConn, "gNB-new")
	amfInstance.Radios[newConn] = newRadio

	evicted := amfInstance.ClaimRanID(newRadio, gnbGlobalRANNodeID(t, "ABCDE1"))
	if evicted == nil {
		t.Fatal("expected existing radio to be evicted")
	}

	if evicted != existing {
		t.Errorf("expected evicted radio to be the existing one (%q), got %q", existing.Name, evicted.Name)
	}

	if _, still := amfInstance.Radios[existingConn]; still {
		t.Error("evicted radio should have been removed from Radios map")
	}

	if got, ok := amfInstance.Radios[newConn]; !ok || got != newRadio {
		t.Error("new radio should remain in Radios map")
	}

	if newRadio.RanID == nil || newRadio.RanID.GNbID == nil || newRadio.RanID.GNbID.GNBValue == "" {
		t.Error("new radio should have RanID set to the claimed value")
	}
}

func TestClaimRanID_DifferentIDDoesNotEvict(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	existingConn := &sctp.SCTPConn{}
	existing := newRadioForTest(existingConn, "gNB-old")
	amfInstance.Radios[existingConn] = existing

	if evicted := amfInstance.ClaimRanID(existing, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Fatalf("setup: unexpected eviction of %q", evicted.Name)
	}

	newConn := &sctp.SCTPConn{}
	newRadio := newRadioForTest(newConn, "gNB-new")
	amfInstance.Radios[newConn] = newRadio

	evicted := amfInstance.ClaimRanID(newRadio, gnbGlobalRANNodeID(t, "FEDCBA"))
	if evicted != nil {
		t.Fatalf("expected no eviction for a different Global RAN Node ID, got %q", evicted.Name)
	}

	if len(amfInstance.Radios) != 2 {
		t.Errorf("expected both radios to remain in pool, got %d", len(amfInstance.Radios))
	}
}

func TestClaimRanID_SelfClaimIsNoOp(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	conn := &sctp.SCTPConn{}
	radio := newRadioForTest(conn, "gNB-A")
	amfInstance.Radios[conn] = radio

	if evicted := amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Fatalf("first claim should not evict, got %q", evicted.Name)
	}

	evicted := amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1"))
	if evicted != nil {
		t.Fatalf("self-claim should be a no-op, got eviction of %q", evicted.Name)
	}

	if _, still := amfInstance.Radios[conn]; !still {
		t.Error("radio should remain in Radios map after self-claim")
	}
}
