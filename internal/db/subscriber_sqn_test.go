// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"encoding/hex"
	"errors"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/milenage"
)

func TestAdvanceSubscriberSQNIsAtomicUnderConcurrency(t *testing.T) {
	dbInstance := setupTestDB(t)

	const imsi = "001010000000042"

	profileID, _ := createPolicyDeps(t, dbInstance, t.Name())

	if err := dbInstance.CreateSubscriber(context.Background(), &db.Subscriber{
		Imsi:           imsi,
		PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
		SequenceNumber: "000000000000",
		ProfileID:      profileID,
	}); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}

	const workers = 64

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = map[string]int{}
	)

	for range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			creds, err := dbInstance.AdvanceSubscriberSQN(context.Background(), imsi, "", "")
			if err != nil {
				t.Errorf("advance: %v", err)
				return
			}

			mu.Lock()
			seen[creds.SequenceNumber]++
			mu.Unlock()
		}()
	}

	wg.Wait()

	if len(seen) != workers {
		t.Errorf("expected %d distinct sequence numbers, got %d: %v", workers, len(seen), seen)
	}

	for sqn, count := range seen {
		if count > 1 {
			t.Errorf("sequence number %s handed out %d times", sqn, count)
		}
	}
}

func TestAdvanceSubscriberSQNMissingSubscriberIsNotFound(t *testing.T) {
	dbInstance := setupTestDB(t)

	_, err := dbInstance.AdvanceSubscriberSQN(context.Background(), "001010000000999", "", "")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("absent subscriber should report ErrNotFound, got: %v", err)
	}
}

func TestAdvanceSubscriberSQNQueryErrorIsNotReportedAsNotFound(t *testing.T) {
	dbInstance := setupTestDB(t)

	if err := dbInstance.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err := dbInstance.AdvanceSubscriberSQN(context.Background(), "001010000000042", "", "")
	if err == nil {
		t.Fatal("a failing subscriber lookup must surface an error")
	}

	if errors.Is(err, db.ErrNotFound) {
		t.Fatalf("a transient lookup failure must not be reported as a missing subscriber, got: %v", err)
	}
}

func TestAdvanceSubscriberIMSSQNInterleavesWithPacketSwitched(t *testing.T) {
	dbInstance := setupTestDB(t)

	const imsi = "001010000000043"

	profileID, _ := createPolicyDeps(t, dbInstance, t.Name())

	if err := dbInstance.CreateSubscriber(context.Background(), &db.Subscriber{
		Imsi:           imsi,
		PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
		SequenceNumber: "000000000020",
		ProfileID:      profileID,
	}); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}

	steps := []struct {
		ims  bool
		want string
	}{
		{ims: true, want: "00000000005f"},
		{ims: false, want: "000000000080"},
		{ims: true, want: "0000000000bf"},
		{ims: true, want: "0000000000df"},
		{ims: false, want: "000000000100"},
	}

	for i, step := range steps {
		var (
			creds *db.AdvancedCredentials
			err   error
		)

		if step.ims {
			creds, err = dbInstance.AdvanceSubscriberIMSSQN(context.Background(), imsi, "", "")
		} else {
			creds, err = dbInstance.AdvanceSubscriberSQN(context.Background(), imsi, "", "")
		}

		if err != nil {
			t.Fatalf("step %d: %v", i, err)
		}

		if creds.SequenceNumber != step.want {
			t.Fatalf("step %d: sequence number = %s, want %s", i, creds.SequenceNumber, step.want)
		}
	}
}

func TestAdvanceSubscriberIMSSQNMissingSubscriberIsNotFound(t *testing.T) {
	dbInstance := setupTestDB(t)

	_, err := dbInstance.AdvanceSubscriberIMSSQN(context.Background(), "001010000000999", "", "")
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAdvanceSubscriberIMSSQNResynchronises(t *testing.T) {
	dbInstance := setupTestDB(t)

	const (
		imsi = "001010000000044"
		k    = "465b5ce8b199b49faa5f0a2ee238a6bc"
		opc  = "cd63cb71954a9f4e48a5994e37a02baf"
	)

	profileID, _ := createPolicyDeps(t, dbInstance, t.Name())

	if err := dbInstance.CreateSubscriber(context.Background(), &db.Subscriber{
		Imsi:           imsi,
		PermanentKey:   k,
		Opc:            opc,
		SequenceNumber: "000000000020",
		ProfileID:      profileID,
	}); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}

	rand := make([]byte, 16)
	kb, _ := hex.DecodeString(k)
	opcb, _ := hex.DecodeString(opc)
	sqnMS, _ := hex.DecodeString("00000000105e")

	akStar := make([]byte, 6)
	if err := milenage.F2345(opcb, kb, rand, nil, nil, nil, nil, akStar); err != nil {
		t.Fatalf("f5*: %v", err)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	if err := milenage.F1(opcb, kb, rand, sqnMS, []byte{0, 0}, macA, macS); err != nil {
		t.Fatalf("f1*: %v", err)
	}

	auts := make([]byte, 0, 14)
	for i := range sqnMS {
		auts = append(auts, sqnMS[i]^akStar[i])
	}

	auts = append(auts, macS...)

	creds, err := dbInstance.AdvanceSubscriberIMSSQN(context.Background(), imsi, hex.EncodeToString(auts), hex.EncodeToString(rand))
	if err != nil {
		t.Fatalf("resynchronise: %v", err)
	}

	if creds.SequenceNumber != "00000000107f" {
		t.Fatalf("sequence number = %s, want 00000000107f", creds.SequenceNumber)
	}

	auts[13] ^= 0x01

	if _, err := dbInstance.AdvanceSubscriberIMSSQN(context.Background(), imsi, hex.EncodeToString(auts), hex.EncodeToString(rand)); err == nil {
		t.Fatal("an AUTS with a bad MAC-S was accepted")
	}

	sub, err := dbInstance.GetSubscriber(context.Background(), imsi)
	if err != nil {
		t.Fatalf("get subscriber: %v", err)
	}

	if sub.SequenceNumber != "00000000107f" {
		t.Fatalf("stored sequence number = %s after a refused resynchronisation, want 00000000107f", sub.SequenceNumber)
	}
}
