// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

// TestAttachUnknownIMSI checks that an Attach Request from an unprovisioned IMSI
// is rejected with ATTACH REJECT #2 ("IMSI unknown in HSS") and the S1 context
// is released, without starting authentication.
func TestAttachUnknownIMSI(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 7)

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: "001010000000999"},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}

	b, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, b)

	// Expect Attach Reject (downlink NAS) followed by the UE Context Release Command.
	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + Release Command, got %d S1AP messages", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != emmCauseIMSIUnknownInHSS {
		t.Fatalf("Attach Reject cause = %d, want %d", rej.Cause, emmCauseIMSIUnknownInHSS)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}
