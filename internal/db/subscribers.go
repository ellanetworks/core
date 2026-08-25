// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SubscribersTableName = "subscribers"

const (
	getSubscriberStmt         = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	createSubscriberStmt      = "INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, profileID) VALUES ($Subscriber.id, $Subscriber.imsi, $Subscriber.sequenceNumber, $Subscriber.permanentKey, $Subscriber.opc, $Subscriber.profileID)"
	editSubscriberProfileStmt = "UPDATE %s SET profileID=$Subscriber.profileID WHERE imsi==$Subscriber.imsi"
	editSubscriberSeqNumStmt  = "UPDATE %s SET sequenceNumber=$Subscriber.sequenceNumber WHERE imsi==$Subscriber.imsi"
	casSubscriberSeqNumStmt   = "UPDATE %s SET sequenceNumber=$sqnCAS.next WHERE imsi==$sqnCAS.imsi AND sequenceNumber==$sqnCAS.expected"
	deleteSubscriberStmt      = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
	countSubscribersStmt      = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

const subscriberFilterClause = `
    ($subscriberFilterArgs.search IS NULL OR imsi LIKE $subscriberFilterArgs.search ESCAPE '\')
    AND ($subscriberFilterArgs.data_network_id IS NULL
         OR profileID IN (SELECT profileID FROM %s WHERE dataNetworkID = $subscriberFilterArgs.data_network_id))`

const listSubscribersFilteredStmt = `
  SELECT &Subscriber.*, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE` + subscriberFilterClause + `
  ORDER BY imsi
  LIMIT $ListArgs.limit OFFSET $ListArgs.offset`

const countSubscribersFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE` + subscriberFilterClause

type Subscriber struct {
	ID             string `db:"id"` // UUIDv7
	Imsi           string `db:"imsi"`
	SequenceNumber string `db:"sequenceNumber"`
	PermanentKey   string `db:"permanentKey"`
	Opc            string `db:"opc"`
	ProfileID      string `db:"profileID"`
}

type SubscriberFilters struct {
	Search        *string
	DataNetworkID *string
}

type subscriberFilterArgs struct {
	Search        *string `db:"search"`
	DataNetworkID *string `db:"data_network_id"`
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

func (f *SubscriberFilters) args() subscriberFilterArgs {
	if f == nil {
		return subscriberFilterArgs{}
	}

	args := subscriberFilterArgs{DataNetworkID: f.DataNetworkID}

	if f.Search != nil {
		pattern := "%" + likeEscaper.Replace(*f.Search) + "%"
		args.Search = &pattern
	}

	return args
}

func (db *Database) ListSubscribersPage(ctx context.Context, filters *SubscriberFilters, page int, perPage int) ([]Subscriber, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var subs []Subscriber

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	filterArgs := filters.args()

	err := db.conn().Query(ctx, db.listSubscribersStmt, args, filterArgs).GetAll(&subs, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.countSubscribersFiltered(ctx, filterArgs)
			if countErr != nil {
				span.RecordError(countErr)
				span.SetStatus(codes.Error, "fallback count failed")

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

	return subs, count, nil
}

func (db *Database) countSubscribersFiltered(ctx context.Context, filterArgs subscriberFilterArgs) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (filtered count)", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	if err := db.conn().Query(ctx, db.countSubscribersFilteredStmt, filterArgs).Get(&result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) GetSubscriber(ctx context.Context, imsi string) (*Subscriber, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	row := Subscriber{Imsi: imsi}

	err := db.conn().Query(ctx, db.getSubscriberStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

func (db *Database) CreateSubscriber(ctx context.Context, subscriber *Subscriber) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "insert").Inc()

	if subscriber.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate subscriber id: %w", err)
		}

		subscriber.ID = id.String()
	}

	_, err := opCreateSubscriber.Invoke(ctx, db, subscriber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateSubscriberProfile(ctx context.Context, subscriber *Subscriber) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

	_, err := opUpdateSubscriberProfile.Invoke(ctx, db, subscriber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) EditSubscriberSequenceNumber(ctx context.Context, imsi string, sequenceNumber string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (sequence number)", "UPDATE", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

	subscriber := &Subscriber{
		Imsi:           imsi,
		SequenceNumber: sequenceNumber,
	}

	_, err := opEditSubscriberSeqNum.Invoke(ctx, db, subscriber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteSubscriber(ctx context.Context, imsi string) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "delete").Inc()

	_, err := opDeleteSubscriber.Invoke(ctx, db, &stringPayload{Value: imsi})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) CountSubscribers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countSubscribersStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
