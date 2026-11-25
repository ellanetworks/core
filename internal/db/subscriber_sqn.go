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
	IMSI        string `json:"imsi"`
	ResyncAuts  string `json:"resyncAuts,omitempty"`
	ResyncRand  string `json:"resyncRand,omitempty"`
	ResyncSQNms string `json:"resyncSqnMs,omitempty"`
	Domain      string `json:"domain,omitempty"`
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
	if _, ok := applyInfoFrom(ctx); ok {
		creds, advanced, err := db.casSubscriberSQN(ctx, payload)
		if err != nil {
			return nil, err
		}

		if !advanced {
			return nil, fmt.Errorf("sequence number for subscriber %s changed during apply", payload.IMSI)
		}

		return creds, nil
	}

	defer lockSQN(payload.IMSI).Unlock()

	for range maxSQNCASAttempts {
		creds, advanced, err := db.casSubscriberSQN(ctx, payload)
		if err != nil {
			return nil, err
		}

		if advanced {
			return creds, nil
		}
	}

	return nil, fmt.Errorf("sequence number for subscriber %s contended beyond %d attempts", payload.IMSI, maxSQNCASAttempts)
}

func (db *Database) casSubscriberSQN(ctx context.Context, payload *AdvanceSQNPayload) (*AdvancedCredentials, bool, error) {
	row, err := scanSubscriber(db.runner(ctx).QueryRowContext(ctx, getSubscriberSQL, payload.IMSI))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrNotFound
		}

		return nil, false, fmt.Errorf("read subscriber %s: %w", payload.IMSI, err)
	}

	if row.PermanentKey == "" || row.Opc == "" {
		return nil, false, invalidCommand(fmt.Errorf("subscriber %s missing key material", payload.IMSI))
	}

	next, err := nextSQN(row, payload)
	if err != nil {
		return nil, false, invalidCommand(err)
	}

	res, err := db.runner(ctx).ExecContext(ctx, casSubscriberSeqNumSQL, next, payload.IMSI, row.SequenceNumber)
	if err != nil {
		return nil, false, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected != 1 {
		return nil, false, nil
	}

	return &AdvancedCredentials{
		PermanentKey:   row.PermanentKey,
		Opc:            row.Opc,
		SequenceNumber: next,
	}, true, nil
}

func nextSQN(row Subscriber, payload *AdvanceSQNPayload) (string, error) {
	switch payload.Domain {
	case "":
		if payload.ResyncSQNms != "" {
			return sqn.NextFromResync(payload.ResyncSQNms)
		}

		return sqn.Next(row.SequenceNumber, row.Opc, row.PermanentKey, payload.ResyncAuts, payload.ResyncRand)
	case SQNDomainIMS:
		if payload.ResyncSQNms != "" {
			return sqn.NextIMSFromResync(row.SequenceNumber, payload.ResyncSQNms)
		}

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

func (db *Database) advanceSubscriberSQN(ctx context.Context, payload *AdvanceSQNPayload) (_ *AdvancedCredentials, err error) {
	ctx, finish := startDBQueryNamed(ctx, "UPDATE "+SubscribersTableName+" (advance sqn)", SubscribersTableName, "UPDATE")
	defer func() { finish(err) }()

	if err := db.verifySQNResync(ctx, payload); err != nil {
		return nil, err
	}

	creds, err := opAdvanceSubscriberSQN.Invoke(ctx, db, payload)
	if err != nil {
		return nil, err
	}

	if creds == nil {
		err = fmt.Errorf("advance sequence number for subscriber %s: leader returned no credentials", payload.IMSI)
		return nil, err
	}

	return creds, nil
}

func (db *Database) verifySQNResync(ctx context.Context, payload *AdvanceSQNPayload) error {
	if payload.ResyncAuts == "" {
		return nil
	}

	row, err := scanSubscriber(db.reader().QueryRowContext(ctx, getSubscriberSQL, payload.IMSI))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}

		return fmt.Errorf("read subscriber %s: %w", payload.IMSI, err)
	}

	if row.PermanentKey == "" || row.Opc == "" {
		return invalidCommand(fmt.Errorf("subscriber %s missing key material", payload.IMSI))
	}

	sqnMS, err := sqn.ResyncHex(row.Opc, row.PermanentKey, payload.ResyncAuts, payload.ResyncRand)
	if err != nil {
		return invalidCommand(err)
	}

	payload.ResyncSQNms = sqnMS
	payload.ResyncAuts = ""
	payload.ResyncRand = ""

	return nil
}
