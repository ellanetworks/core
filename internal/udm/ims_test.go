// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/milenage"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/core/internal/udm"
)

const (
	imsTestK   = "465b5ce8b199b49faa5f0a2ee238a6bc"
	imsTestOPc = "cd63cb71954a9f4e48a5994e37a02baf"
)

type fakeIMSStore struct {
	sqn string
	err error
}

func (f *fakeIMSStore) AdvanceIMSSequenceNumber(_ context.Context, _, resyncAuts, resyncRand string) (*udm.AdvancedCredentials, error) {
	if f.err != nil {
		return nil, f.err
	}

	next, err := sqn.NextIMS(f.sqn)
	if resyncAuts != "" {
		next, err = sqn.NextIMSResync(f.sqn, imsTestOPc, imsTestK, resyncAuts, resyncRand)
	}

	if err != nil {
		return nil, err
	}

	f.sqn = next

	return &udm.AdvancedCredentials{PermanentKey: imsTestK, Opc: imsTestOPc, SequenceNumber: next}, nil
}

func TestGenerateIMSVectorIsAcceptedByTheUE(t *testing.T) {
	store := &fakeIMSStore{sqn: "000000000020"}

	av, err := udm.NewIMSCredentials(store).GenerateIMSVector(context.Background(), "001010000000001", nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	k, _ := hex.DecodeString(imsTestK)
	opc, _ := hex.DecodeString(imsTestOPc)

	res, ck, ik := make([]byte, 8), make([]byte, 16), make([]byte, 16)
	ak := make([]byte, 6)

	if err := milenage.F2345(opc, k, av.RAND, res, ck, ik, ak, nil); err != nil {
		t.Fatalf("f2345: %v", err)
	}

	sqnUE := make([]byte, 6)
	for i := range sqnUE {
		sqnUE[i] = av.AUTN[i] ^ ak[i]
	}

	if got := hex.EncodeToString(sqnUE); got != store.sqn {
		t.Fatalf("SQN = %s, want %s", got, store.sqn)
	}

	if ind := uint64(sqnUE[5] & 0x1f); ind != sqn.IMSInd {
		t.Fatalf("IND = %d, want %d", ind, sqn.IMSInd)
	}

	amf := av.AUTN[6:8]
	if !bytes.Equal(amf, []byte{0x00, 0x00}) {
		t.Fatalf("AMF = %x, want 0000", amf)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	if err := milenage.F1(opc, k, av.RAND, sqnUE, amf, macA, macS); err != nil {
		t.Fatalf("f1: %v", err)
	}

	switch {
	case len(av.RAND) != 16 || len(av.AUTN) != 16:
		t.Fatalf("RAND %d octets, AUTN %d octets", len(av.RAND), len(av.AUTN))
	case !bytes.Equal(macA, av.AUTN[8:]):
		t.Fatalf("MAC-A mismatch")
	case !bytes.Equal(res, av.XRES):
		t.Fatalf("RES %x != XRES %x", res, av.XRES)
	case !bytes.Equal(ck, av.CK) || !bytes.Equal(ik, av.IK):
		t.Fatalf("CK/IK mismatch")
	}
}

func TestGenerateIMSVectorPropagatesUnknownSubscriber(t *testing.T) {
	store := &fakeIMSStore{err: udm.ErrSubscriberUnknown}

	_, err := udm.NewIMSCredentials(store).GenerateIMSVector(context.Background(), "001010000000099", nil)
	if !errors.Is(err, udm.ErrSubscriberUnknown) {
		t.Fatalf("err = %v, want ErrSubscriberUnknown", err)
	}
}
