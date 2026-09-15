// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

func TestErrorIndicationPagesTheUEOnReleaseComplete(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).Apn = "internet"

	group, code, err := m.MmeIdentity(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := m.ReallocateGUTI(context.Background(), ue, models.PlmnID{Mcc: "001", Mnc: "01"}, group, code); err != nil {
		t.Fatal(err)
	}

	served, err := m.ServedTAIs(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	ue.AllocateRegistrationArea(served)

	mmeUEID := ue.Conn().MMEUES1APID

	if err := m.NotifyDownlinkData(context.Background(), ue.IMSI(), mme.DefaultERABID, models.DownlinkDataErrorIndication); err != nil {
		t.Fatalf("NotifyDownlinkData: %v", err)
	}

	if ue.PagingState() != mme.PagingIdle {
		t.Fatal("the UE was paged before S1 was released")
	}

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(mmeUEID), ENBUES1APID: s1ap.Ptr(s1ap.ENBUES1APID(7))}
	b, _ := complete.Marshal()
	cpdu, _ := s1ap.Unmarshal(b)

	HandleUEContextReleaseComplete(context.Background(), m, mme.NewRadioForTest(cc), cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.Connected() {
		t.Fatal("the UE should be ECM-IDLE after the Release Complete")
	}

	if ue.PagingState() != mme.PagingAttempting {
		t.Fatal("the UE was never paged after the Error Indication released S1 (TS 23.007 clause 22)")
	}

	pending := ue.PagingPending()
	if pending == nil || pending.Ebi != mme.DefaultERABID {
		t.Errorf("paging pending = %+v, want the EBI the anchor reported", pending)
	}
}
