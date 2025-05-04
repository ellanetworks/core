// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
	"net"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/logger"
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

		imsi TEXT NOT NULL,

		ipAddress TEXT,

		sequenceNumber TEXT NOT NULL,
		permanentKey TEXT NOT NULL,
		opc TEXT NOT NULL,

		profileID INTEGER NOT NULL,
    	FOREIGN KEY (profileID) REFERENCES profiles (id)
)`

const (
	listSubscribersStmt   = "SELECT &Subscriber.* from %s"
	getSubscriberStmt     = "SELECT &Subscriber.* from %s WHERE imsi==$Subscriber.imsi"
	createSubscriberStmt  = "INSERT INTO %s (imsi, ipAddress, sequenceNumber, permanentKey, opc, profileID) VALUES ($Subscriber.imsi, $Subscriber.ipAddress, $Subscriber.sequenceNumber, $Subscriber.permanentKey, $Subscriber.opc, $Subscriber.profileID)"
	editSubscriberStmt    = "UPDATE %s SET ipAddress=$Subscriber.ipAddress, sequenceNumber=$Subscriber.sequenceNumber, permanentKey=$Subscriber.permanentKey, opc=$Subscriber.opc, profileID=$Subscriber.profileID WHERE imsi==$Subscriber.imsi"
	deleteSubscriberStmt  = "DELETE FROM %s WHERE imsi==$Subscriber.imsi"
	getNumSubscribersStmt = "SELECT COUNT(*) AS &NumSubscribers.count FROM %s"
	checkIPStmt           = "SELECT &Subscriber.* FROM %s WHERE ipAddress=$Subscriber.ipAddress"
	allocateIPStmt        = "UPDATE %s SET ipAddress=$Subscriber.ipAddress WHERE imsi=$Subscriber.imsi"
	releaseIPStmt         = "UPDATE %s SET ipAddress=NULL WHERE imsi=$Subscriber.imsi"
)

type NumSubscribers struct {
	Count int `db:"count"`
}

type Subscriber struct {
	ID             int    `db:"id"`
	Imsi           string `db:"imsi"`
	IPAddress      string `db:"ipAddress"`
	SequenceNumber string `db:"sequenceNumber"`
	PermanentKey   string `db:"permanentKey"`
	Opc            string `db:"opc"`
	ProfileID      int    `db:"profileID"`
}

// ListSubscribers returns all subscribers, with OpenTelemetry spans
// named according to the OTLP Span Name conventions.
func (db *Database) ListSubscribers(ctx context.Context) ([]Subscriber, error) {
	operation := "SELECT"
	target := db.subscribersTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	// attach standard semconv + low-card attributes
	stmt := fmt.Sprintf(listSubscribersStmt, db.subscribersTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	q, err := sqlair.Prepare(stmt, Subscriber{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}
	var subs []Subscriber
	if err := db.conn.Query(ctx, q).GetAll(&subs); err != nil {
		if err == sql.ErrNoRows {
			// no rows isn't really an error
			span.SetStatus(codes.Ok, "no rows")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return subs, nil
}

func (db *Database) GetSubscriber(imsi string, ctx context.Context) (*Subscriber, error) {
	operation := "SELECT"
	target := db.subscribersTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getSubscriberStmt, db.subscribersTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	row := Subscriber{Imsi: imsi}
	q, err := sqlair.Prepare(stmt, Subscriber{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return nil, err
	}
	if err := db.conn.Query(ctx, q, row).Get(&row); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return &row, nil
}

func (db *Database) CreateSubscriber(subscriber *Subscriber, ctx context.Context) error {
	operation := "INSERT"
	target := db.subscribersTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(createSubscriberStmt, db.subscribersTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	if _, err := db.GetSubscriber(subscriber.Imsi, ctx); err == nil {
		dupErr := fmt.Errorf("subscriber with imsi %s already exists", subscriber.Imsi)
		span.RecordError(dupErr)
		span.SetStatus(codes.Error, "duplicate key")
		return dupErr
	}

	q, err := sqlair.Prepare(stmt, Subscriber{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, q, subscriber).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) UpdateSubscriber(subscriber *Subscriber, ctx context.Context) error {
	operation := "UPDATE"
	target := db.subscribersTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(editSubscriberStmt, db.subscribersTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// verify existence
	if _, err := db.GetSubscriber(subscriber.Imsi, ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, Subscriber{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, q, subscriber).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) DeleteSubscriber(imsi string, ctx context.Context) error {
	operation := "DELETE"
	target := db.subscribersTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(deleteSubscriberStmt, db.subscribersTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	// verify existence
	if _, err := db.GetSubscriber(imsi, ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return err
	}

	q, err := sqlair.Prepare(stmt, Subscriber{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return err
	}

	if err := db.conn.Query(ctx, q, Subscriber{Imsi: imsi}).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return err
	}

	logger.DBLog.Info("Deleted subscriber", zap.String("imsi", imsi))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (db *Database) SubscribersInProfile(name string, ctx context.Context) (bool, error) {
	// business‐logic span (no direct SQL)
	ctx, span := tracer.Start(ctx, "SubscribersInProfile")
	defer span.End()

	profile, err := db.GetProfile(name, ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "profile not found")
		return false, err
	}

	subs, err := db.ListSubscribers(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "listing failed")
		return false, err
	}

	for _, s := range subs {
		if s.ProfileID == profile.ID {
			span.SetStatus(codes.Ok, "")
			return true, nil
		}
	}

	span.SetStatus(codes.Ok, "none found")
	return false, nil
}

func (db *Database) allocateIP(imsi string, ctx context.Context) (net.IP, error) {
	subscriber, err := db.GetSubscriber(imsi, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %v", err)
	}

	profile, err := db.GetProfileByID(subscriber.ProfileID, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile for subscriber %s: %v", imsi, err)
	}

	_, ipNet, err := net.ParseCIDR(profile.UeIPPool)
	if err != nil {
		return nil, fmt.Errorf("invalid IP pool in profile %s: %v", profile.Name, err)
	}

	baseIP := ipNet.IP
	maskBits, totalBits := ipNet.Mask.Size()
	totalIPs := 1 << (totalBits - maskBits)

	for i := 1; i < totalIPs-1; i++ { // Skip network and broadcast addresses
		ip := addOffsetToIP(baseIP, i)
		ipStr := ip.String()

		stmt, err := sqlair.Prepare(fmt.Sprintf(checkIPStmt, SubscribersTableName), Subscriber{})
		if err != nil {
			return nil, fmt.Errorf("failed to prepare IP check statement: %v", err)
		}
		var existing Subscriber
		err = db.conn.Query(ctx, stmt, Subscriber{IPAddress: ipStr}).Get(&existing)
		if err == sql.ErrNoRows {
			// IP is not allocated, assign it to the subscriber
			subscriber.IPAddress = ipStr
			stmt, err := sqlair.Prepare(fmt.Sprintf(allocateIPStmt, SubscribersTableName), Subscriber{})
			if err != nil {
				return nil, fmt.Errorf("failed to prepare IP allocation statement: %v", err)
			}
			err = db.conn.Query(ctx, stmt, subscriber).Run()
			if err != nil {
				return nil, fmt.Errorf("failed to allocate IP: %v", err)
			}
			return ip, nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to check IP availability: %v", err)
		}
	}
	return nil, fmt.Errorf("no available IP addresses")
}

func (db *Database) AllocateIP(imsi string, ctx context.Context) (net.IP, error) {
	ctx, span := tracer.Start(ctx, "AllocateIP", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	ip, err := db.allocateIP(imsi, ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "allocation failed")
		return nil, err
	}

	span.SetStatus(codes.Ok, "")
	return ip, nil
}

func (db *Database) releaseIP(imsi string, ctx context.Context) error {
	subscriber, err := db.GetSubscriber(imsi, ctx)
	if err != nil {
		return fmt.Errorf("failed to get subscriber: %v", err)
	}

	if subscriber.IPAddress == "" {
		return nil
	}

	stmt, err := sqlair.Prepare(fmt.Sprintf(releaseIPStmt, SubscribersTableName), Subscriber{})
	if err != nil {
		return fmt.Errorf("failed to prepare IP release statement: %v", err)
	}

	err = db.conn.Query(ctx, stmt, subscriber).Run()
	if err != nil {
		return fmt.Errorf("failed to release IP: %v", err)
	}

	return nil
}

// ReleaseIP removes any assigned IP for a subscriber.
func (db *Database) ReleaseIP(imsi string, ctx context.Context) error {
	ctx, span := tracer.Start(ctx, "ReleaseIP", trace.WithSpanKind(trace.SpanKindInternal))
	defer span.End()

	err := db.releaseIP(imsi, ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "release failed")
		return err
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

func (db *Database) NumSubscribers(ctx context.Context) (int, error) {
	operation := "SELECT"
	target := db.subscribersTable
	spanName := fmt.Sprintf("%s %s", operation, target)

	ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	stmt := fmt.Sprintf(getNumSubscribersStmt, db.subscribersTable)
	span.SetAttributes(
		semconv.DBSystemSqlite,
		semconv.DBStatementKey.String(stmt),
		semconv.DBOperationKey.String(operation),
		attribute.String("db.collection", target),
	)

	var result NumSubscribers
	q, err := sqlair.Prepare(stmt, NumSubscribers{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "prepare failed")
		return 0, err
	}

	if err := db.conn.Query(ctx, q).Get(&result); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "execution failed")
		return 0, err
	}

	span.SetStatus(codes.Ok, "")
	return result.Count, nil
}
