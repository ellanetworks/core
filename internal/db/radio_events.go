// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const RadioEventsTableName = "network_logs"

const (
	insertRadioEventSQL     = "INSERT INTO " + RadioEventsTableName + " (timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, raw, details) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	getRadioEventByIDSQL    = "SELECT id, timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, raw, details FROM " + RadioEventsTableName + " WHERE id = ?"
	deleteOldRadioEventsSQL = "DELETE FROM " + RadioEventsTableName + " WHERE timestamp < ?"
	deleteAllRadioEventsSQL = "DELETE FROM " + RadioEventsTableName
)

const listRadioEventsPagedFilteredSQL = `
  SELECT id, timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, raw, details, COUNT(*) OVER()
  FROM ` + RadioEventsTableName + `
  WHERE
    (?1      IS NULL OR protocol      = ?1)
    AND (?2 IS NULL OR direction = ?2)
    AND (?3 IS NULL OR radio_name = ?3)
    AND (?4 IS NULL OR message_type     = ?4)
    AND (?5  IS NULL OR timestamp >= ?5)
    AND (?6    IS NULL OR timestamp <  ?6)
  ORDER BY id DESC
  LIMIT ?7
  OFFSET ?8
`

const countRadioEventsFilteredSQL = `
  SELECT COUNT(*)
  FROM ` + RadioEventsTableName + `
  WHERE
    (?1      IS NULL OR protocol      = ?1)
    AND (?2 IS NULL OR direction = ?2)
    AND (?3 IS NULL OR radio_name = ?3)
    AND (?4 IS NULL OR message_type     = ?4)
    AND (?5  IS NULL OR timestamp >= ?5)
    AND (?6    IS NULL OR timestamp <  ?6)
`

type RadioEventFilters struct {
	Protocol      *string // exact match
	Direction     *string // "inbound" | "outbound"
	RadioName     *string // exact match
	MessageType   *string // exact match
	TimestampFrom *int64  // epoch milliseconds, inclusive lower bound
	TimestampTo   *int64  // epoch milliseconds, exclusive upper bound
}

func (db *Database) InsertRadioEvent(ctx context.Context, radioEvent *dbwriter.RadioEvent) error {
	ctx, span, timer := startDBQuery(ctx, RadioEventsTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.writer().ExecContext(ctx, insertRadioEventSQL, radioEvent.Timestamp, radioEvent.Protocol, radioEvent.MessageType, radioEvent.Direction, radioEvent.LocalAddress, radioEvent.RemoteAddress, radioEvent.RadioName, radioEvent.Raw, radioEvent.Details)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) InsertRadioEvents(ctx context.Context, radioEvents []*dbwriter.RadioEvent) error {
	switch len(radioEvents) {
	case 0:
		return nil
	case 1:
		return db.InsertRadioEvent(ctx, radioEvents[0])
	}

	ctx, span, timer := startDBQuery(ctx, RadioEventsTableName, "BATCH INSERT", "batch_insert", semconv.DBOperationBatchSize(len(radioEvents)))
	defer span.End()
	defer timer.ObserveDuration()

	tx, err := db.writer().BeginTx(ctx, nil)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("begin transaction failed: %w", err)
	}

	for _, radioEvent := range radioEvents {
		if _, err := tx.ExecContext(ctx, insertRadioEventSQL, radioEvent.Timestamp, radioEvent.Protocol, radioEvent.MessageType, radioEvent.Direction, radioEvent.LocalAddress, radioEvent.RemoteAddress, radioEvent.RadioName, radioEvent.Raw, radioEvent.Details); err != nil {
			_ = tx.Rollback()

			recordSpanError(span, err)

			return fmt.Errorf("query failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (db *Database) ListRadioEvents(ctx context.Context, page int, perPage int, filters *RadioEventFilters) ([]dbwriter.RadioEvent, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+RadioEventsTableName+" (paged+filtered)", RadioEventsTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	if filters == nil {
		filters = &RadioEventFilters{}
	}

	logs, count, err := queryPageWithTotal(ctx, db.reader(), listRadioEventsPagedFilteredSQL, scanRadioEvent, filters.Protocol, filters.Direction, filters.RadioName, filters.MessageType, filters.TimestampFrom, filters.TimestampTo, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(logs) == 0 {
		fallbackCount, err := queryCount(ctx, db.reader(), countRadioEventsFilteredSQL, filters.Protocol, filters.Direction, filters.RadioName, filters.MessageType, filters.TimestampFrom, filters.TimestampTo)
		if err != nil {
			recordSpanError(span, err)

			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return logs, count, nil
}

// DeleteOldRadioEvents removes logs older than the specified retention period in days.
func (db *Database) DeleteOldRadioEvents(ctx context.Context, days int) error {
	ctx, span, timer := startDBQueryNamed(ctx, "DELETE "+RadioEventsTableName+" (retention)", RadioEventsTableName, "DELETE", "delete", attribute.Int("retention.days", days))
	defer span.End()
	defer timer.ObserveDuration()

	cutoff := dbwriter.EpochMillis(time.Now().UTC().AddDate(0, 0, -days))

	_, err := db.writer().ExecContext(ctx, deleteOldRadioEventsSQL, cutoff)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ClearRadioEvents(ctx context.Context) error {
	ctx, span, timer := startDBQueryNamed(ctx, "DELETE "+RadioEventsTableName+" (all)", RadioEventsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := db.writer().ExecContext(ctx, deleteAllRadioEventsSQL)
	if err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) GetRadioEventByID(ctx context.Context, id int) (*dbwriter.RadioEvent, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+RadioEventsTableName+" (by ID)", RadioEventsTableName, "SELECT", "select", attribute.Int("radio_event.id", id))
	defer span.End()
	defer timer.ObserveDuration()

	log, err := scanRadioEvent(db.reader().QueryRowContext(ctx, getRadioEventByIDSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &log, nil
}

func scanRadioEvent(row scanner) (dbwriter.RadioEvent, error) {
	var v dbwriter.RadioEvent

	if err := row.Scan(&v.ID, &v.Timestamp, &v.Protocol, &v.MessageType, &v.Direction, &v.LocalAddress, &v.RemoteAddress, &v.RadioName, &v.Raw, &v.Details); err != nil {
		return dbwriter.RadioEvent{}, err
	}

	return v, nil
}
