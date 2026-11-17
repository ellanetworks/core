// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/sctp"
)

func TestS1SetupOutcomeCarriesTheRefOfItsRequest(t *testing.T) {
	m := newTestMME(t)
	conn := new(sctp.SCTPConn)
	m.trackRadio(context.Background(), conn, RadioInfo{})

	const ref = "eNB:001-01:MacroeNB-00001"

	m.setSetupRef(conn, ref)

	for _, procedure := range []S1APProcedure{S1APProcedureS1SetupFailure, S1APProcedureS1SetupResponse} {
		if _, got, _ := m.enbIdentityByConn(conn, procedure); got != ref {
			t.Errorf("%s ref = %q, want %q", procedure, got, ref)
		}
	}

	if _, got, _ := m.enbIdentityByConn(conn, S1APProcedureS1SetupRequest); got != "" {
		t.Errorf("S1SetupRequest ref = %q, want empty: the eNB has not claimed an identity", got)
	}

	for _, procedure := range []S1APProcedure{S1APProcedureS1SetupFailure, S1APProcedureS1SetupResponse, S1APProcedureS1SetupRequest} {
		if _, _, id := m.enbIdentityByConn(conn, procedure); id != "" {
			t.Errorf("%s radio ID = %q, want empty: the eNB has not claimed an identity", procedure, id)
		}
	}

	if id := m.radioIDByConn(conn); id != "" {
		t.Errorf("radio ID = %q, want empty: the eNB has not claimed an identity", id)
	}
}

func TestRadioIDFollowsTheClaimedENBID(t *testing.T) {
	m := newTestMME(t)
	conn := new(sctp.SCTPConn)
	m.trackRadio(context.Background(), conn, RadioInfo{})

	radio := m.RadioForConn(conn)
	claimENBID(t, m, radio, testENBID(1))

	want := testENBKey(t, 1)

	for _, procedure := range []S1APProcedure{S1APProcedureS1SetupResponse, S1APProcedureErrorIndication} {
		if _, _, id := m.enbIdentityByConn(conn, procedure); id != want {
			t.Errorf("%s radio ID = %q, want %q", procedure, id, want)
		}
	}

	if id := m.radioIDByConn(conn); id != want {
		t.Errorf("radio ID = %q, want %q", id, want)
	}
}
