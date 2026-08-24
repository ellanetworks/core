// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/sqn"
)

type AdvanceSQNPayload struct {
	IMSI       string `json:"imsi"`
	ResyncAuts string `json:"resyncAuts,omitempty"`
	ResyncRand string `json:"resyncRand,omitempty"`
}

type AdvancedCredentials struct {
	PermanentKey   string `json:"permanentKey"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

type sqnCAS struct {
	IMSI     string `db:"imsi"`
	Expected string `db:"expected"`
	Next     string `db:"next"`
}

const sqnLockShards = 256

var sqnLocks [sqnLockShards]sync.Mutex

func lockSQN(imsi string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(imsi))

	mu := &sqnLocks[h.Sum32()%sqnLockShards]
	mu.Lock()

	return mu
}

const maxSQNCASAttempts = 16

func (db *Database) applyAdvanceSubscriberSQN(ctx context.Context, payload *AdvanceSQNPayload) (any, error) {
	defer lockSQN(payload.IMSI).Unlock()

	for range maxSQNCASAttempts {
		row := Subscriber{Imsi: payload.IMSI}

		if err := db.runner(ctx).Query(ctx, db.getSubscriberStmt, row).Get(&row); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("read subscriber %s: %w", payload.IMSI, err)
		}

		if row.PermanentKey == "" || row.Opc == "" {
			return nil, fmt.Errorf("subscriber %s missing key material", payload.IMSI)
		}

		next, err := sqn.Next(row.SequenceNumber, row.Opc, row.PermanentKey, payload.ResyncAuts, payload.ResyncRand)
		if err != nil {
			return nil, err
		}

		swap := sqnCAS{IMSI: payload.IMSI, Expected: row.SequenceNumber, Next: next}

		var outcome sqlair.Outcome

		if err := db.runner(ctx).Query(ctx, db.casSubscriberSqnNumStmt, swap).Get(&outcome); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}

		rowsAffected, err := outcome.Result().RowsAffected()
		if err != nil {
			return nil, fmt.Errorf("rows affected: %w", err)
		}

		if rowsAffected == 1 {
			return &AdvancedCredentials{
				PermanentKey:   row.PermanentKey,
				Opc:            row.Opc,
				SequenceNumber: next,
			}, nil
		}
	}

	return nil, fmt.Errorf("sequence number for subscriber %s contended beyond %d attempts", payload.IMSI, maxSQNCASAttempts)
}

func (db *Database) AdvanceSubscriberSQN(ctx context.Context, imsi, resyncAuts, resyncRand string) (*AdvancedCredentials, error) {
	creds, err := opAdvanceSubscriberSQN.Invoke(db, &AdvanceSQNPayload{
		IMSI:       imsi,
		ResyncAuts: resyncAuts,
		ResyncRand: resyncRand,
	})
	if err != nil {
		return nil, err
	}

	if creds == nil {
		return nil, fmt.Errorf("advance sequence number for subscriber %s: leader returned no credentials", imsi)
	}

	return creds, nil
}
