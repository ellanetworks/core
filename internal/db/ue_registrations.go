// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const UERegistrationsTableName = "ue_registrations"

const ueRegistrationsSchema = 21

const (
	UERegistrationTypeMME           = "mme"
	UERegistrationTypeAMF3GPPAccess = "amf-3gpp-access"
)

const (
	upsertUERegistrationSQL          = "INSERT INTO " + UERegistrationsTableName + " (imsi, type, nodeID, purged, version) VALUES (?, ?, ?, 0, ?) ON CONFLICT(imsi, type) DO UPDATE SET nodeID=excluded.nodeID, purged=0, version=excluded.version"
	purgeUERegistrationSQL           = "UPDATE " + UERegistrationsTableName + " SET purged=1, version=? WHERE imsi==? AND type==? AND nodeID==? AND purged==0"
	cancelUERegistrationSQL          = "UPDATE " + UERegistrationsTableName + " SET purged=1, version=? WHERE imsi==? AND type==? AND purged==0"
	purgeUERegistrationsByNodeSQL    = "UPDATE " + UERegistrationsTableName + " SET purged=1, version=? WHERE nodeID==? AND purged==0"
	getUERegistrationSQL             = "SELECT imsi, type, nodeID, purged, version FROM " + UERegistrationsTableName + " WHERE imsi==? AND type==?"
	listAllUERegistrationsSQL        = "SELECT imsi, type, nodeID, purged, version FROM " + UERegistrationsTableName + " ORDER BY imsi ASC"
	listActiveUERegistrationsSQL     = "SELECT imsi, type, nodeID, purged, version FROM " + UERegistrationsTableName + " WHERE purged==0 AND imsi IN (SELECT value FROM json_each(?)) ORDER BY imsi ASC"
	maxUERegistrationVersionSQL      = "SELECT COALESCE(MAX(version), 0) FROM " + UERegistrationsTableName
	resetUERegistrationsSQL          = "UPDATE %s SET purged=0, version=0"
	ueRegistrationsTableExistsSQLFmt = "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s'"
)

type UERegistration struct {
	Imsi    string
	Type    string
	NodeID  string
	Purged  bool
	Version int64
}

type registerUEPayload struct {
	Imsi   string `json:"imsi"`
	Type   string `json:"type"`
	NodeID string `json:"node_id"`
	Cancel string `json:"cancel,omitempty"`
}

type purgeUERegistrationPayload struct {
	Imsi   string `json:"imsi"`
	Type   string `json:"type"`
	NodeID string `json:"node_id"`
}

func IsValidUERegistrationType(t string) bool {
	switch t {
	case UERegistrationTypeMME, UERegistrationTypeAMF3GPPAccess:
		return true
	}

	return false
}

func validateRegisterUE(regType, nodeID, cancel string) error {
	if !IsValidUERegistrationType(regType) {
		return fmt.Errorf("invalid UE registration type %q", regType)
	}

	if cancel != "" && (!IsValidUERegistrationType(cancel) || cancel == regType) {
		return fmt.Errorf("invalid UE registration type to cancel %q", cancel)
	}

	if nodeID == "" {
		return fmt.Errorf("node ID is required")
	}

	return nil
}

func (db *Database) RegisterUE(ctx context.Context, imsi, regType, nodeID, cancel string) (_ int64, err error) {
	if err := validateRegisterUE(regType, nodeID, cancel); err != nil {
		return 0, err
	}

	ctx, finish := startDBQuery(ctx, UERegistrationsTableName, "UPSERT")
	defer func() { finish(err) }()

	version, err := opRegisterUE.Invoke(ctx, db, &registerUEPayload{
		Imsi:   imsi,
		Type:   regType,
		NodeID: nodeID,
		Cancel: cancel,
	})
	if err != nil {
		return 0, err
	}

	return version, nil
}

