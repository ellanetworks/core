// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ausf

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sqn"
	"github.com/ellanetworks/core/internal/udm"
)

// internalStore is a minimal in-memory SubscriberStore for same-package tests.
type internalStore struct {
	mu   sync.Mutex
	subs map[string]*Subscriber
}

func newInternalStore() *internalStore {
	return &internalStore{subs: make(map[string]*Subscriber)}
}

func (s *internalStore) add(imsi string, sub *Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.subs[imsi] = sub
}

func (s *internalStore) GetSubscriber(_ context.Context, imsi string) (*Subscriber, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subs[imsi]
	if !ok {
		return nil, fmt.Errorf("subscriber %s not found", imsi)
	}

	return sub, nil
}

func (s *internalStore) UpdateSequenceNumber(_ context.Context, imsi string, sqn string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subs[imsi]
	if !ok {
		return fmt.Errorf("subscriber %s not found", imsi)
	}

	sub.SequenceNumber = sqn

	return nil
}

var (
	intTestPLMN  = models.PlmnID{Mcc: "001", Mnc: "01"}
	intTestSN, _ = intTestPLMN.ServingNetworkName()
)

const (
	intTestK    = "465b5ce8b199b49faa5f0a2ee238a6bc"
	intTestOPc  = "cd63cb71954a9f4e48a5994e37a02baf"
	intTestIMSI = "001010000000001"
)

var intTestSUCI = "suci-0-001-01-0000-0-0-0000000001"

func noopKeys(_ string, _ int) (string, error) {
	return "", fmt.Errorf("not expected")
}

