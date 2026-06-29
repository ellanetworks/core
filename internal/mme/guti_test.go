// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

func TestAssignGUTI(t *testing.T) {
	m := newTestMME(t)
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}

	ue := m.NewUe(&captureConn{}, 7)
	guti := m.AssignGUTI(ue, plmn, 0x1234, 0x56)

	if guti.Type != eps.IdentityGUTI || guti.MCC != "001" || guti.MNC != "01" ||
		guti.MMEGroupID != 0x1234 || guti.MMECode != 0x56 {
		t.Fatalf("unexpected GUTI: %+v", guti)
	}

	if ue.mtmsi != guti.MTMSI {
		t.Fatalf("UE M-TMSI = %d, GUTI M-TMSI = %d", ue.mtmsi, guti.MTMSI)
	}

	got, ok := m.LookupUeByMTMSI(guti.MTMSI)
	if !ok || got != ue {
		t.Fatal("UE not indexed by its M-TMSI")
	}

	// A second UE gets a distinct M-TMSI.
	ue2 := m.NewUe(&captureConn{}, 8)
	if guti2 := m.AssignGUTI(ue2, plmn, 0x1234, 0x56); guti2.MTMSI == guti.MTMSI {
		t.Fatalf("M-TMSI not unique: both %d", guti2.MTMSI)
	}

	// Releasing the UE clears the index.
	m.RemoveUe(ue)

	if _, ok := m.LookupUeByMTMSI(guti.MTMSI); ok {
		t.Fatal("M-TMSI index not cleared on UE removal")
	}
}

// TestAssignGUTIReallocationFreesOld checks that reassigning a GUTI to a UE that
// already holds one — the MME reallocates on every IMSI attach (TS 24.301
// §5.5.1.2.4) — drops the previous M-TMSI from the index so a stale S-TMSI no
// longer resolves to the UE.
func TestAssignGUTIReallocationFreesOld(t *testing.T) {
	m := newTestMME(t)
	plmn := models.PlmnID{Mcc: "001", Mnc: "01"}
	ue := m.NewUe(&captureConn{}, 7)

	first := m.AssignGUTI(ue, plmn, 1, 1).MTMSI
	second := m.AssignGUTI(ue, plmn, 1, 1).MTMSI

	if first == second {
		t.Fatal("reallocation reused the same M-TMSI")
	}

	if _, ok := m.LookupUeByMTMSI(first); ok {
		t.Fatal("previous M-TMSI still indexed after reallocation")
	}

	if got, ok := m.LookupUeByMTMSI(second); !ok || got != ue {
		t.Fatal("UE not indexed by its new M-TMSI")
	}
}
