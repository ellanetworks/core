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

const radioEventRefSchema = 28

const (
	deleteOldRadioEventsSQL = "DELETE FROM " + RadioEventsTableName + " WHERE timestamp < ?"
	deleteAllRadioEventsSQL = "DELETE FROM " + RadioEventsTableName
)

type radioEventStatements struct {
	insert  string
	getByID string
	list    string
	count   string
	withRef bool
}

func newRadioEventStatements(refColumn string, withRef bool) radioEventStatements {
	cols := "id, timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, " + refColumn + ", raw, details"

	where := `
  WHERE
    (?1      IS NULL OR protocol      = ?1)
    AND (?2 IS NULL OR direction = ?2)
    AND (?3 IS NULL OR ` + refColumn + ` = ?3)
    AND (?4 IS NULL OR message_type     = ?4)
    AND (?5  IS NULL OR timestamp >= ?5)
    AND (?6    IS NULL OR timestamp <  ?6)
`

	insert := "INSERT INTO " + RadioEventsTableName + " (timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, raw, details) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	if withRef {
		insert = "INSERT INTO " + RadioEventsTableName + " (timestamp, protocol, message_type, direction, local_address, remote_address, radio_name, radio_ref, raw, details) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	}

	return radioEventStatements{
		insert:  insert,
		getByID: "SELECT " + cols + " FROM " + RadioEventsTableName + " WHERE id = ?",
		list: `
  SELECT ` + cols + `, COUNT(*) OVER()
  FROM ` + RadioEventsTableName + where + `
  ORDER BY id DESC
  LIMIT ?7
  OFFSET ?8
`,
		count:   "\n  SELECT COUNT(*)\n  FROM " + RadioEventsTableName + where,
		withRef: withRef,
	}
}

var (
	radioEventSQL       = newRadioEventStatements("radio_ref", true)
	legacyRadioEventSQL = newRadioEventStatements("''", false)
)

func (db *Database) radioEventStatements(ctx context.Context) radioEventStatements {
	if db.appliedSchemaAtLeast(ctx, radioEventRefSchema) {
		return radioEventSQL
	}

	return legacyRadioEventSQL
}

func (q radioEventStatements) insertArgs(e *dbwriter.RadioEvent) []any {
	if q.withRef {
		return []any{e.Timestamp, e.Protocol, e.MessageType, e.Direction, e.LocalAddress, e.RemoteAddress, e.RadioName, e.RadioRef, e.Raw, e.Details}
	}

	return []any{e.Timestamp, e.Protocol, e.MessageType, e.Direction, e.LocalAddress, e.RemoteAddress, e.RadioName, e.Raw, e.Details}
}

type RadioEventFilters struct {
	Protocol      *string // exact match
	Direction     *string // "inbound" | "outbound"
	RadioRef      *string // exact match
	MessageType   *string // exact match
	TimestampFrom *int64  // epoch milliseconds, inclusive lower bound
	TimestampTo   *int64  // epoch milliseconds, exclusive upper bound
}

func (db *Database) InsertRadioEvent(ctx context.Context, radioEvent *dbwriter.RadioEvent) (err error) {
	ctx, finish := startDBQuery(ctx, RadioEventsTableName, "INSERT")
	defer func() { finish(err) }()

	q := db.radioEventStatements(ctx)

	_, err = db.writer().ExecContext(ctx, q.insert, q.insertArgs(radioEvent)...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) InsertRadioEvents(ctx context.Context, radioEvents []*dbwriter.RadioEvent) (err error) {
	switch len(radioEvents) {
	case 0:
		return nil
	case 1:
		return db.InsertRadioEvent(ctx, radioEvents[0])
	}

	ctx, finish := startDBQuery(ctx, RadioEventsTableName, "BATCH INSERT", semconv.DBOperationBatchSize(len(radioEvents)))
	defer func() { finish(err) }()

	tx, err := db.writer().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}

	q := db.radioEventStatements(ctx)

	for _, radioEvent := range radioEvents {
		if _, err := tx.ExecContext(ctx, q.insert, q.insertArgs(radioEvent)...); err != nil {
			_ = tx.Rollback()

			return fmt.Errorf("query failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (db *Database) ListRadioEvents(ctx context.Context, page int, perPage int, filters *RadioEventFilters) (_ []dbwriter.RadioEvent, _ int, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+RadioEventsTableName+" (paged+filtered)", RadioEventsTableName, "SELECT", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer func() { finish(err) }()

	if filters == nil {
		filters = &RadioEventFilters{}
	}

	q := db.radioEventStatements(ctx)

	logs, count, err := queryPageWithTotal(ctx, db.reader(), q.list, scanRadioEvent, filters.Protocol, filters.Direction, filters.RadioRef, filters.MessageType, filters.TimestampFrom, filters.TimestampTo, perPage, (page-1)*perPage)
	if err != nil {
		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(logs) == 0 {
		fallbackCount, err := queryCount(ctx, db.reader(), q.count, filters.Protocol, filters.Direction, filters.RadioRef, filters.MessageType, filters.TimestampFrom, filters.TimestampTo)
		if err != nil {
			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	return logs, count, nil
}

// DeleteOldRadioEvents removes logs older than the specified retention period in days.
func (db *Database) DeleteOldRadioEvents(ctx context.Context, days int) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+RadioEventsTableName+" (retention)", RadioEventsTableName, "DELETE", attribute.Int("retention.days", days))
	defer func() { finish(err) }()

	cutoff := dbwriter.EpochMillis(time.Now().UTC().AddDate(0, 0, -days))

	_, err = db.writer().ExecContext(ctx, deleteOldRadioEventsSQL, cutoff)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) ClearRadioEvents(ctx context.Context) (err error) {
	ctx, finish := startDBQueryNamed(ctx, "DELETE "+RadioEventsTableName+" (all)", RadioEventsTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = db.writer().ExecContext(ctx, deleteAllRadioEventsSQL)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	return nil
}

func (db *Database) GetRadioEventByID(ctx context.Context, id int) (_ *dbwriter.RadioEvent, err error) {
	ctx, finish := startDBQueryNamed(ctx, "SELECT "+RadioEventsTableName+" (by ID)", RadioEventsTableName, "SELECT", attribute.Int("radio_event.id", id))
	defer func() { finish(err) }()

	log, err := scanRadioEvent(db.reader().QueryRowContext(ctx, db.radioEventStatements(ctx).getByID, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &log, nil
}

func scanRadioEvent(row scanner) (dbwriter.RadioEvent, error) {
	var v dbwriter.RadioEvent

	if err := row.Scan(&v.ID, &v.Timestamp, &v.Protocol, &v.MessageType, &v.Direction, &v.LocalAddress, &v.RemoteAddress, &v.RadioName, &v.RadioRef, &v.Raw, &v.Details); err != nil {
		return dbwriter.RadioEvent{}, err
	}

	return v, nil
}
