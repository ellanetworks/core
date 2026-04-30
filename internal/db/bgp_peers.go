package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const BGPPeersTableName = "bgp_peers"

const (
	listBGPPeersPagedStmt = "SELECT &BGPPeer.*, COUNT(*) OVER() AS &NumItems.count FROM %s ORDER BY id ASC LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	listAllBGPPeersStmt   = "SELECT &BGPPeer.* FROM %s ORDER BY id ASC"
	getBGPPeerStmt        = "SELECT &BGPPeer.* FROM %s WHERE id==$BGPPeer.id"
	createBGPPeerStmt     = "INSERT INTO %s (address, remoteAS, holdTime, password, description) VALUES ($BGPPeer.address, $BGPPeer.remoteAS, $BGPPeer.holdTime, $BGPPeer.password, $BGPPeer.description)"
	updateBGPPeerStmt     = "UPDATE %s SET address=$BGPPeer.address, remoteAS=$BGPPeer.remoteAS, holdTime=$BGPPeer.holdTime, password=$BGPPeer.password, description=$BGPPeer.description WHERE id==$BGPPeer.id"
	deleteBGPPeerStmt     = "DELETE FROM %s WHERE id==$BGPPeer.id"
	countBGPPeersStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

type BGPPeer struct {
	ID          int    `db:"id"`
	Address     string `db:"address"`
	RemoteAS    int    `db:"remoteAS"`
	HoldTime    int    `db:"holdTime"`
	Password    string `db:"password"` // stored in plaintext — required by GoBGP TCP MD5 API
	Description string `db:"description"`
}

func (db *Database) ListBGPPeersPage(ctx context.Context, page, perPage int) ([]BGPPeer, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", BGPPeersTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "select").Inc()

	var peers []BGPPeer

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	err := db.conn().Query(ctx, db.listBGPPeersStmt, args).GetAll(&peers, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountBGPPeers(ctx)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return peers, count, nil
}

func (db *Database) ListAllBGPPeers(ctx context.Context) ([]BGPPeer, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", BGPPeersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "select").Inc()

	var peers []BGPPeer

	err := db.conn().Query(ctx, db.listAllBGPPeersStmt).GetAll(&peers)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return peers, nil
}

func (db *Database) GetBGPPeer(ctx context.Context, id int) (*BGPPeer, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", BGPPeersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "select").Inc()

	row := BGPPeer{ID: id}

	err := db.conn().Query(ctx, db.getBGPPeerStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.RecordError(err)
			span.SetStatus(codes.Error, "not found")

			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreateBGPPeer(ctx context.Context, peer *BGPPeer) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", BGPPeersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "insert").Inc()

	result, err := db.applyCreateBGPPeer(ctx, peer)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	peer.ID = result.(int)

	db.publishOpTopics([]Topic{TopicBGPPeers}, 0)
	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateBGPPeer(ctx context.Context, peer *BGPPeer) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", BGPPeersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "update").Inc()

	_, err := db.applyUpdateBGPPeer(ctx, peer)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	db.publishOpTopics([]Topic{TopicBGPPeers}, 0)
	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteBGPPeer(ctx context.Context, id int) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", BGPPeersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "delete").Inc()

	_, err := db.applyDeleteBGPPeer(ctx, &intPayload{Value: id})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	db.publishOpTopics([]Topic{TopicBGPPeers}, 0)
	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountBGPPeers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", BGPPeersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", BGPPeersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(BGPPeersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(BGPPeersTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countBGPPeersStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
