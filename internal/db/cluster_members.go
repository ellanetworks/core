// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

const ClusterMembersTableName = "cluster_members"

const (
	DrainStateActive   = "active"
	DrainStateDraining = "draining"
	DrainStateDrained  = "drained"
)

const clusterMemberIdentitySchema = 20

func (db *Database) requireIdentitySchemaFor(ctx context.Context, nodeID string) error {
	if db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		return nil
	}

	if _, ok := pki.LegacyNodeID(nodeID); ok {
		return nil
	}

	return ErrMigrationPending
}

const clusterMemberColumnsPreV20 = "nodeID, apiAddress, binaryVersion, drainState, drainUpdatedAt"

const (
	listClusterMembersStmt        = "SELECT nodeID, amfPointer, displayName, apiAddress, binaryVersion, drainState, drainUpdatedAt FROM " + ClusterMembersTableName + " ORDER BY nodeID ASC"
	listClusterMembersPreV20Stmt  = "SELECT " + clusterMemberColumnsPreV20 + " FROM " + ClusterMembersTableName + " ORDER BY nodeID ASC"
	getClusterMemberStmt          = "SELECT nodeID, amfPointer, displayName, apiAddress, binaryVersion, drainState, drainUpdatedAt FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	getClusterMemberPreV20Stmt    = "SELECT " + clusterMemberColumnsPreV20 + " FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	upsertClusterMemberStmt       = "INSERT INTO " + ClusterMembersTableName + " (nodeID, amfPointer, displayName, apiAddress, binaryVersion) VALUES (?, ?, ?, ?, ?) ON CONFLICT(nodeID) DO UPDATE SET amfPointer=excluded.amfPointer, apiAddress=?, binaryVersion=?"
	upsertClusterMemberPreV20Stmt = "INSERT INTO " + ClusterMembersTableName + " (nodeID, raftAddress, apiAddress, binaryVersion) VALUES (?, '', ?, ?) ON CONFLICT(nodeID) DO UPDATE SET apiAddress=?, binaryVersion=?"
	deleteClusterMemberStmt       = "DELETE FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	countClusterMembersStmt       = "SELECT COUNT(*) FROM " + ClusterMembersTableName
	getDrainStateStmt             = "SELECT drainState FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	setDrainStateStmt             = "UPDATE " + ClusterMembersTableName + " SET drainState=?, drainUpdatedAt=? WHERE nodeID==?"
	setDisplayNameStmt            = "UPDATE " + ClusterMembersTableName + " SET displayName=? WHERE nodeID==?"
)

const (
	DefaultAMFPointer = 1
	MaxAMFPointer     = 63
)

type ClusterMember struct {
	NodeID         string
	AMFPointer     int
	DisplayName    string
	APIAddress     string
	BinaryVersion  string
	DrainState     string
	DrainUpdatedAt int64
}

func IsValidDrainState(s string) bool {
	switch s {
	case DrainStateActive, DrainStateDraining, DrainStateDrained:
		return true
	}

	return false
}

func (db *Database) ListClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := listClusterMembersStmt, scanClusterMember
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		query, scan = listClusterMembersPreV20Stmt, scanClusterMemberPreV20
	}

	members, err := queryAll(ctx, db.reader(), query, scan)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return members, nil
}

func (db *Database) GetClusterMember(ctx context.Context, nodeID string) (*ClusterMember, error) {
	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := getClusterMemberStmt, scanClusterMember
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		query, scan = getClusterMemberPreV20Stmt, scanClusterMemberPreV20
	}

	row, err := scan(db.reader().QueryRowContext(ctx, query, nodeID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			recordSpanError(span, err)

			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) UpsertClusterMember(ctx context.Context, member *ClusterMember) error {
	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "UPSERT", "upsert")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opUpsertClusterMember.Invoke(ctx, db, member)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteClusterMember(ctx context.Context, nodeID string) error {
	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeleteClusterMember.Invoke(ctx, db, &nodeIDPayload{Value: pki.NodeID(nodeID)})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func normalizeDrainState(s string) string {
	if s == "" {
		return DrainStateActive
	}

	return s
}

func drainTransitionAllowed(current string, target string) bool {
	switch target {
	case DrainStateDraining:
		return normalizeDrainState(current) == DrainStateActive
	case DrainStateDrained:
		return normalizeDrainState(current) == DrainStateDraining
	case DrainStateActive:
		return normalizeDrainState(current) != DrainStateActive
	default:
		return false
	}
}

func (db *Database) SetDrainState(ctx context.Context, nodeID string, state string) (string, error) {
	if !IsValidDrainState(state) {
		return "", fmt.Errorf("invalid drain state %q", state)
	}

	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	member := &ClusterMember{
		NodeID:         nodeID,
		DrainState:     state,
		DrainUpdatedAt: time.Now().Unix(),
	}

	effective, err := opSetDrainState.Invoke(ctx, db, member)
	if err != nil {
		recordSpanError(span, err)

		return "", err
	}

	return effective, nil
}

func (db *Database) CountClusterMembers(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countClusterMembersStmt)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// MaxDisplayNameLength bounds the operator-facing name.
const MaxDisplayNameLength = 63

// SetDisplayName sets a cluster member's operator-facing name. The name
// is free text; lookups and destructive operations take the identity.
func (db *Database) SetDisplayName(ctx context.Context, nodeID string, name string) error {
	if len(name) > MaxDisplayNameLength {
		return fmt.Errorf("display name is %d bytes, over the %d-byte limit", len(name), MaxDisplayNameLength)
	}

	ctx, span, timer := startDBQuery(ctx, ClusterMembersTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	member := &ClusterMember{NodeID: nodeID, DisplayName: name}

	if _, err := opSetDisplayName.Invoke(ctx, db, member); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func scanClusterMember(row scanner) (ClusterMember, error) {
	var v ClusterMember

	if err := row.Scan(&v.NodeID, &v.AMFPointer, &v.DisplayName, &v.APIAddress, &v.BinaryVersion, &v.DrainState, &v.DrainUpdatedAt); err != nil {
		return ClusterMember{}, err
	}

	return v, nil
}

func scanClusterMemberPreV20(row scanner) (ClusterMember, error) {
	var v ClusterMember

	if err := row.Scan(&v.NodeID, &v.APIAddress, &v.BinaryVersion, &v.DrainState, &v.DrainUpdatedAt); err != nil {
		return ClusterMember{}, err
	}

	return v, nil
}
