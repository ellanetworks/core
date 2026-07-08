// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestUEContextReleaseCompleteArmsMobileReachable(t *testing.T) {
	m := newTestMME(t)

	ue, cc := securedUE(t, m) // connected; the release moves it to ECM-IDLE

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: 7}

	b, err := complete.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	cpdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(cc), cpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.Connected() {
		t.Fatal("UE still connected after S1 release")
	}

	if !ue.MobileReachableArmedForTest() {
		t.Fatal("mobile reachable timer not armed when UE moved to ECM-IDLE")
	}

	m.RemoveUe(ue) // stop the default-duration timer
}
