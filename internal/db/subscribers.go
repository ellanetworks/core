// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SubscribersTableName = "subscribers"

// subscriberDescriptionSchema is the migration that adds subscribers.description.
// It is post-baseline, so a cluster node reaches it through Raft some time after
// the new binary starts; until then the column is absent and every statement
// naming it fails to compile in SQLite. Reads pick a variant on
// cachedAppliedSchema(); writes are gated by RequireSchema at op registration.
const subscriberDescriptionSchema = 18

const subscriberMSISDNSchema = 22

const (
	createSubscriberStmt      = "INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, profileID, description, msisdn) VALUES ($Subscriber.id, $Subscriber.imsi, $Subscriber.sequenceNumber, $Subscriber.permanentKey, $Subscriber.opc, $Subscriber.profileID, $Subscriber.description, $Subscriber.msisdn)"
	editSubscriberProfileStmt = "UPDATE %s SET profileID=$Subscriber.profileID, description=$Subscriber.description, msisdn=$Subscriber.msisdn WHERE imsi==$Subscriber.imsi"
	editSubscriberSeqNumStmt  = "UPDATE %s SET sequenceNumber=$Subscriber.sequenceNumber WHERE imsi==$Subscriber.imsi"
	casSubscriberSeqNumStmt   = "UPDATE %s SET sequenceNumber=$sqnCAS.next WHERE imsi==$sqnCAS.imsi AND sequenceNumber==$sqnCAS.expected"
	deleteSubscriberStmt      = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
	countSubscribersStmt      = "SELECT COUNT(*) AS &NumItems.count FROM %s"
)

const subscriberColumnsPreV18 = "&Subscriber.id, &Subscriber.imsi, &Subscriber.sequenceNumber, &Subscriber.permanentKey, &Subscriber.opc, &Subscriber.profileID"

const subscriberColumnsPreV22 = subscriberColumnsPreV18 + ", &Subscriber.description"

const (
	getSubscriberStmt         = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	getSubscriberPreV22Stmt   = "SELECT " + subscriberColumnsPreV22 + " from %s WHERE imsi==$Subscriber.imsi"
	getSubscriberPreV18Stmt   = "SELECT " + subscriberColumnsPreV18 + " from %s WHERE imsi==$Subscriber.imsi"
	getSubscriberByMSISDNStmt = "SELECT &Subscriber.* from %s WHERE msisdn==$Subscriber.msisdn AND msisdn!=''"
	listIMSIsWithMSISDNStmt   = "SELECT &subscriberIMSI.imsi FROM %s WHERE msisdn!='' AND imsi IN ($SliceIDs[:])"
)

const (
	subscriberSearchClause = `
    ($subscriberFilterArgs.search IS NULL
     OR imsi LIKE $subscriberFilterArgs.search ESCAPE '\'
     OR (msisdn != '' AND '+' || msisdn LIKE $subscriberFilterArgs.search ESCAPE '\')
     OR description LIKE $subscriberFilterArgs.search ESCAPE '\')`

	subscriberSearchClausePreV22 = `
    ($subscriberFilterArgs.search IS NULL
     OR imsi LIKE $subscriberFilterArgs.search ESCAPE '\'
     OR description LIKE $subscriberFilterArgs.search ESCAPE '\')`

	subscriberSearchClausePreV18 = `
    ($subscriberFilterArgs.search IS NULL OR imsi LIKE $subscriberFilterArgs.search ESCAPE '\')`

	subscriberDataNetworkClause = `
    AND ($subscriberFilterArgs.data_network_id IS NULL
         OR profileID IN (SELECT profileID FROM %s WHERE dataNetworkID = $subscriberFilterArgs.data_network_id))`
)

const (
	listSubscribersFilteredStmt = `
  SELECT &Subscriber.*, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE` + subscriberSearchClause + subscriberDataNetworkClause + `
  ORDER BY imsi
  LIMIT $ListArgs.limit OFFSET $ListArgs.offset`

	listSubscribersFilteredPreV22Stmt = `
  SELECT ` + subscriberColumnsPreV22 + `, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE` + subscriberSearchClausePreV22 + subscriberDataNetworkClause + `
  ORDER BY imsi
  LIMIT $ListArgs.limit OFFSET $ListArgs.offset`

	listSubscribersFilteredPreV18Stmt = `
  SELECT ` + subscriberColumnsPreV18 + `, COUNT(*) OVER() AS &NumItems.count
  FROM %s
  WHERE` + subscriberSearchClausePreV18 + subscriberDataNetworkClause + `
  ORDER BY imsi
  LIMIT $ListArgs.limit OFFSET $ListArgs.offset`

	countSubscribersFilteredStmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE` + subscriberSearchClause + subscriberDataNetworkClause

	countSubscribersFilteredPreV22Stmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE` + subscriberSearchClausePreV22 + subscriberDataNetworkClause

	countSubscribersFilteredPreV18Stmt = `
  SELECT COUNT(*) AS &NumItems.count
  FROM %s
  WHERE` + subscriberSearchClausePreV18 + subscriberDataNetworkClause
)

