// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"encoding/binary"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestReallocateGUTI(t *testing.T) {
	m := newTestMME(t)
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}

	ue := m.NewUe(&captureConn{}, 7)

	guti, err := m.ReallocateGUTI(t.Context(), ue, plmn, 0x1234, 0x56)
	if err != nil {
		t.Fatalf("ReallocateGUTI: %v", err)
	}

	if guti.GUTI == nil || guti.GUTI.PLMN.MCC != "001" || guti.GUTI.PLMN.MNC != "01" ||
		guti.GUTI.MMEGroupID != 0x1234 || guti.GUTI.MMECode != 0x56 {
		t.Fatalf("unexpected GUTI: %+v", guti)
	}

	if ue.Tmsi().Uint32() != binary.BigEndian.Uint32(guti.GUTI.TMSI[:]) {
		t.Fatalf("UE M-TMSI = %d, GUTI M-TMSI = %x", ue.Tmsi().Uint32(), guti.GUTI.TMSI)
	}

	got, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:]))
	if !ok || got != ue {
		t.Fatal("UE not indexed by its M-TMSI")
	}

	ue2 := m.NewUe(&captureConn{}, 8)

	guti2, err := m.ReallocateGUTI(t.Context(), ue2, plmn, 0x1234, 0x56)
	if err != nil {
		t.Fatalf("ReallocateGUTI: %v", err)
	}

	if binary.BigEndian.Uint32(guti2.GUTI.TMSI[:]) == binary.BigEndian.Uint32(guti.GUTI.TMSI[:]) {
		t.Fatalf("M-TMSI not unique: both %d", binary.BigEndian.Uint32(guti2.GUTI.TMSI[:]))
	}

	m.RemoveUe(ue)

	if _, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(guti.GUTI.TMSI[:])); ok {
		t.Fatal("M-TMSI index not cleared on UE removal")
	}
}

// TS 24.301 §5.5.1.2.7
func TestReallocateGUTITwoPhase(t *testing.T) {
	m := newTestMME(t)
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}
	ue := m.NewUe(&captureConn{}, 7)

	firstGUTI, err := m.ReallocateGUTI(t.Context(), ue, plmn, 1, 1)
	if err != nil {
		t.Fatalf("ReallocateGUTI: %v", err)
	}

	first := binary.BigEndian.Uint32(firstGUTI.GUTI.TMSI[:])

	m.CommitGUTIRealloc(ue)

	secondGUTI, err := m.ReallocateGUTI(t.Context(), ue, plmn, 1, 1)
	if err != nil {
		t.Fatalf("ReallocateGUTI: %v", err)
	}

	second := binary.BigEndian.Uint32(secondGUTI.GUTI.TMSI[:])

	if first == second {
		t.Fatal("reallocation reused the same M-TMSI")
	}

	if got, ok := m.LookupUeByMTMSI(first); !ok || got != ue {
		t.Fatal("old M-TMSI must stay resolvable until the UE acknowledges")
	}

	if got, ok := m.LookupUeByMTMSI(second); !ok || got != ue {
		t.Fatal("UE not indexed by its new M-TMSI")
	}

	m.CommitGUTIRealloc(ue)

	if _, ok := m.LookupUeByMTMSI(first); ok {
		t.Fatal("old M-TMSI still indexed after commit")
	}

	if got, ok := m.LookupUeByMTMSI(second); !ok || got != ue {
		t.Fatal("UE not indexed by its new M-TMSI after commit")
	}
}
