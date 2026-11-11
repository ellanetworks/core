// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

const BGPPeersTableName = "bgp_peers"

const (
	listBGPPeersPagedStmt = "SELECT id, address, remoteAS, holdTime, password, description, COUNT(*) OVER() FROM " + BGPPeersTableName + " ORDER BY id ASC LIMIT ? OFFSET ?"
	listAllBGPPeersStmt   = "SELECT id, address, remoteAS, holdTime, password, description FROM " + BGPPeersTableName + " ORDER BY id ASC"
	getBGPPeerStmt        = "SELECT id, address, remoteAS, holdTime, password, description FROM " + BGPPeersTableName + " WHERE id==?"
	createBGPPeerStmt     = "INSERT INTO " + BGPPeersTableName + " (address, remoteAS, holdTime, password, description) VALUES (?, ?, ?, ?, ?)"
	updateBGPPeerStmt     = "UPDATE " + BGPPeersTableName + " SET address=?, remoteAS=?, holdTime=?, password=?, description=? WHERE id==?"
	deleteBGPPeerStmt     = "DELETE FROM " + BGPPeersTableName + " WHERE id==?"
	countBGPPeersStmt     = "SELECT COUNT(*) FROM " + BGPPeersTableName
)

type BGPPeer struct {
	ID          int
	Address     string
	RemoteAS    int
	HoldTime    int
	Password    string // stored in plaintext — required by GoBGP TCP MD5 API
	Description string
}

func (db *Database) ListBGPPeersPage(ctx context.Context, page, perPage int) ([]BGPPeer, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+BGPPeersTableName+" (paged)", BGPPeersTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	peers, count, err := queryPage(ctx, db.reader(), listBGPPeersPagedStmt, scanBGPPeer, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(peers) == 0 {
		fallbackCount, countErr := db.CountBGPPeers(ctx)
		if countErr != nil {
			return nil, 0, nil
		}

		return nil, fallbackCount, nil
	}

	return peers, count, nil
}

func (db *Database) ListAllBGPPeers(ctx context.Context) ([]BGPPeer, error) {
	ctx, span, timer := startDBQuery(ctx, BGPPeersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	peers, err := queryAll(ctx, db.reader(), listAllBGPPeersStmt, scanBGPPeer)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return peers, nil
}

func (db *Database) GetBGPPeer(ctx context.Context, id int) (*BGPPeer, error) {
	ctx, span, timer := startDBQuery(ctx, BGPPeersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	row, err := scanBGPPeer(db.reader().QueryRowContext(ctx, getBGPPeerStmt, id))
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

func (db *Database) CreateBGPPeer(ctx context.Context, peer *BGPPeer) error {
	ctx, span, timer := startDBQuery(ctx, BGPPeersTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	result, err := db.applyCreateBGPPeer(ctx, peer)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	peer.ID = result.(int)

	db.publishOpTopics([]Topic{TopicBGPPeers})

	return nil
}

func (db *Database) UpdateBGPPeer(ctx context.Context, peer *BGPPeer) error {
	ctx, span, timer := startDBQuery(ctx, BGPPeersTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.applyUpdateBGPPeer(ctx, peer)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	db.publishOpTopics([]Topic{TopicBGPPeers})

	return nil
}

func (db *Database) DeleteBGPPeer(ctx context.Context, id int) error {
	ctx, span, timer := startDBQuery(ctx, BGPPeersTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.applyDeleteBGPPeer(ctx, &intPayload{Value: id})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	db.publishOpTopics([]Topic{TopicBGPPeers})

	return nil
}

func (db *Database) CountBGPPeers(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, BGPPeersTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countBGPPeersStmt)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func scanBGPPeer(row scanner) (BGPPeer, error) {
	var v BGPPeer

	if err := row.Scan(&v.ID, &v.Address, &v.RemoteAS, &v.HoldTime, &v.Password, &v.Description); err != nil {
		return BGPPeer{}, err
	}

	return v, nil
}
