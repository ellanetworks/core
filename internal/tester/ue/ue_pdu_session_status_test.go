// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import "testing"

func TestActivePDUSessionIDsReportsEstablishedSessions(t *testing.T) {
	u := &UE{pduSessions: map[uint8]PDUSessionInfo{}}

	if got := u.ActivePDUSessionIDs(); len(got) != 0 {
		t.Fatalf("ActivePDUSessionIDs = %v, want none", got)
	}

	u.pduSessions[5] = PDUSessionInfo{PDUSessionID: 5}
	u.pduSessions[1] = PDUSessionInfo{PDUSessionID: 1}

	got := u.ActivePDUSessionIDs()
	if len(got) != 2 || got[0] != 1 || got[1] != 5 {
		t.Fatalf("ActivePDUSessionIDs = %v, want [1 5]", got)
	}
}
