// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

func TestReallocateGUTI(t *testing.T) {
	m := newTestMME(t)
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}

	ue := m.NewUe(&captureConn{}, 7)
	guti := m.ReallocateGUTI(ue, plmn, 0x1234, 0x56)

	if guti.Type != eps.IdentityGUTI || guti.MCC != "001" || guti.MNC != "01" ||
		guti.MMEGroupID != 0x1234 || guti.MMECode != 0x56 {
		t.Fatalf("unexpected GUTI: %+v", guti)
	}

	if ue.Tmsi().Uint32() != guti.MTMSI {
		t.Fatalf("UE M-TMSI = %d, GUTI M-TMSI = %d", ue.Tmsi().Uint32(), guti.MTMSI)
	}

	got, ok := m.LookupUeByMTMSI(guti.MTMSI)
	if !ok || got != ue {
		t.Fatal("UE not indexed by its M-TMSI")
	}

	// A second UE gets a distinct M-TMSI.
	ue2 := m.NewUe(&captureConn{}, 8)
	if guti2 := m.ReallocateGUTI(ue2, plmn, 0x1234, 0x56); guti2.MTMSI == guti.MTMSI {
		t.Fatalf("M-TMSI not unique: both %d", guti2.MTMSI)
	}

	// Releasing the UE clears the index.
	m.RemoveUe(ue)

	if _, ok := m.LookupUeByMTMSI(guti.MTMSI); ok {
		t.Fatal("M-TMSI index not cleared on UE removal")
	}
}

// TestReallocateGUTITwoPhase checks the two-phase GUTI reallocation used by both
// attach and TAU: reallocating over an existing M-TMSI stages the new one while
// the old stays resolvable (TS 24.301 §5.5.1.2.7, §5.5.3.2.4 — the old GUTI is
// valid until completion), and CommitGUTIRealloc frees the old only on the UE's
// acknowledgement.
func TestReallocateGUTITwoPhase(t *testing.T) {
	m := newTestMME(t)
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}
	ue := m.NewUe(&captureConn{}, 7)

	first := m.ReallocateGUTI(ue, plmn, 1, 1).MTMSI
	m.CommitGUTIRealloc(ue)

	second := m.ReallocateGUTI(ue, plmn, 1, 1).MTMSI

	if first == second {
		t.Fatal("reallocation reused the same M-TMSI")
	}

	// Both M-TMSIs resolve to the UE until the reallocation is committed.
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
