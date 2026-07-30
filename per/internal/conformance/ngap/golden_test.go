// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngaptest

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
)

// NGAP uses ALIGNED PER (TS 38.413 §9.5).
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

	// 00          preamble: RANNodeName absent (1 bit = 0) + 7 pad
	// 00 F1 10    PLMN (fixed 3 octets, no length determinant)
	// 00          GNBIDChoice idx 0 (1 bit) + GNBID length (2 bits, n-lb = 0)
	//             + 5 pad
	// 01          GNBID.Value = 1 (octet-aligned, 1 octet)
	// 00          SupportedTAList count: range 1..256 → one octet, n-lb = 0
	// 00 F1 10    TAI item 0: PLMN
	// 00 00 01    TAI item 0: TAC
	// 80          DefaultPagingDRX = 2 (2 bits: 10) + 6 pad
	expected := []byte{
		0x00,
		0x00, 0xF1, 0x10,
		0x00,
		0x01,
		0x00,
		0x00, 0xF1, 0x10,
		0x00, 0x00, 0x01,
		0x80,
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

// X.691 §14: a root value is ext-bit 0 plus a 3-bit index, an extension
// addition ext-bit 1 plus a normally-small number.
func TestCauseMiscEnumVectors(t *testing.T) {
	cases := []struct {
		value int
		want  []byte
	}{
		{0, []byte{0x00}}, // 0 000 + 4 pad
		{5, []byte{0x50}}, // 0 101 + 4 pad
		{6, []byte{0x80}}, // 1 (ext) + normally-small 0 (0 + 000000) + 1 pad
		{7, []byte{0x81}}, // 1 (ext) + normally-small 1 (0 + 000001) + 1 pad
	}
	for _, c := range cases {
		buf, err := per.Marshal(&CauseMisc{Value: c.value}, per.Aligned)
		if err != nil {
			t.Fatalf("value %d: marshal: %v", c.value, err)
		}

		if !bytes.Equal(buf, c.want) {
			t.Fatalf("value %d: encoding = %s, want %s",
				c.value, hex.EncodeToString(buf), hex.EncodeToString(c.want))
		}

		var got CauseMisc
		if err := per.Unmarshal(buf, &got, per.Aligned); err != nil {
			t.Fatalf("value %d: unmarshal: %v", c.value, err)
		}

		if got.Value != c.value {
			t.Fatalf("round-trip = %d, want %d", got.Value, c.value)
		}
	}
}
