// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// TestAttachTrackingAreaNotAllowed checks an Attach from a serving cell outside the
// served area is rejected with ATTACH REJECT #12 and the S1 context released, without
// authenticating (TS 24.301 §5.5.1.2.5).
func TestAttachTrackingAreaNotAllowed(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	// Served PLMN 001/01 but TAC 2, which the operator does not serve (it serves TAC 1).
	ue.Conn().ServingTAI = s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 2}

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: testSubscriber.IMSI},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}

	b, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), b)

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + Release Command, got %d S1AP messages", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != mme.EmmCauseTrackingAreaNotAllowed {
		t.Fatalf("Attach Reject cause = %d, want %d", rej.Cause, mme.EmmCauseTrackingAreaNotAllowed)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}

// TestAttachProtocolError checks an ATTACH REQUEST whose mandatory IEs are absent is
// answered with ATTACH REJECT #96 and the S1 context released (TS 24.301 §5.5.1.2.7 b).
func TestAttachProtocolError(t *testing.T) {
	for _, tc := range []struct {
		name string
		nas  []byte
	}{
		{name: "header only", nas: []byte{0x07, 0x41}},
		{name: "trailing garbage", nas: []byte{0x07, 0x41, 0xff, 0xff, 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			cc := &captureConn{}
			ue := newAttachUe(m, cc, 7)

			HandleNAS(context.Background(), m, ue.Conn(), tc.nas)

			if len(cc.sent) != 2 {
				t.Fatalf("expected Attach Reject + Release Command, got %d S1AP messages", len(cc.sent))
			}

			rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
			if err != nil {
				t.Fatalf("not an Attach Reject: %v", err)
			}

			if rej.Cause != mme.EmmCauseInvalidMandatoryInfo {
				t.Fatalf("Attach Reject cause = %d, want %d", rej.Cause, mme.EmmCauseInvalidMandatoryInfo)
			}

			parseUEContextReleaseCommand(t, cc.sent[1])
		})
	}
}

// TestAttachUnknownIMSI checks that an Attach Request from an unprovisioned IMSI
// is rejected with ATTACH REJECT #2 ("IMSI unknown in HSS") and the S1 context
// is released, without starting authentication.
func TestAttachUnknownIMSI(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

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

	HandleNAS(context.Background(), m, ue.Conn(), b)

	if len(cc.sent) != 2 {
		t.Fatalf("expected Attach Reject + Release Command, got %d S1AP messages", len(cc.sent))
	}

	rej, err := eps.ParseAttachReject(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("not an Attach Reject: %v", err)
	}

	if rej.Cause != mme.EmmCauseIMSIUnknownInHSS {
		t.Fatalf("Attach Reject cause = %d, want %d", rej.Cause, mme.EmmCauseIMSIUnknownInHSS)
	}

	// The reject carries the T3402 back-off (12 min), mirroring the AMF's T3502.
	wantT3402, err := eps.EncodeGPRSTimer(mme.T3402Backoff)
	if err != nil {
		t.Fatal(err)
	}

	if rej.T3402 != wantT3402 {
		t.Fatalf("Attach Reject T3402 = %#x, want %#x (12 min)", rej.T3402, wantT3402)
	}

	parseUEContextReleaseCommand(t, cc.sent[1])
}
