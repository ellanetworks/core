// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
)

const FramedRoutesTableName = "subscriber_framed_routes"

const (
	createFramedRouteSQL        = "INSERT INTO " + FramedRoutesTableName + " (id, imsi, dataNetworkID, prefix) VALUES (?, ?, ?, ?)"
	deleteFramedRoutesByPairSQL = "DELETE FROM " + FramedRoutesTableName + " WHERE imsi==? AND dataNetworkID==?"
	listFramedRoutesByPairSQL   = "SELECT id, imsi, dataNetworkID, prefix FROM " + FramedRoutesTableName + " WHERE imsi==? AND dataNetworkID==? ORDER BY prefix"
	listFramedRoutesByDNSQL     = "SELECT id, imsi, dataNetworkID, prefix FROM " + FramedRoutesTableName + " WHERE dataNetworkID==? ORDER BY imsi, prefix"
	listAllFramedRoutesSQL      = "SELECT id, imsi, dataNetworkID, prefix FROM " + FramedRoutesTableName + " ORDER BY prefix"
)

type SubscriberFramedRoute struct {
	ID            string // UUIDv7
	Imsi          string // FK to subscribers.imsi
	DataNetworkID string // FK to data_networks.id
	Prefix        string // normalized CIDR, netip.Prefix.Masked().String()
}

type framedRoutesPayload struct {
	Imsi          string   `json:"imsi"`
	DataNetworkID string   `json:"data_network_id"`
	Prefixes      []string `json:"prefixes"`
}

// ReplaceFramedRoutes atomically swaps a (subscriber, data network) pair's
// framed-route set (delete-then-insert); an empty set clears it. Prefixes are
// normalized so the UNIQUE(prefix) constraint and overlap checks see one form.
func (db *Database) ReplaceFramedRoutes(ctx context.Context, imsi, dataNetworkID string, prefixes []netip.Prefix) (err error) {
	ctx, finish := startDBQuery(ctx, FramedRoutesTableName, "REPLACE")
	defer func() { finish(err) }()

	normalized := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		normalized = append(normalized, p.Masked().String())
	}

	_, err = opReplaceFramedRoutes.Invoke(ctx, db, &framedRoutesPayload{
		Imsi:          imsi,
		DataNetworkID: dataNetworkID,
		Prefixes:      normalized,
	})
	if err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteFramedRoutes(ctx context.Context, imsi, dataNetworkID string) error {
	return db.ReplaceFramedRoutes(ctx, imsi, dataNetworkID, nil)
}

func (db *Database) applyReplaceFramedRoutes(ctx context.Context, payload *framedRoutesPayload) (any, error) {
	del := SubscriberFramedRoute{Imsi: payload.Imsi, DataNetworkID: payload.DataNetworkID}
	if _, err := db.runner(ctx).ExecContext(ctx, deleteFramedRoutesByPairSQL, del.Imsi, del.DataNetworkID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	for _, prefix := range payload.Prefixes {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate framed route id: %w", err)
		}

		fr := SubscriberFramedRoute{
			ID:            id.String(),
			Imsi:          payload.Imsi,
			DataNetworkID: payload.DataNetworkID,
			Prefix:        prefix,
		}

		if _, err := db.runner(ctx).ExecContext(ctx, createFramedRouteSQL, fr.ID, fr.Imsi, fr.DataNetworkID, fr.Prefix); err != nil {
			if isUniqueNameError(err) {
				return nil, ErrAlreadyExists
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) ListFramedRoutesBySubscriberDataNetwork(ctx context.Context, imsi, dataNetworkID string) (_ []SubscriberFramedRoute, err error) {
	ctx, finish := startDBQuery(ctx, FramedRoutesTableName, "SELECT")
	defer func() { finish(err) }()

	routes, err := queryAll(ctx, db.reader(), listFramedRoutesByPairSQL, scanSubscriberFramedRoute, imsi, dataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if routes == nil {
		return []SubscriberFramedRoute{}, nil
	}

	return routes, nil
}

func (db *Database) ListFramedRoutesByDataNetwork(ctx context.Context, dataNetworkID string) (_ []SubscriberFramedRoute, err error) {
	ctx, finish := startDBQuery(ctx, FramedRoutesTableName, "SELECT")
	defer func() { finish(err) }()

	routes, err := queryAll(ctx, db.reader(), listFramedRoutesByDNSQL, scanSubscriberFramedRoute, dataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if routes == nil {
		return []SubscriberFramedRoute{}, nil
	}

	return routes, nil
}

func (db *Database) ListAllFramedRoutes(ctx context.Context) (_ []SubscriberFramedRoute, err error) {
	ctx, finish := startDBQuery(ctx, FramedRoutesTableName, "SELECT")
	defer func() { finish(err) }()

	routes, err := queryAll(ctx, db.reader(), listAllFramedRoutesSQL, scanSubscriberFramedRoute)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if routes == nil {
		return []SubscriberFramedRoute{}, nil
	}

	return routes, nil
}

func scanSubscriberFramedRoute(row scanner) (SubscriberFramedRoute, error) {
	var v SubscriberFramedRoute

	if err := row.Scan(&v.ID, &v.Imsi, &v.DataNetworkID, &v.Prefix); err != nil {
		return SubscriberFramedRoute{}, err
	}

	return v, nil
}
