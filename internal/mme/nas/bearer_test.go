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

type tacBearerStore struct {
	fakeBearerStore
	tacs string
}

func (s tacBearerStore) GetOperator(_ context.Context) (*db.Operator, error) {
	return &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: s.tacs, Ciphering: `["AES"]`, Integrity: `["AES"]`}, nil
}

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
