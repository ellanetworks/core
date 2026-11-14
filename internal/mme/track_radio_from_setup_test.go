// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
	"github.com/ellanetworks/core/sctp"
)

func setupRequestValue(t *testing.T, req *s1ap.S1SetupRequest) []byte {
	t.Helper()

	b, err := req.Marshal()
	if err != nil {
		t.Fatalf("marshal S1 Setup Request: %v", err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal S1 Setup Request: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok {
		t.Fatalf("S1 Setup Request decoded as %T", pdu)
	}

	return im.Value
}

func TestTrackRadioFromSetupReturnsTheRefOfAnUnnamedENB(t *testing.T) {
	m := newTestMME(t)

	value := setupRequestValue(t, &s1ap.S1SetupRequest{
		GlobalENBID:      testENBID(1),
		SupportedTAs:     s1ap.SupportedTAs{{TAC: 1, BroadcastPLMNs: s1ap.BPLMNs{{0x00, 0xf1, 0x10}}}},
		DefaultPagingDRX: new(s1ap.PagingDRX),
	})

	want, ok := testRanNodeID(t, 1).Ref()
	if !ok {
		t.Fatal("test eNB has no ref")
	}

	if got := m.TrackRadioFromSetup(context.Background(), new(sctp.SCTPConn), value); got != want {
		t.Fatalf("TrackRadioFromSetup = %q, want %q", got, want)
	}
}

func TestTrackRadioFromSetupReturnsNoRefOnAParseFailure(t *testing.T) {
	m := newTestMME(t)

	if got := m.TrackRadioFromSetup(context.Background(), new(sctp.SCTPConn), []byte{0xff}); got != "" {
		t.Fatalf("TrackRadioFromSetup = %q, want empty", got)
	}
}
