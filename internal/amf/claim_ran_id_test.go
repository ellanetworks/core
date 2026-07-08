// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/sctp"
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

func newRadioForTest(a *amf.AMF, conn *sctp.SCTPConn, name string) *amf.Radio {
	ran := &amf.Radio{
		Conn: conn,
		Log:  zap.NewNop(),
	}
	ran.BindAMFForTest(a)
	a.UpdateRadioName(ran, name)

	return ran
}

func TestClaimRanID_NoExistingRadio(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	conn := &sctp.SCTPConn{}

	radio := newRadioForTest(amfInstance, conn, "gNB-A")
	amfInstance.SetRadioForTest(conn, radio)

	evicted := amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1"))
	if evicted != nil {
		t.Fatalf("expected no eviction, got radio %q", amfInstance.RadioNameForTest(evicted))
	}

	if radio.RanID == nil || radio.RanID.GNbID == nil {
		t.Fatal("expected radio.RanID and RanID.GNbID to be populated")
	}

	if radio.RanPresent != amf.RanPresentGNbID {
		t.Errorf("expected RanPresent=%d, got %d", amf.RanPresentGNbID, radio.RanPresent)
	}

	if amfInstance.CountRadios() != 1 {
		t.Errorf("expected 1 radio in pool, got %d", amfInstance.CountRadios())
	}
}

func TestClaimRanID_EvictsDuplicateGNB(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	existingConn := &sctp.SCTPConn{}
	existing := newRadioForTest(amfInstance, existingConn, "gNB-old")
	amfInstance.SetRadioForTest(existingConn, existing)

	if evicted := amfInstance.ClaimRanID(existing, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Fatalf("setup: unexpected eviction of %q", amfInstance.RadioNameForTest(evicted))
	}

	newConn := &sctp.SCTPConn{}
	newRadio := newRadioForTest(amfInstance, newConn, "gNB-new")
	amfInstance.SetRadioForTest(newConn, newRadio)

	evicted := amfInstance.ClaimRanID(newRadio, gnbGlobalRANNodeID(t, "ABCDE1"))
	if evicted == nil {
		t.Fatal("expected existing radio to be evicted")
	}

	if evicted != existing {
		t.Errorf("expected evicted radio to be the existing one (%q), got %q", amfInstance.RadioNameForTest(existing), amfInstance.RadioNameForTest(evicted))
	}

	if _, still := amfInstance.RadioForTest(existingConn); still {
		t.Error("evicted radio should have been removed from radios map")
	}

	if got, ok := amfInstance.RadioForTest(newConn); !ok || got != newRadio {
		t.Error("new radio should remain in radios map")
	}

	if newRadio.RanID == nil || newRadio.RanID.GNbID == nil || newRadio.RanID.GNbID.GNBValue == "" {
		t.Error("new radio should have RanID set to the claimed value")
	}
}

func TestClaimRanID_DifferentIDDoesNotEvict(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	existingConn := &sctp.SCTPConn{}
	existing := newRadioForTest(amfInstance, existingConn, "gNB-old")
	amfInstance.SetRadioForTest(existingConn, existing)

	if evicted := amfInstance.ClaimRanID(existing, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Fatalf("setup: unexpected eviction of %q", amfInstance.RadioNameForTest(evicted))
	}

	newConn := &sctp.SCTPConn{}
	newRadio := newRadioForTest(amfInstance, newConn, "gNB-new")
	amfInstance.SetRadioForTest(newConn, newRadio)

	evicted := amfInstance.ClaimRanID(newRadio, gnbGlobalRANNodeID(t, "FEDCBA"))
	if evicted != nil {
		t.Fatalf("expected no eviction for a different Global RAN Node ID, got %q", amfInstance.RadioNameForTest(evicted))
	}

	if amfInstance.CountRadios() != 2 {
		t.Errorf("expected both radios to remain in pool, got %d", amfInstance.CountRadios())
	}
}

func TestClaimRanID_SelfClaimIsNoOp(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)

	conn := &sctp.SCTPConn{}
	radio := newRadioForTest(amfInstance, conn, "gNB-A")
	amfInstance.SetRadioForTest(conn, radio)

	if evicted := amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1")); evicted != nil {
		t.Fatalf("first claim should not evict, got %q", amfInstance.RadioNameForTest(evicted))
	}

	evicted := amfInstance.ClaimRanID(radio, gnbGlobalRANNodeID(t, "ABCDE1"))
	if evicted != nil {
		t.Fatalf("self-claim should be a no-op, got eviction of %q", amfInstance.RadioNameForTest(evicted))
	}

	if _, still := amfInstance.RadioForTest(conn); !still {
		t.Error("radio should remain in radios map after self-claim")
	}
}
