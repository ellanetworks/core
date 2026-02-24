// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const SubscribersTableName = "subscribers"

const QueryCreateSubscribersTable = `
	CREATE TABLE IF NOT EXISTS %s (
 		id INTEGER PRIMARY KEY AUTOINCREMENT,

		imsi TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),

		ipAddress TEXT UNIQUE,

		sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
		permanentKey TEXT NOT NULL CHECK (length(permanentKey) = 32),
		opc TEXT NOT NULL CHECK (length(opc) = 32),

		policyID INTEGER NOT NULL,

		FOREIGN KEY (policyID) REFERENCES policies (id) ON DELETE CASCADE
)`

const (
	listSubscribersPagedStmt     = "SELECT &Subscriber.*, COUNT(*) OVER() AS &NumItems.count from %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getSubscriberStmt            = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	createSubscriberStmt         = "INSERT INTO %s (imsi, ipAddress, sequenceNumber, permanentKey, opc, policyID) VALUES ($Subscriber.imsi, $Subscriber.ipAddress, $Subscriber.sequenceNumber, $Subscriber.permanentKey, $Subscriber.opc, $Subscriber.policyID)"
	editSubscriberPolicyStmt     = "UPDATE %s SET policyID=$Subscriber.policyID WHERE imsi==$Subscriber.imsi"
	editSubscriberSeqNumStmt     = "UPDATE %s SET sequenceNumber=$Subscriber.sequenceNumber WHERE imsi==$Subscriber.imsi"
	deleteSubscriberStmt         = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
	countSubscribersStmt         = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	countSubscribersInPolicyStmt = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE policyID=$Subscriber.policyID"
	countSubscribersWithIPStmt   = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE ipAddress IS NOT NULL AND TRIM(ipAddress) <> ''"
	checkIPStmt                  = "SELECT &Subscriber.* FROM %s WHERE ipAddress=$Subscriber.ipAddress"
	allocateIPStmt               = "UPDATE %s SET ipAddress=$Subscriber.ipAddress WHERE imsi=$Subscriber.imsi"
	releaseIPStmt                = "UPDATE %s SET ipAddress=NULL WHERE imsi=$Subscriber.imsi"
	releaseAllIPStmt             = "UPDATE %s SET ipAddress=NULL"
)

type Subscriber struct {
	ID             int     `db:"id"`
	Imsi           string  `db:"imsi"`
	IPAddress      *string `db:"ipAddress"`
	SequenceNumber string  `db:"sequenceNumber"`
	PermanentKey   string  `db:"permanentKey"`
	Opc            string  `db:"opc"`
	PolicyID       int     `db:"policyID"`
}

func (db *Database) ListSubscribersPage(ctx context.Context, page int, perPage int) ([]Subscriber, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
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

	err := db.conn.Query(ctx, db.listSubscribersStmt, args).GetAll(&subs, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountSubscribers(ctx)
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

	return subs, count, nil
}

func (db *Database) GetSubscriber(ctx context.Context, imsi string) (*Subscriber, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	row := Subscriber{Imsi: imsi}

	err := db.conn.Query(ctx, db.getSubscriberStmt, row).Get(&row)
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
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("INSERT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "insert").Inc()

	err := db.conn.Query(ctx, db.createSubscriberStmt, subscriber).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdateSubscriberPolicy(ctx context.Context, subscriber *Subscriber) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.updateSubscriberPolicyStmt, subscriber).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) EditSubscriberSequenceNumber(ctx context.Context, imsi string, sequenceNumber string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (sequence number)", "UPDATE", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("UPDATE"),
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

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.updateSubscriberSqnNumStmt, subscriber).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteSubscriber(ctx context.Context, imsi string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("DELETE"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.conn.Query(ctx, db.deleteSubscriberStmt, Subscriber{Imsi: imsi}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) SubscribersInPolicy(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"SubscribersInPolicy",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
		),
	)
	defer span.End()

	policy, err := db.GetPolicy(ctx, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			span.RecordError(ErrNotFound)
			span.SetStatus(codes.Error, "policy not found")

			return false, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "policy not found")

		return false, fmt.Errorf("policy not found: %w", err)
	}

	count, err := db.CountSubscribersInPolicy(ctx, policy.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "counting failed")

		return false, fmt.Errorf("counting failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return count > 0, nil
}

func (db *Database) PoliciesInDataNetwork(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"PoliciesInDataNetwork",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
		),
	)
	defer span.End()

	dataNetwork, err := db.GetDataNetwork(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "data network not found")

		return false, fmt.Errorf("data network not found: %w", err)
	}

	policies, _, err := db.ListPoliciesPage(ctx, 1, 1000)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "listing failed")

		return false, fmt.Errorf("listing failed: %w", err)
	}

	for _, p := range policies {
		if p.DataNetworkID == dataNetwork.ID {
			span.SetStatus(codes.Ok, "")
			return true, nil
		}
	}

	span.SetStatus(codes.Ok, "none found")

	return false, nil
}

