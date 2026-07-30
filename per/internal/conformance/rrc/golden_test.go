// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrctest

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

// TestRRCReleaseGoldenVector verifies UNALIGNED PER encoding of an RRCRelease-
// like message. NR-RRC uses BASIC-PER Unaligned variant per TS 38.331 §8.1.
//
// Test message:
//   - rrc-TransactionIdentifier: 1 (range 0..3, 2 bits → 01)
//   - criticalExtensions: rrcRelease (choice idx 0, range 2, 1 bit → 0)
//   - deprioritisationReq present (OPTIONAL preamble bit → 1)
//   - extended absent (OPTIONAL preamble bit → 0, comes BEFORE mandatory fields)
//   - type: 0 (range 0..1, 1 bit → 0)
//   - time: 1 (range 0..1, 1 bit → 1)
//   - 1 pad bit
//
// UNALIGNED PER packs all bits without padding, MSB-first within the octet:
//
//	01       rrc-TransactionID = 1 (2 bits, range 0..3)
//	0        criticalExtensions CHOICE index = 0 (1 bit, 2 alternatives)
//	1        deprioritisationReq present (1 bit, OPTIONAL preamble)
//	0        type = 0 (1 bit, range 0..1)
//	1        time = 1 (1 bit, range 0..1)
//	0        extended absent (1 bit, OPTIONAL preamble)
//	0        pad to the octet boundary
//	= 0101_0100 = 0x54
func TestRRCReleaseGoldenVector(t *testing.T) {
	extended := false
	msg := &RRCRelease{
		RRCTransactionID: 1,
		CriticalExtensions: ReleaseChoice{
			RRCRelease: &RRCReleaseIEs{
				Deprioritisation: &DeprioritisationReq{
					Type: 0,
					Time: 1,
				},
			},
		},
	}
	_ = extended

	buf, err := per.Marshal(msg, per.Unaligned)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	expected := []byte{0x52}

	if !bytes.Equal(buf, expected) {
		t.Fatalf("encoding mismatch:\n got: %s\nwant: %s",
			hex.EncodeToString(buf), hex.EncodeToString(expected))
	}

	// Verify roundtrip.
	var msg2 RRCRelease
	if err := per.Unmarshal(buf, &msg2, per.Unaligned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg2.RRCTransactionID != 1 {
		t.Errorf("RRCTransactionID = %d, want 1", msg2.RRCTransactionID)
	}

	if msg2.CriticalExtensions.RRCRelease == nil {
		t.Fatal("RRCRelease nil")
	}

	dp := msg2.CriticalExtensions.RRCRelease.Deprioritisation
	if dp == nil {
		t.Fatal("Deprioritisation nil")
	}

	if dp.Type != 0 || dp.Time != 1 {
		t.Errorf("Type=%d Time=%d, want 0,1", dp.Type, dp.Time)
	}
}

func TestRRCReleaseLateChoice(t *testing.T) {
	msg := &RRCRelease{
		RRCTransactionID: 3,
		CriticalExtensions: ReleaseChoice{
			Late: new(bool),
		},
	}

	buf, err := per.Marshal(msg, per.Unaligned)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var msg2 RRCRelease
	if err := per.Unmarshal(buf, &msg2, per.Unaligned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if msg2.RRCTransactionID != 3 {
		t.Errorf("RRCTransactionID = %d, want 3", msg2.RRCTransactionID)
	}

	if msg2.CriticalExtensions.Late == nil {
		t.Fatal("Late nil")
	}
}