type Subscriber struct {
	ID             string `db:"id"` // UUIDv7
	Imsi           string `db:"imsi"`
	SequenceNumber string `db:"sequenceNumber"`
	PermanentKey   string `db:"permanentKey"`
	Opc            string `db:"opc"`
	ProfileID      string `db:"profileID"`
	Description    string `db:"description"`
	Msisdn         string `db:"msisdn"`
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
	querySummary := fmt.Sprintf("%s %s (paged)", "SELECT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SubscribersTableName),
			attribute.Int("ella.db.page", page),
			attribute.Int("ella.db.page_size", perPage),
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

	stmt := db.listSubscribersStmt
	if !db.appliedSchemaAtLeast(ctx, subscriberDescriptionSchema) {
		stmt = db.listSubscribersPreV18Stmt
	} else if !db.appliedSchemaAtLeast(ctx, subscriberMSISDNSchema) {
		stmt = db.listSubscribersPreV22Stmt
	}

	err := db.conn().Query(ctx, stmt, args, filterArgs).GetAll(&subs, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fallbackCount, countErr := db.countSubscribersFiltered(ctx, filterArgs)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	return subs, count, nil
}

func (db *Database) countSubscribersFiltered(ctx context.Context, filterArgs subscriberFilterArgs) (int, error) {
	querySummary := fmt.Sprintf("%s %s (filtered count)", "SELECT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	stmt := db.countSubscribersFilteredStmt
	if !db.appliedSchemaAtLeast(ctx, subscriberDescriptionSchema) {
		stmt = db.countSubscribersFilteredPreV18Stmt
	} else if !db.appliedSchemaAtLeast(ctx, subscriberMSISDNSchema) {
		stmt = db.countSubscribersFilteredPreV22Stmt
	}

	if err := db.conn().Query(ctx, stmt, filterArgs).Get(&result); err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) GetSubscriber(ctx context.Context, imsi string) (*Subscriber, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	row := Subscriber{Imsi: imsi}

	stmt := db.getSubscriberStmt
	if !db.appliedSchemaAtLeast(ctx, subscriberDescriptionSchema) {
		stmt = db.getSubscriberPreV18Stmt
	} else if !db.appliedSchemaAtLeast(ctx, subscriberMSISDNSchema) {
		stmt = db.getSubscriberPreV22Stmt
	}

	err := db.conn().Query(ctx, stmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

const maxIMSIsPerQuery = 500

type subscriberIMSI struct {
	Imsi string `db:"imsi"`
}

func (db *Database) IMSIsWithMSISDN(ctx context.Context, imsis []string) (map[string]bool, error) {
	querySummary := fmt.Sprintf("%s %s (with msisdn)", "SELECT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	found := make(map[string]bool, len(imsis))

	if !db.appliedSchemaAtLeast(ctx, subscriberMSISDNSchema) {
		return found, nil
	}

	for chunk := range slices.Chunk(imsis, maxIMSIsPerQuery) {
		var rows []subscriberIMSI

		if err := db.conn().Query(ctx, db.listIMSIsWithMSISDNStmt, SliceIDs(chunk)).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
			recordSpanError(span, err)
			return nil, fmt.Errorf("query failed: %w", err)
		}

		for _, r := range rows {
			found[r.Imsi] = true
		}
	}

	return found, nil
}

func (db *Database) GetSubscriberByMSISDN(ctx context.Context, msisdn string) (*Subscriber, error) {
	querySummary := fmt.Sprintf("%s %s (by msisdn)", "SELECT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	if msisdn == "" || !db.appliedSchemaAtLeast(ctx, subscriberMSISDNSchema) {
		return nil, ErrNotFound
	}

	row := Subscriber{Msisdn: msisdn}

	err := db.conn().Query(ctx, db.getSubscriberByMSISDNStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateSubscriber(ctx context.Context, subscriber *Subscriber) error {
	querySummary := fmt.Sprintf("%s %s", "INSERT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "insert").Inc()

	if subscriber.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate subscriber id: %w", err)
		}

		subscriber.ID = id.String()
	}

	_, err := opCreateSubscriber.Invoke(ctx, db, subscriber)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdateSubscriberProfile(ctx context.Context, subscriber *Subscriber) error {
	querySummary := fmt.Sprintf("%s %s", "UPDATE", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

	_, err := opUpdateSubscriberProfile.Invoke(ctx, db, subscriber)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) EditSubscriberSequenceNumber(ctx context.Context, imsi string, sequenceNumber string) error {
	querySummary := fmt.Sprintf("%s %s (sequence number)", "UPDATE", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(SubscribersTableName),
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
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteSubscriber(ctx context.Context, imsi string) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "delete").Inc()

	_, err := opDeleteSubscriber.Invoke(ctx, db, &stringPayload{Value: imsi})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) CountSubscribers(ctx context.Context) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", SubscribersTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countSubscribersStmt).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}