// TestConfirmSuccess exercises the full Authenticate→Confirm happy path
// from inside the package. After Authenticate, we replicate what the UE
// does: run Milenage F2345 with the same RAND/K/OPc to obtain RES, CK,
// IK, then derive XRES* via the same KDF.
func TestConfirmSuccess(t *testing.T) {
	store := newInternalStore()
	store.add(intTestIMSI, &Subscriber{
		PermanentKey:   intTestK,
		Opc:            intTestOPc,
		SequenceNumber: "000000000000",
	})

	a := New(store, noopKeys)
	ctx := context.Background()

	result, err := a.Authenticate(ctx, intTestSUCI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Replicate UE-side derivation of RES*.
	k, _ := hex.DecodeString(intTestK)
	opc, _ := hex.DecodeString(intTestOPc)
	randBytes, _ := hex.DecodeString(result.Rand)

	RES := make([]byte, 8)
	CK := make([]byte, 16)
	IK := make([]byte, 16)

	if err := udm.F2345(opc, k, randBytes, RES, CK, IK, nil, nil); err != nil {
		t.Fatalf("F2345 failed: %v", err)
	}

	xresStar, err := udm.DeriveXresStar(CK, IK, intTestSN, randBytes, RES)
	if err != nil {
		t.Fatalf("udm.DeriveXresStar failed: %v", err)
	}

	xresStarHex := hex.EncodeToString(xresStar)

	supi, kseaf, err := a.Confirm(ctx, result.AuthCtxID, xresStarHex)
	if err != nil {
		t.Fatalf("Confirm failed: %v", err)
	}

	if supi.IMSI() != intTestIMSI {
		t.Fatalf("expected IMSI %s, got %s", intTestIMSI, supi.IMSI())
	}

	if len(kseaf) == 0 {
		t.Fatal("expected non-empty Kseaf")
	}
}

// TestConfirmSuccess_ReturnsCorrectKseaf verifies that the Kseaf returned
// by Confirm matches an independently derived value.
func TestConfirmSuccess_ReturnsCorrectKseaf(t *testing.T) {
	store := newInternalStore()
	store.add(intTestIMSI, &Subscriber{
		PermanentKey:   intTestK,
		Opc:            intTestOPc,
		SequenceNumber: "000000000000",
	})

	a := New(store, noopKeys)
	ctx := context.Background()

	result, err := a.Authenticate(ctx, intTestSUCI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}

	// Derive RES* and Kseaf the same way the AUSF does internally.
	k, _ := hex.DecodeString(intTestK)
	opc, _ := hex.DecodeString(intTestOPc)
	randBytes, _ := hex.DecodeString(result.Rand)

	// We need SQN⊕AK to derive Kausf. Parse AUTN: first 6 bytes are SQN⊕AK.
	autnBytes, _ := hex.DecodeString(result.Autn)
	sqnXorAK := autnBytes[:6]

	RES := make([]byte, 8)
	CK := make([]byte, 16)
	IK := make([]byte, 16)

	if err := udm.F2345(opc, k, randBytes, RES, CK, IK, nil, nil); err != nil {
		t.Fatalf("F2345 failed: %v", err)
	}

	xresStar, err := udm.DeriveXresStar(CK, IK, intTestSN, randBytes, RES)
	if err != nil {
		t.Fatalf("udm.DeriveXresStar failed: %v", err)
	}

	kausf, err := udm.DeriveKausf(CK, IK, intTestSN, sqnXorAK)
	if err != nil {
		t.Fatalf("udm.DeriveKausf failed: %v", err)
	}

	expectedKseaf, err := deriveKseaf(kausf, intTestSN)
	if err != nil {
		t.Fatalf("deriveKseaf failed: %v", err)
	}

	_, gotKseaf, err := a.Confirm(ctx, result.AuthCtxID, hex.EncodeToString(xresStar))
	if err != nil {
		t.Fatalf("Confirm failed: %v", err)
	}

	if hex.EncodeToString(gotKseaf) != hex.EncodeToString(expectedKseaf) {
		t.Fatalf("Kseaf mismatch:\n  got  %s\n  want %s", hex.EncodeToString(gotKseaf), hex.EncodeToString(expectedKseaf))
	}
}

// TestResyncSuccess_SQNAdvancesBy33 does a full resync round-trip:
//  1. Authenticate normally (issues the challenge RAND).
//  2. Build a valid AUTS from that RAND and a known SQN_MS (simulating
//     what the UE sends when it detects a SQN mismatch).
//  3. Call Authenticate again with the ResyncInfo.
//  4. Assert the stored SQN equals udm.AdvanceSQN(sqnMs, 33).
func TestResyncSuccess_SQNAdvancesBy33(t *testing.T) {
	// UE's true SQN — the value the USIM holds.
	const sqnMsHex = "000000000100"

	store := newInternalStore()
	store.add(intTestIMSI, &Subscriber{
		PermanentKey:   intTestK,
		Opc:            intTestOPc,
		SequenceNumber: "000000000000",
	})

	a := New(store, noopKeys)
	ctx := context.Background()

	// Step 1: Normal authenticate — this issues the challenge RAND.
	result, err := a.Authenticate(ctx, intTestSUCI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("first Authenticate failed: %v", err)
	}

	randBytes, _ := hex.DecodeString(result.Rand)
	k, _ := hex.DecodeString(intTestK)
	opc, _ := hex.DecodeString(intTestOPc)
	sqnMs, _ := hex.DecodeString(sqnMsHex)

	// Step 2: Build a valid AUTS = (SQN_MS ⊕ AK) || MAC-S.
	AK := make([]byte, 6)
	if err := udm.F2345(opc, k, randBytes, nil, nil, nil, nil, AK); err != nil {
		t.Fatalf("F2345 for AK failed: %v", err)
	}

	concSQN := make([]byte, 6)
	for i := range 6 {
		concSQN[i] = sqnMs[i] ^ AK[i]
	}

	amfZero, _ := hex.DecodeString("0000")
	macS := make([]byte, 8)

	if err := udm.F1(opc, k, randBytes, sqnMs, amfZero, nil, macS); err != nil {
		t.Fatalf("F1 for MAC-S failed: %v", err)
	}

	auts := append(concSQN, macS...)

	// Step 3: Authenticate with resync.
	_, err = a.Authenticate(ctx, intTestSUCI, intTestPLMN, &ResyncInfo{
		Rand: result.Rand,
		Auts: hex.EncodeToString(auts),
	})
	if err != nil {
		t.Fatalf("resync Authenticate failed: %v", err)
	}

	// Step 4: Verify stored SQN = sqnMs + 33 (udm.IndStep + 1).
	store.mu.Lock()
	gotSQN := store.subs[intTestIMSI].SequenceNumber
	store.mu.Unlock()

	wantSQN, _ := udm.AdvanceSQN(sqnMsHex, udm.IndStep+1) // 0x100 + 33 = 0x121
	if gotSQN != wantSQN {
		t.Fatalf("after resync: want SQN %s, got %s", wantSQN, gotSQN)
	}
}

func ueResStar(t *testing.T, result *AuthResult) string {
	t.Helper()

	k, _ := hex.DecodeString(intTestK)
	opc, _ := hex.DecodeString(intTestOPc)
	randBytes, _ := hex.DecodeString(result.Rand)

	res := make([]byte, 8)
	ck := make([]byte, 16)
	ik := make([]byte, 16)

	if err := udm.F2345(opc, k, randBytes, res, ck, ik, nil, nil); err != nil {
		t.Fatalf("F2345 failed: %v", err)
	}

	resStar, err := udm.DeriveXresStar(ck, ik, intTestSN, randBytes, res)
	if err != nil {
		t.Fatalf("udm.DeriveXresStar failed: %v", err)
	}

	return hex.EncodeToString(resStar)
}

func TestAuthenticateBySUPIConfirmsTheSubscriber(t *testing.T) {
	store := &internalStore{subs: map[string]*Subscriber{intTestIMSI: {PermanentKey: intTestK, Opc: intTestOPc, SequenceNumber: "000000000000"}}}

	a := New(store, noopKeys)

	result, err := a.Authenticate(t.Context(), "imsi-"+intTestIMSI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("Authenticate by SUPI failed: %v", err)
	}

	supi, kseaf, err := a.Confirm(t.Context(), result.AuthCtxID, ueResStar(t, result))
	if err != nil {
		t.Fatalf("Confirm failed: %v", err)
	}

	if supi.IMSI() != intTestIMSI || len(kseaf) == 0 {
		t.Fatalf("Confirm = %s, %x, want SUPI %s and a K_SEAF", supi, kseaf, intTestIMSI)
	}
}

func TestConcurrentAttemptsForOneSubscriberConfirmIndependently(t *testing.T) {
	store := &internalStore{subs: map[string]*Subscriber{intTestIMSI: {PermanentKey: intTestK, Opc: intTestOPc, SequenceNumber: "000000000000"}}}

	a := New(store, noopKeys)
	ctx := t.Context()

	first, err := a.Authenticate(ctx, intTestSUCI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("first Authenticate failed: %v", err)
	}

	second, err := a.Authenticate(ctx, "imsi-"+intTestIMSI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("second Authenticate failed: %v", err)
	}

	third, err := a.Authenticate(ctx, "imsi-"+intTestIMSI, intTestPLMN, nil)
	if err != nil {
		t.Fatalf("third Authenticate failed: %v", err)
	}

	if _, _, err := a.Confirm(ctx, third.AuthCtxID, ueResStar(t, second)); err == nil {
		t.Fatal("an attempt confirmed with the RES* of another attempt")
	}

	for name, result := range map[string]*AuthResult{"first": first, "second": second} {
		supi, kseaf, err := a.Confirm(ctx, result.AuthCtxID, ueResStar(t, result))
		if err != nil {
			t.Fatalf("Confirm of the %s attempt failed: %v", name, err)
		}

		if supi.IMSI() != intTestIMSI || len(kseaf) == 0 {
			t.Fatalf("Confirm of the %s attempt = %s, %x", name, supi, kseaf)
		}

		if _, _, err := a.Confirm(ctx, result.AuthCtxID, ueResStar(t, result)); !errors.Is(err, ErrAuthContextNotFound) {
			t.Fatalf("second Confirm of the %s attempt = %v, want %v", name, err, ErrAuthContextNotFound)
		}
	}
}

func (f *internalStore) AdvanceSequenceNumber(_ context.Context, imsi, resyncAuts, resyncRand string) (*AdvancedCredentials, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sub, ok := f.subs[imsi]
	if !ok {
		return nil, fmt.Errorf("subscriber %s not found", imsi)
	}

	next, err := sqn.Next(sub.SequenceNumber, sub.Opc, sub.PermanentKey, resyncAuts, resyncRand)
	if err != nil {
		return nil, err
	}

	sub.SequenceNumber = next

	return &AdvancedCredentials{
		PermanentKey:   sub.PermanentKey,
		Opc:            sub.Opc,
		SequenceNumber: next,
	}, nil
}
