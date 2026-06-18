// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"bytes"
	"context"
	"testing"
)

// fakeEPSStore is a SubscriberStore seeded with TS 35.208 test-set-1 credentials.
type fakeEPSStore struct {
	sub        Subscriber
	updatedSQN string
}

func (f *fakeEPSStore) GetSubscriber(_ context.Context, _ string) (*Subscriber, error) {
	s := f.sub

	return &s, nil
}

func (f *fakeEPSStore) UpdateSequenceNumber(_ context.Context, _, sqn string) error {
	f.updatedSQN = sqn

	return nil
}

// TestGenerateEPSVectorConsistency checks the EPS-AKA vector the way a UE does:
// the RAND is internally random, so rather than a fixed known-answer it verifies
// every relationship in the assembly — XRES = f2, AUTN = (SQN⊕AK)‖AMF‖f1, K_ASME =
// KDF, and that the SQN advanced one IND step and was persisted.
func TestGenerateEPSVectorConsistency(t *testing.T) {
	store := &fakeEPSStore{sub: Subscriber{
		PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
		SequenceNumber: "000000000000",
	}}
	svc := New(store, func(string, int) (string, error) { return "", nil })

	plmn := []byte{0x00, 0xf1, 0x10}

	vec, err := svc.GenerateEPSVector(context.Background(), "001010000000001", plmn, "", "")
	if err != nil {
		t.Fatal(err)
	}

	k := mustHex(t, "465b5ce8b199b49faa5f0a2ee238a6bc")
	opc := mustHex(t, "cd63cb71954a9f4e48a5994e37a02baf")
	amf := mustHex(t, "8000")

	// UE side: f2345 over the returned RAND yields XRES, CK, IK, AK.
	res := make([]byte, 8)
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	ak := make([]byte, 6)

	if err := F2345(opc, k, vec.RAND[:], res, ck, ik, ak, nil); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(vec.XRES, res) {
		t.Fatalf("XRES = %x, recomputed %x", vec.XRES, res)
	}

	// Recover SQN from AUTN = (SQN⊕AK)‖AMF‖MAC-A and check it advanced by one IND
	// step (IndStep) from the seeded 0.
	sqnXorAK := vec.AUTN[0:6]

	sqn := make([]byte, 6)
	for i := range sqn {
		sqn[i] = sqnXorAK[i] ^ ak[i]
	}

	if want := []byte{0, 0, 0, 0, 0, byte(IndStep)}; !bytes.Equal(sqn, want) {
		t.Fatalf("SQN = %x, want %x (advanced by one IND step)", sqn, want)
	}

	if !bytes.Equal(vec.AUTN[6:8], amf) {
		t.Fatalf("AUTN AMF = %x, want %x", vec.AUTN[6:8], amf)
	}

	macA := make([]byte, 8)
	macS := make([]byte, 8)

	if err := F1(opc, k, vec.RAND[:], sqn, amf, macA, macS); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(vec.AUTN[8:16], macA) {
		t.Fatalf("AUTN MAC-A = %x, recomputed %x", vec.AUTN[8:16], macA)
	}

	wantKASME, err := deriveKASME(ck, ik, plmn, sqnXorAK)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(vec.KASME, wantKASME) {
		t.Fatalf("K_ASME = %x, recomputed %x", vec.KASME, wantKASME)
	}

	if store.updatedSQN != "000000000020" {
		t.Fatalf("persisted SQN = %q, want 000000000020", store.updatedSQN)
	}
}
