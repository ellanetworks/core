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

	"github.com/ellanetworks/core/internal/sqn"
)

type AdvanceSQNPayload struct {
	IMSI       string `json:"imsi"`
	ResyncAuts string `json:"resyncAuts,omitempty"`
	ResyncRand string `json:"resyncRand,omitempty"`
	Domain     string `json:"domain,omitempty"`
}

const SQNDomainIMS = "ims"

type AdvancedCredentials struct {
	PermanentKey   string `json:"permanentKey"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
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
		row, err := scanSubscriber(db.runner(ctx).QueryRowContext(ctx, getSubscriberStmt, payload.IMSI))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("read subscriber %s: %w", payload.IMSI, err)
		}

		if row.PermanentKey == "" || row.Opc == "" {
			return nil, fmt.Errorf("subscriber %s missing key material", payload.IMSI)
		}

		next, err := nextSQN(row, payload)
		if err != nil {
			return nil, err
		}

		res, err := db.runner(ctx).ExecContext(ctx, casSubscriberSeqNumStmt, next, payload.IMSI, row.SequenceNumber)
		if err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}

		rowsAffected, err := res.RowsAffected()
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

func nextSQN(row Subscriber, payload *AdvanceSQNPayload) (string, error) {
	switch payload.Domain {
	case "":
		return sqn.Next(row.SequenceNumber, row.Opc, row.PermanentKey, payload.ResyncAuts, payload.ResyncRand)
	case SQNDomainIMS:
		if payload.ResyncAuts != "" {
			return sqn.NextIMSResync(row.SequenceNumber, row.Opc, row.PermanentKey, payload.ResyncAuts, payload.ResyncRand)
		}

		return sqn.NextIMS(row.SequenceNumber)
	default:
		return "", fmt.Errorf("unknown sequence number domain %q", payload.Domain)
	}
}

func (db *Database) AdvanceSubscriberSQN(ctx context.Context, imsi, resyncAuts, resyncRand string) (*AdvancedCredentials, error) {
	return db.advanceSubscriberSQN(ctx, &AdvanceSQNPayload{IMSI: imsi, ResyncAuts: resyncAuts, ResyncRand: resyncRand})
}

func (db *Database) AdvanceSubscriberIMSSQN(ctx context.Context, imsi, resyncAuts, resyncRand string) (*AdvancedCredentials, error) {
	return db.advanceSubscriberSQN(ctx, &AdvanceSQNPayload{IMSI: imsi, ResyncAuts: resyncAuts, ResyncRand: resyncRand, Domain: SQNDomainIMS})
}

func (db *Database) advanceSubscriberSQN(ctx context.Context, payload *AdvanceSQNPayload) (*AdvancedCredentials, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "UPDATE "+SubscribersTableName+" (advance sqn)", SubscribersTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	creds, err := opAdvanceSubscriberSQN.Invoke(ctx, db, payload)
	if err != nil {
		recordSpanError(span, err)

		return nil, err
	}

	if creds == nil {
		err = fmt.Errorf("advance sequence number for subscriber %s: leader returned no credentials", payload.IMSI)
		recordSpanError(span, err)

		return nil, err
	}

	return creds, nil
}
