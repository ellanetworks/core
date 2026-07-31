// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrctest

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

// NR-RRC uses BASIC-PER Unaligned (TS 38.331 §8.1), which packs every field
// bit-adjacent, MSB-first, both SEQUENCE preamble bits ahead of the fields:
//
//	01  rrc-TransactionID = 1 (2 bits, range 0..3)
//	0   criticalExtensions CHOICE index = 0 (1 bit, 2 alternatives)
//	1   deprioritisationReq present (preamble)
//	0   extended absent (preamble)
//	0   type = 0 (1 bit, range 0..1)
//	1   time = 1 (1 bit, range 0..1)
//	0   pad to the octet boundary
//	= 0101_0010 = 0x52
func TestRRCReleaseGoldenVector(t *testing.T) {
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
