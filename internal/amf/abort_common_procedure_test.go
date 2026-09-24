// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "testing"

func TestAbortedCommonProcedureKeepsARegisteredUE(t *testing.T) {
	a := New(nil, nil, nil)
	radio := &Radio{amf: a, name: "gnb-1", Conn: &downlinkOrderConn{}}

	ueConn, err := a.NewUeConn(radio, 1)
	if err != nil {
		t.Fatalf("NewUeConn: %v", err)
	}

	ue := NewUeContext()
	ue.ForceStateForTest(Registered)
	a.AttachUeConn(t.Context(), ue, ueConn)

	a.abortCommonProcedure(t.Context(), ue)

	if ue.State() != Registered {
		t.Fatalf("state = %s after an aborted authentication, want Registered: TS 24.501 §5.4.1.3.7 releases the N1 NAS signalling connection only", ue.State())
	}
}
