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
	listClusterMembersSQL        = "SELECT nodeID, amfPointer, displayName, apiAddress, binaryVersion, drainState, drainUpdatedAt FROM " + ClusterMembersTableName + " ORDER BY nodeID ASC"
	listClusterMembersPreV20SQL  = "SELECT " + clusterMemberColumnsPreV20 + " FROM " + ClusterMembersTableName + " ORDER BY nodeID ASC"
	getClusterMemberSQL          = "SELECT nodeID, amfPointer, displayName, apiAddress, binaryVersion, drainState, drainUpdatedAt FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	getClusterMemberPreV20SQL    = "SELECT " + clusterMemberColumnsPreV20 + " FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	upsertClusterMemberSQL       = "INSERT INTO " + ClusterMembersTableName + " (nodeID, amfPointer, displayName, apiAddress, binaryVersion) VALUES (?, ?, ?, ?, ?) ON CONFLICT(nodeID) DO UPDATE SET amfPointer=excluded.amfPointer, apiAddress=?, binaryVersion=?"
	upsertClusterMemberPreV20SQL = "INSERT INTO " + ClusterMembersTableName + " (nodeID, raftAddress, apiAddress, binaryVersion) VALUES (?, '', ?, ?) ON CONFLICT(nodeID) DO UPDATE SET apiAddress=?, binaryVersion=?"
	deleteClusterMemberSQL       = "DELETE FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	countClusterMembersSQL       = "SELECT COUNT(*) FROM " + ClusterMembersTableName
	getDrainStateSQL             = "SELECT drainState FROM " + ClusterMembersTableName + " WHERE nodeID==?"
	setDrainStateSQL             = "UPDATE " + ClusterMembersTableName + " SET drainState=?, drainUpdatedAt=? WHERE nodeID==?"
	setDisplayNameSQL            = "UPDATE " + ClusterMembersTableName + " SET displayName=? WHERE nodeID==?"
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

func (db *Database) ListClusterMembers(ctx context.Context) (_ []ClusterMember, err error) {
	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "SELECT")
	defer func() { finish(err) }()

	query, scan := listClusterMembersSQL, scanClusterMember
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		query, scan = listClusterMembersPreV20SQL, scanClusterMemberPreV20
	}

	members, err := queryAll(ctx, db.reader(), query, scan)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return members, nil
}

func (db *Database) GetClusterMember(ctx context.Context, nodeID string) (_ *ClusterMember, err error) {
	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "SELECT")
	defer func() { finish(err) }()

	query, scan := getClusterMemberSQL, scanClusterMember
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		query, scan = getClusterMemberPreV20SQL, scanClusterMemberPreV20
	}

	row, err := scan(db.reader().QueryRowContext(ctx, query, nodeID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) UpsertClusterMember(ctx context.Context, member *ClusterMember) (err error) {
	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "UPSERT")
	defer func() { finish(err) }()

	_, err = opUpsertClusterMember.Invoke(ctx, db, member)
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteClusterMember(ctx context.Context, nodeID string) (err error) {
	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteClusterMember.Invoke(ctx, db, &nodeIDPayload{Value: pki.NodeID(nodeID)})
	if err != nil {
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

func (db *Database) SetDrainState(ctx context.Context, nodeID string, state string) (_ string, err error) {
	if !IsValidDrainState(state) {
		return "", fmt.Errorf("invalid drain state %q", state)
	}

	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "UPDATE")
	defer func() { finish(err) }()

	member := &ClusterMember{
		NodeID:         nodeID,
		DrainState:     state,
		DrainUpdatedAt: time.Now().Unix(),
	}

	effective, err := opSetDrainState.Invoke(ctx, db, member)
	if err != nil {
		return "", err
	}

	return effective, nil
}

func (db *Database) CountClusterMembers(ctx context.Context) (_ int, err error) {
	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "SELECT")
	defer func() { finish(err) }()

	count, err := queryCount(ctx, db.reader(), countClusterMembersSQL)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

// MaxDisplayNameLength bounds the operator-facing name.
const MaxDisplayNameLength = 63

// SetDisplayName sets a cluster member's operator-facing name. The name
// is free text; lookups and destructive operations take the identity.
func (db *Database) SetDisplayName(ctx context.Context, nodeID string, name string) (err error) {
	if len(name) > MaxDisplayNameLength {
		return fmt.Errorf("display name is %d bytes, over the %d-byte limit", len(name), MaxDisplayNameLength)
	}

	ctx, finish := startDBQuery(ctx, ClusterMembersTableName, "UPDATE")
	defer func() { finish(err) }()

	member := &ClusterMember{NodeID: nodeID, DisplayName: name}

	if _, err := opSetDisplayName.Invoke(ctx, db, member); err != nil {
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
