// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

const BGPImportPrefixesTableName = "bgp_import_prefixes"

const (
	listImportPrefixesByPeerSQL   = "SELECT id, peerID, prefix, maxLength FROM " + BGPImportPrefixesTableName + " WHERE peerID==? ORDER BY id ASC"
	createImportPrefixSQL         = "INSERT INTO " + BGPImportPrefixesTableName + " (peerID, prefix, maxLength) VALUES (?, ?, ?)"
	deleteImportPrefixesByPeerSQL = "DELETE FROM " + BGPImportPrefixesTableName + " WHERE peerID==?"
)

type BGPImportPrefix struct {
	ID        int
	PeerID    int
	Prefix    string
	MaxLength int
}

func (db *Database) ListImportPrefixesByPeer(ctx context.Context, peerID int) (_ []BGPImportPrefix, err error) {
	ctx, finish := startDBQuery(ctx, BGPImportPrefixesTableName, "SELECT", attribute.Int("bgp.peer_id", peerID))
	defer func() { finish(err) }()

	prefixes, err := queryAll(ctx, db.reader(), listImportPrefixesByPeerSQL, scanBGPImportPrefix, peerID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return prefixes, nil
}

func (db *Database) SetImportPrefixesForPeer(ctx context.Context, peerID int, prefixes []BGPImportPrefix) (err error) {
	ctx, finish := startDBQuery(ctx, BGPImportPrefixesTableName, "REPLACE", attribute.Int("bgp.peer_id", peerID))
	defer func() { finish(err) }()

	tx, err := db.writer().BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, deleteImportPrefixesByPeerSQL, peerID); err != nil {
		return fmt.Errorf("delete existing prefixes: %w", err)
	}

	for _, prefix := range prefixes {
		prefix.PeerID = peerID

		if _, err := tx.ExecContext(ctx, createImportPrefixSQL, prefix.PeerID, prefix.Prefix, prefix.MaxLength); err != nil {
			return fmt.Errorf("insert prefix: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	db.publishOpTopics([]Topic{TopicBGPPeers})

	return nil
}

func scanBGPImportPrefix(row scanner) (BGPImportPrefix, error) {
	var v BGPImportPrefix

	if err := row.Scan(&v.ID, &v.PeerID, &v.Prefix, &v.MaxLength); err != nil {
		return BGPImportPrefix{}, err
	}

	return v, nil
}
