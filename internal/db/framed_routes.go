// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const FramedRoutesTableName = "subscriber_framed_routes"

const (
	createFramedRouteStmt        = "INSERT INTO %s (id, imsi, dataNetworkID, prefix) VALUES ($SubscriberFramedRoute.id, $SubscriberFramedRoute.imsi, $SubscriberFramedRoute.dataNetworkID, $SubscriberFramedRoute.prefix)"
	deleteFramedRoutesByPairStmt = "DELETE FROM %s WHERE imsi==$SubscriberFramedRoute.imsi AND dataNetworkID==$SubscriberFramedRoute.dataNetworkID"
	listFramedRoutesByPairStmt   = "SELECT &SubscriberFramedRoute.* FROM %s WHERE imsi==$SubscriberFramedRoute.imsi AND dataNetworkID==$SubscriberFramedRoute.dataNetworkID ORDER BY prefix"
	listFramedRoutesByDNStmt     = "SELECT &SubscriberFramedRoute.* FROM %s WHERE dataNetworkID==$SubscriberFramedRoute.dataNetworkID ORDER BY imsi, prefix"
	listAllFramedRoutesStmt      = "SELECT &SubscriberFramedRoute.* FROM %s ORDER BY prefix"
)

type SubscriberFramedRoute struct {
	ID            string `db:"id"`            // UUIDv7
	Imsi          string `db:"imsi"`          // FK to subscribers.imsi
	DataNetworkID string `db:"dataNetworkID"` // FK to data_networks.id
	Prefix        string `db:"prefix"`        // normalized CIDR, netip.Prefix.Masked().String()
}

type framedRoutesPayload struct {
	Imsi          string   `json:"imsi"`
	DataNetworkID string   `json:"data_network_id"`
	Prefixes      []string `json:"prefixes"`
}

// ReplaceFramedRoutes atomically swaps a (subscriber, data network) pair's
// framed-route set (delete-then-insert); an empty set clears it. Prefixes are
// normalized so the UNIQUE(prefix) constraint and overlap checks see one form.
func (db *Database) ReplaceFramedRoutes(ctx context.Context, imsi, dataNetworkID string, prefixes []netip.Prefix) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "REPLACE", FramedRoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("REPLACE"),
			attribute.String("db.collection", FramedRoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FramedRoutesTableName, "replace"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FramedRoutesTableName, "replace").Inc()

	normalized := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		normalized = append(normalized, p.Masked().String())
	}

	_, err := opReplaceFramedRoutes.Invoke(db, &framedRoutesPayload{
		Imsi:          imsi,
		DataNetworkID: dataNetworkID,
		Prefixes:      normalized,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteFramedRoutes(ctx context.Context, imsi, dataNetworkID string) error {
	return db.ReplaceFramedRoutes(ctx, imsi, dataNetworkID, nil)
}

func (db *Database) applyReplaceFramedRoutes(ctx context.Context, payload *framedRoutesPayload) (any, error) {
	del := SubscriberFramedRoute{Imsi: payload.Imsi, DataNetworkID: payload.DataNetworkID}
	if err := db.runner(ctx).Query(ctx, db.deleteFramedRoutesByPairStmt, del).Run(); err != nil {
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

		if err := db.runner(ctx).Query(ctx, db.createFramedRouteStmt, fr).Run(); err != nil {
			if isUniqueNameError(err) {
				return nil, ErrAlreadyExists
			}

			return nil, fmt.Errorf("query failed: %w", err)
		}
	}

	return nil, nil
}

func (db *Database) ListFramedRoutesBySubscriberDataNetwork(ctx context.Context, imsi, dataNetworkID string) ([]SubscriberFramedRoute, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", FramedRoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", FramedRoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FramedRoutesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FramedRoutesTableName, "select").Inc()

	var routes []SubscriberFramedRoute

	params := SubscriberFramedRoute{Imsi: imsi, DataNetworkID: dataNetworkID}

	err := db.conn().Query(ctx, db.listFramedRoutesByPairStmt, params).GetAll(&routes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return []SubscriberFramedRoute{}, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return routes, nil
}

func (db *Database) ListFramedRoutesByDataNetwork(ctx context.Context, dataNetworkID string) ([]SubscriberFramedRoute, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", FramedRoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", FramedRoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FramedRoutesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FramedRoutesTableName, "select").Inc()

	var routes []SubscriberFramedRoute

	params := SubscriberFramedRoute{DataNetworkID: dataNetworkID}

	err := db.conn().Query(ctx, db.listFramedRoutesByDNStmt, params).GetAll(&routes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return []SubscriberFramedRoute{}, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return routes, nil
}

func (db *Database) ListAllFramedRoutes(ctx context.Context) ([]SubscriberFramedRoute, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", FramedRoutesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", FramedRoutesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(FramedRoutesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(FramedRoutesTableName, "select").Inc()

	var routes []SubscriberFramedRoute

	err := db.conn().Query(ctx, db.listAllFramedRoutesStmt).GetAll(&routes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return []SubscriberFramedRoute{}, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return routes, nil
}
