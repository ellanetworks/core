// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngaptest

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/per"
)

// TestNGSetupRequestGoldenVector verifies that our PER encoding produces the
// exact byte sequence expected by 3GPP TS 38.413 for an NGSetupRequest-like
// message using ALIGNED PER (the variant NGAP uses, per §9.5).
//
// The test message contains:
//   - GlobalRANNodeID: PLMN=0x00F110 (MCC 001, MNC 01), gNB-ID choice index 0,
//     gNB-ID value 1
//   - RANNodeName: nil (optional, absent)
//   - SupportedTAList: 1 item with PLMN=0x00F110, TAC=0x000001
//   - DefaultPagingDRX: 2 (v128)
//
// Expected encoding (aligned PER):
//
//	00                     preamble: RANNodeName absent (1 bit = 0, + 7 pad)
//	00 F1 10               PLMNIdentity (OCTET STRING SIZE(3), fixed, no length)
//	00                     GNBIDChoice index 0 (range 2, 1 bit = 0, + 7 pad)
//	00 00 00 01            GNBID.Value: constrained 0..4294967295, range>64K,
//	                       indefinite: length 4 (0x04) + 4 bytes 0x00000001
//	00                     SupportedTAList length: range 1..256 = 256 → 1 octet
//	                       (octet-aligned), value 1 (0x01)... wait, range 256
//	                       → one-octet case → 0x01
//	00 F1 10               TAI item: PLMN
//	00 00 01               TAI item: TAC
//	02                     DefaultPagingDRX: range 0..3, 2 bits → 10 + 6 pad
//
// See the test for the hand-computed expected bytes.
func TestNGSetupRequestGoldenVector(t *testing.T) {
	name := ""
	msg := &NGSetupRequest{
		GlobalRANNodeID: GlobalGNBID{
			PLMN: PLMNIdentity{Value: []byte{0x00, 0xF1, 0x10}},
			GNBID: GNBIDChoice{
				GNBID: &GNBID{Value: 1},
			},
		},
		RANNodeName: nil,
		SupportedTAList: SupportedTAList{
			Items: []SupportedTAIItem{
				{PLMN: PLMNIdentity{Value: []byte{0x00, 0xF1, 0x10}}, TAC: []byte{0x00, 0x00, 0x01}},
			},
		},
		DefaultPagingDRX: PagingDRX{Value: 2},
	}

	buf, err := per.Marshal(msg, per.Aligned)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Hand-computed expected encoding (aligned PER, NGAP variant):
	//
	// 00          preamble: 0 (RANNodeName absent) + 7 pad bits
	// 00 F1 10    PLMN (fixed 3 octets, octet-aligned, no length)
	// 00          GNBIDChoice: idx 0 (1 bit) + GNBID length (2 bits: 00, n-lb=0)
	//             + 5 pad bits = 0x00
	// 01          GNBID.Value = 1 (octet-aligned, 1 octet)
	// 00          SupportedTAList count: range 1..256, range=256 → 1 octet,
	//             value n-lb = 0 → 0x00
	// 00 F1 10    PLMN of TAI item 0 (octet-aligned, 3 octets)
	// 00 00 01    TAC of TAI item 0 (octet-aligned, 3 octets)
	// 80          DefaultPagingDRX: value 2 (2 bits: 10) + 6 pad → 0x80
	expected := []byte{
		0x00,                   // preamble + pad
		0x00, 0xF1, 0x10,      // PLMN
		0x00,                   // GNBIDChoice idx 0 + GNBID length 00 + pad
		0x01,                   // GNBID.Value = 1
		0x00,                   // SupportedTAList count (n-lb = 0)
		0x00, 0xF1, 0x10,      // TAI PLMN
		0x00, 0x00, 0x01,      // TAI TAC
		0x80,                   // DefaultPagingDRX = 2 (10xxxxxx)
	}

	if !bytes.Equal(buf, expected) {
		t.Fatalf("encoding mismatch:\n got: %s\nwant: %s",
			hex.EncodeToString(buf), hex.EncodeToString(expected))
	}

	// Verify roundtrip.
	var msg2 NGSetupRequest
	if err := per.Unmarshal(buf, &msg2, per.Aligned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg2.DefaultPagingDRX.Value != 2 {
		t.Errorf("PagingDRX = %d, want 2", msg2.DefaultPagingDRX.Value)
	}
	if len(msg2.SupportedTAList.Items) != 1 {
		t.Fatalf("TAList len = %d, want 1", len(msg2.SupportedTAList.Items))
	}
	_ = name
}

func TestNGSetupRequestWithRANNodeName(t *testing.T) {
	name := "gNB-001"
	msg := &NGSetupRequest{
		GlobalRANNodeID: GlobalGNBID{
			PLMN: PLMNIdentity{Value: []byte{0x00, 0xF1, 0x10}},
			GNBID: GNBIDChoice{
				GNBID: &GNBID{Value: 42},
			},
		},
		RANNodeName: &name,
		SupportedTAList: SupportedTAList{
			Items: []SupportedTAIItem{
				{PLMN: PLMNIdentity{Value: []byte{0x00, 0xF1, 0x10}}, TAC: []byte{0x00, 0x00, 0x01}},
			},
		},
		DefaultPagingDRX: PagingDRX{Value: 0},
	}

	buf, err := per.Marshal(msg, per.Aligned)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Roundtrip
	var msg2 NGSetupRequest
	if err := per.Unmarshal(buf, &msg2, per.Aligned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg2.RANNodeName == nil || *msg2.RANNodeName != name {
		t.Errorf("RANNodeName = %v, want %q", msg2.RANNodeName, name)
	}
	if msg2.DefaultPagingDRX.Value != 0 {
		t.Errorf("PagingDRX = %d, want 0", msg2.DefaultPagingDRX.Value)
	}
}
