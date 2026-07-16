// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex %q: %v", s, err)
	}

	return b
}

// TestDecodeUncertaintyMatchesSpecTable checks decodeUncertainty against the
// worked values published in TS 23.032 Table 1 (the uncertainty function
// r = C·((1+x)^K − 1), C=10, x=0.1). The small values are given exactly in the
// table; the large ones are quoted to a few significant figures, so those are
// checked within 2%.
func TestDecodeUncertaintyMatchesSpecTable(t *testing.T) {
	for _, tc := range []struct {
		k        int64
		wantM    int64
		tolFracM float64 // fractional tolerance; 0 means exact
	}{
		{0, 0, 0},
		{1, 1, 0},
		{2, 2, 0},            // spec 2.1 m, decoder rounds to int metres
		{20, 57, 0},          // spec 57.3 m
		{40, 443, 0},         // spec 443 m
		{60, 3000, 0.02},     // spec "3 km"
		{80, 20000, 0.03},    // spec "20 km"
		{100, 138000, 0.01},  // spec "138 km"
		{120, 927000, 0.01},  // spec "927 km"
		{127, 1800000, 0.01}, // spec "1800 km"
	} {
		got := decodeUncertainty(tc.k)

		if tc.tolFracM == 0 {
			if got != tc.wantM {
				t.Errorf("K=%d: got %d m, want %d m", tc.k, got, tc.wantM)
			}

			continue
		}

		tol := int64(float64(tc.wantM) * tc.tolFracM)
		if got < tc.wantM-tol || got > tc.wantM+tol {
			t.Errorf("K=%d: got %d m, want %d m ±%d", tc.k, got, tc.wantM, tol)
		}
	}
}

// TestEncodeLongitudeFloors guards TS 23.032 §6.1: N is floor(X·2^24/360), not
// a truncation toward zero. A western (negative) longitude one part past a cell
// boundary must round down to the more negative cell, or it lands one 24-bit
// cell (~2.4 m) east of where it belongs.
func TestEncodeLongitudeFloors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lonE7 int32
		wantN int64
	}{
		{"east of a boundary rounds down", 214577, 1000},
		{"west of a boundary rounds down", -214577, -1001},
		{"prime meridian", 0, 0},
		{"one E7 west", -1, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotN := encodeLongitude(tc.lonE7) - longitudeOffset
			if gotN != tc.wantN {
				t.Errorf("encodeLongitude(%d): N=%d, want %d", tc.lonE7, gotN, tc.wantN)
			}
		})
	}
}

// TestSessionAckInboundDeduplicates covers duplicate detection: a byte-identical
// retransmission is flagged (TS 37.355 §4.3.4) while a distinct PDU that reuses
// the same LPP sequence number is not, and each acknowledgement still draws a
// fresh downlink sequence number (§4.3.3). The two real handset replies below
// both carry uplink sequence number 1.
func TestSessionAckInboundDeduplicates(t *testing.T) {
	caps := mustHex(t, "f001014200")      // ProvideCapabilities, seq 1
	loc := mustHex(t, "f003014a18014180") // ProvideLocationInformation, seq 1

	s := NewSession("imsi-1", "sess-1", "agnss_ue_assisted")

	if ackSeq, dup := s.AckInbound(caps); dup || ackSeq != 0 {
		t.Errorf("first PDU: got (ackSeq=%d, dup=%v), want (0, false)", ackSeq, dup)
	}

	if ackSeq, dup := s.AckInbound(caps); !dup || ackSeq != 1 {
		t.Errorf("retransmitted PDU: got (ackSeq=%d, dup=%v), want (1, true)", ackSeq, dup)
	}

	// Distinct message, same LPP sequence number: must not be treated as a dup.
	if ackSeq, dup := s.AckInbound(loc); dup || ackSeq != 2 {
		t.Errorf("distinct PDU reusing seq: got (ackSeq=%d, dup=%v), want (2, false)", ackSeq, dup)
	}
}