func (db *Database) PurgeUERegistration(ctx context.Context, imsi, regType, nodeID string) (err error) {
	if !IsValidUERegistrationType(regType) {
		return fmt.Errorf("invalid UE registration type %q", regType)
	}

	ctx, finish := startDBQuery(ctx, UERegistrationsTableName, "UPDATE")
	defer func() { finish(err) }()

	_, err = opPurgeUERegistration.Invoke(ctx, db, &purgeUERegistrationPayload{
		Imsi:   imsi,
		Type:   regType,
		NodeID: nodeID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) GetUERegistration(ctx context.Context, imsi, regType string) (_ *UERegistration, err error) {
	ctx, finish := startDBQuery(ctx, UERegistrationsTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanUERegistration(db.reader().QueryRowContext(ctx, getUERegistrationSQL, imsi, regType))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) ListAllUERegistrations(ctx context.Context) (_ []UERegistration, err error) {
	ctx, finish := startDBQuery(ctx, UERegistrationsTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil, nil
	}

	regs, err := queryAll(ctx, db.reader(), listAllUERegistrationsSQL, scanUERegistration)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return regs, nil
}

func (db *Database) ListActiveUERegistrations(ctx context.Context, imsis []string) (_ []UERegistration, err error) {
	ctx, finish := startDBQuery(ctx, UERegistrationsTableName, "SELECT")
	defer func() { finish(err) }()

	if len(imsis) == 0 || !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil, nil
	}

	regs, err := queryAll(ctx, db.reader(), listActiveUERegistrationsSQL, scanUERegistration, jsonArray(imsis))
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return regs, nil
}

func (db *Database) nextUERegistrationVersion(ctx context.Context) (int64, error) {
	var version int64

	if err := db.runner(ctx).QueryRowContext(ctx, maxUERegistrationVersionSQL).Scan(&version); err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	next := version + 1

	if db.raftManager != nil {
		next = max(next, int64(db.raftManager.AppliedIndex())+1)
	}

	return next, nil
}

func (db *Database) applyRegisterUE(ctx context.Context, p *registerUEPayload) (any, error) {
	known, err := db.imsiKnown(ctx, p.Imsi)
	if err != nil {
		return nil, err
	}

	if !known {
		return nil, ErrNotFound
	}

	existing, err := scanUERegistration(db.runner(ctx).QueryRowContext(ctx, getUERegistrationSQL, p.Imsi, p.Type))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	unchanged := err == nil && existing.NodeID == p.NodeID && !existing.Purged && existing.Version != 0

	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return nil, err
	}

	version := existing.Version

	if !unchanged {
		version = next

		row := UERegistration{
			Imsi:    p.Imsi,
			Type:    p.Type,
			NodeID:  p.NodeID,
			Version: next,
		}

		if _, err := db.runner(ctx).ExecContext(ctx, upsertUERegistrationSQL, row.Imsi, row.Type, row.NodeID, row.Version); err != nil {
			if isForeignKeyError(err) {
				return nil, ErrNotFound
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	if p.Cancel != "" {
		if _, err := db.runner(ctx).ExecContext(ctx, cancelUERegistrationSQL, next, p.Imsi, p.Cancel); err != nil {
			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return version, nil
}

func (db *Database) applyPurgeUERegistration(ctx context.Context, p *purgeUERegistrationPayload) (any, error) {
	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return nil, err
	}

	row := UERegistration{Imsi: p.Imsi, Type: p.Type, NodeID: p.NodeID, Version: next}

	if _, err := db.runner(ctx).ExecContext(ctx, purgeUERegistrationSQL, row.Version, row.Imsi, row.Type, row.NodeID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	return nil, db.deleteVisitorIfUnregistered(ctx, p.Imsi)
}

func (db *Database) purgeUERegistrationsByNode(ctx context.Context, nodeID string) error {
	if !db.appliedSchemaAtLeast(ctx, ueRegistrationsSchema) {
		return nil
	}

	next, err := db.nextUERegistrationVersion(ctx)
	if err != nil {
		return err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, purgeUERegistrationsByNodeSQL, next, nodeID); err != nil {
		return fmt.Errorf("purge UE registrations of node %s: %w", nodeID, err)
	}

	return nil
}

func resetUERegistrationsInRestoredDB(ctx context.Context, dbPath string) error {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("open restored db: %w", err)
	}

	defer func() { _ = conn.Close() }()

	var count int
	if err := conn.QueryRowContext(ctx, fmt.Sprintf(ueRegistrationsTableExistsSQLFmt, UERegistrationsTableName)).Scan(&count); err != nil {
		return fmt.Errorf("look up %s: %w", UERegistrationsTableName, err)
	}

	if count == 0 {
		return nil
	}

	if _, err := conn.ExecContext(ctx, fmt.Sprintf(resetUERegistrationsSQL, UERegistrationsTableName)); err != nil {
		return fmt.Errorf("reset %s: %w", UERegistrationsTableName, err)
	}

	return nil
}

func scanUERegistration(row scanner) (UERegistration, error) {
	var v UERegistration

	if err := row.Scan(&v.Imsi, &v.Type, &v.NodeID, &v.Purged, &v.Version); err != nil {
		return UERegistration{}, err
	}

	return v, nil
}
