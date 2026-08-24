// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
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