func (db *Database) AllocateIP(ctx context.Context, imsi string) (net.IP, error) {
	ctx, span := tracer.Start(
		ctx,
		"AllocateIP",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
		),
	)
	defer span.End()

	subscriber, err := db.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %v", err)
	}

	policy, err := db.GetPolicyByID(ctx, subscriber.PolicyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy for subscriber %s: %v", imsi, err)
	}

	dataNetwork, err := db.GetDataNetworkByID(ctx, policy.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("failed to get data network for policy %s: %v", policy.Name, err)
	}

	_, ipNet, err := net.ParseCIDR(dataNetwork.IPPool)
	if err != nil {
		return nil, fmt.Errorf("invalid IP pool in policy %s: %v", policy.Name, err)
	}

	ctx, ipAllocSpan := tracer.Start(ctx, "IP Allocation Loop")
	defer ipAllocSpan.End()

	baseIP := ipNet.IP
	maskBits, totalBits := ipNet.Mask.Size()
	totalIPs := 1 << (totalBits - maskBits)

	for i := 1; i < totalIPs-1; i++ { // Skip network and broadcast addresses
		ip := addOffsetToIP(baseIP, i)
		ipStr := ip.String()

		var existing Subscriber

		err = db.conn.Query(ctx, db.checkSubscriberIPStmt, Subscriber{IPAddress: &ipStr}).Get(&existing)
		if errors.Is(err, sql.ErrNoRows) {
			// IP is not allocated, assign it to the subscriber
			subscriber.IPAddress = &ipStr

			timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))

			DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

			err = db.conn.Query(ctx, db.allocateSubscriberIPStmt, subscriber).Run()

			timer.ObserveDuration()

			if err != nil {
				if isUniqueNameError(err) {
					logger.WithTrace(ctx, logger.DBLog).Warn("IP address collision during allocation, retrying", zap.String("ip", ipStr))
					continue
				}

				return nil, fmt.Errorf("failed to allocate IP: %v", err)
			}

			return ip, nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to check IP availability: %v", err)
		}
	}

	return nil, fmt.Errorf("no available IP addresses")
}

// ReleaseIP removes any assigned IP for a subscriber.
func (db *Database) ReleaseIP(ctx context.Context, imsi string) error {
	ctx, span := tracer.Start(
		ctx,
		"ReleaseIP",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
		),
	)
	defer span.End()

	subscriber, err := db.GetSubscriber(ctx, imsi)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get subscriber")

		return fmt.Errorf("failed to get subscriber: %v", err)
	}

	if subscriber.IPAddress == nil {
		span.SetStatus(codes.Ok, "no IP to release")
		return nil
	}

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

	err = db.conn.Query(ctx, db.releaseSubscriberIPStmt, subscriber).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "release failed")

		return fmt.Errorf("failed to release IP: %v", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// ReleaseAllIPs removes all assigned IPs for all subscribers.
// This is meant to be called only on startup or shutdown.
func (db *Database) ReleaseAllIPs(ctx context.Context) error {
	ctx, span := tracer.Start(
		ctx,
		"ReleaseAllIPs",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "update").Inc()

	err := db.conn.Query(ctx, db.releaseAllIPStmt).Run()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "release failed")

		return fmt.Errorf("failed to release all IPs: %v", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func addOffsetToIP(baseIP net.IP, offset int) net.IP {
	resultIP := make(net.IP, len(baseIP))
	copy(resultIP, baseIP)

	for i := len(resultIP) - 1; i >= 0; i-- {
		offset += int(resultIP[i])
		resultIP[i] = byte(offset)
		offset >>= 8
	}

	return resultIP
}

func (db *Database) CountSubscribers(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countSubscribersStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) CountSubscribersInPolicy(ctx context.Context, policyID int) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by policy)", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
			attribute.Int("policy_id", policyID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	subscriber := Subscriber{PolicyID: policyID}

	err := db.conn.Query(ctx, db.countSubscribersByPolicyStmt, subscriber).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) CountSubscribersWithIP(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (with IP)", "SELECT", SubscribersTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemSqlite,
			semconv.DBOperationKey.String("SELECT"),
			attribute.String("db.collection", SubscribersTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SubscribersTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SubscribersTableName, "select").Inc()

	var result NumItems

	err := db.conn.Query(ctx, db.countSubscribersWithIPStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}
