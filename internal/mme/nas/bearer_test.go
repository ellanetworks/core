// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/udm"
)

// tacBearerStore overrides the operator's supported TACs for TAC-parsing tests.
type tacBearerStore struct {
	fakeBearerStore
	tacs string
}

func (s tacBearerStore) GetOperator(_ context.Context) (*db.Operator, error) {
	return &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: s.tacs, Ciphering: `["AES"]`, Integrity: `["AES"]`}, nil
}

// TestOperatorTACsHex confirms supported TACs are parsed as hex (not decimal) and
// that a configured value wider than the 16-bit E-UTRAN TAC is excluded rather
// than narrowed (TS 23.003). "000064" is hex 0x64 (decimal would be 64), "00ffff"
// is the largest valid LTE TAC, and "010002" exceeds 16 bits and is dropped.
func TestOperatorTACsHex(t *testing.T) {
	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), tacBearerStore{tacs: `["000064","00ffff","010002"]`}, &fakeSessionManager{})

	got, err := m.OperatorTACs(context.Background())
	if err != nil {
		t.Fatalf("operatorTACs: %v", err)
	}

	want := []uint16{0x0064, 0xffff}
	if len(got) != len(want) {
		t.Fatalf("operatorTACs = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("operatorTACs[%d] = %#x, want %#x", i, got[i], want[i])
		}
	}
}

func TestBitRateToBps(t *testing.T) {
	cases := []struct {
		in   string
		want uint64
	}{
		{"1000 bps", 1000},
		{"500 Kbps", 500_000},
		{"100 Mbps", 100_000_000},
		{"1 Gbps", 1_000_000_000},
		{"2 Tbps", 2_000_000_000_000},
		{"", 0},
		{"garbage", 0},  // no unit
		{"5", 0},        // missing unit token
		{"abc Mbps", 0}, // non-numeric value
		{"10 Xbps", 0},  // unknown unit
		{"1 Gbps x", 0}, // too many tokens
	}

	for _, c := range cases {
		if got := mme.BitRateToBps(c.in); got != c.want {
			t.Errorf("mme.BitRateToBps(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
