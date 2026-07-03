// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/canonical/sqlair"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	CellPositionsTableName = "cell_positions"

	// RATNR and RATEUTRA identify the radio access technology a cell position
	// applies to (NCGI vs ECGI).
	RATNR    = "nr"
	RATEUTRA = "eutra"
)

const (
	createCellPositionStmt    = `INSERT INTO %s (id, rat, mcc, mnc, cell_identity, gnb_id, latitude, longitude, altitude, uncertainty_semi_major, uncertainty_semi_minor, orientation_major, confidence, source, created_at, updated_at) VALUES ($CellPosition.id, $CellPosition.rat, $CellPosition.mcc, $CellPosition.mnc, $CellPosition.cell_identity, $CellPosition.gnb_id, $CellPosition.latitude, $CellPosition.longitude, $CellPosition.altitude, $CellPosition.uncertainty_semi_major, $CellPosition.uncertainty_semi_minor, $CellPosition.orientation_major, $CellPosition.confidence, $CellPosition.source, $CellPosition.created_at, $CellPosition.updated_at);`
	getCellPositionStmt       = `SELECT &CellPosition.* FROM %s WHERE id==$CellPosition.id;`
	getCellPositionByCellStmt = `SELECT &CellPosition.* FROM %s WHERE rat==$CellPosition.rat AND mcc==$CellPosition.mcc AND mnc==$CellPosition.mnc AND cell_identity==$CellPosition.cell_identity;`
	listCellPositionsStmt     = `SELECT &CellPosition.* FROM %s ORDER BY created_at DESC;`
	// source is intentionally excluded from the SET clause: it records how the
	// row originated (currently always "provisioned") and must not change on
	// edit. CellPositionRequest has no Source field, so leaving it in the SET
	// clause would blank it on every update.
	updateCellPositionStmt = `UPDATE %s SET rat==$CellPosition.rat, mcc==$CellPosition.mcc, mnc==$CellPosition.mnc, cell_identity==$CellPosition.cell_identity, gnb_id==$CellPosition.gnb_id, latitude==$CellPosition.latitude, longitude==$CellPosition.longitude, altitude==$CellPosition.altitude, uncertainty_semi_major==$CellPosition.uncertainty_semi_major, uncertainty_semi_minor==$CellPosition.uncertainty_semi_minor, orientation_major==$CellPosition.orientation_major, confidence==$CellPosition.confidence, updated_at==$CellPosition.updated_at WHERE id==$CellPosition.id;`
	deleteCellPositionStmt = `DELETE FROM %s WHERE id==$CellPosition.id;`
)

// CellPosition maps a serving cell (NCGI for NR, ECGI for E-UTRA) to a
// provisioned geographic antenna position. It is the LMF's coordinate anchor
// for Cell-ID and E-CID positioning when the RAN does not signal its position.
//
// Coordinates are WGS-84 decimal degrees; uncertainties are in metres; the
// orientation is degrees (0..179) of the ellipse major axis; confidence is a
// percentage (0..100). CellIdentity is the hex NR Cell Identity (36-bit) or
// E-UTRA Cell Identity (28-bit), matching models.Ncgi.NrCellID / Ecgi.EutraCellID.
type CellPosition struct {
	ID                   string   `db:"id"`
	RAT                  string   `db:"rat"`
	Mcc                  string   `db:"mcc"`
	Mnc                  string   `db:"mnc"`
	CellIdentity         string   `db:"cell_identity"`
	GNbID                *string  `db:"gnb_id"`
	Latitude             float64  `db:"latitude"`
	Longitude            float64  `db:"longitude"`
	Altitude             *float64 `db:"altitude"`
	UncertaintySemiMajor *float64 `db:"uncertainty_semi_major"`
	UncertaintySemiMinor *float64 `db:"uncertainty_semi_minor"`
	OrientationMajor     *int     `db:"orientation_major"`
	Confidence           *int     `db:"confidence"`
	Source               string   `db:"source"`
	CreatedAt            string   `db:"created_at"`
	UpdatedAt            string   `db:"updated_at"`
}

func (db *Database) CreateCellPosition(ctx context.Context, c *CellPosition) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", CellPositionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", CellPositionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(CellPositionsTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(CellPositionsTableName, "insert").Inc()

	if c.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "uuid generation failed")

			return fmt.Errorf("generate cell position id: %w", err)
		}

		c.ID = id.String()
	}

	if c.Source == "" {
		c.Source = "provisioned"
	}

	c.CellIdentity = strings.ToLower(c.CellIdentity)

	now := time.Now().Format(time.RFC3339)
	c.CreatedAt = now
	c.UpdatedAt = now

	if err := db.conn().Query(ctx, db.createCellPositionStmt, c).Run(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) GetCellPosition(ctx context.Context, id string) (*CellPosition, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", CellPositionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", CellPositionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(CellPositionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(CellPositionsTableName, "select").Inc()

	var c CellPosition

	err := db.conn().Query(ctx, db.getCellPositionStmt, CellPosition{ID: id}).Get(&c)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &c, nil
}

// GetCellPositionByCell looks up a provisioned position by its serving-cell
// natural key. Returns ErrNotFound when no row matches.
func (db *Database) GetCellPositionByCell(ctx context.Context, rat, mcc, mnc, cellIdentity string) (*CellPosition, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", CellPositionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", CellPositionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(CellPositionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(CellPositionsTableName, "select").Inc()

	var c CellPosition

	filter := CellPosition{RAT: rat, Mcc: mcc, Mnc: mnc, CellIdentity: strings.ToLower(cellIdentity)}

	err := db.conn().Query(ctx, db.getCellPositionByCellStmt, filter).Get(&c)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &c, nil
}

func (db *Database) ListCellPositions(ctx context.Context) ([]CellPosition, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", CellPositionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", CellPositionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(CellPositionsTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(CellPositionsTableName, "select").Inc()

	var positions []CellPosition

	err := db.conn().Query(ctx, db.listCellPositionsStmt).GetAll(&positions)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return positions, nil
}

func (db *Database) UpdateCellPosition(ctx context.Context, c *CellPosition) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", CellPositionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", CellPositionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(CellPositionsTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(CellPositionsTableName, "update").Inc()

	c.UpdatedAt = time.Now().Format(time.RFC3339)
	c.CellIdentity = strings.ToLower(c.CellIdentity)

	var outcome sqlair.Outcome

	err := db.conn().Query(ctx, db.updateCellPositionStmt, c).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		span.SetStatus(codes.Error, "not found")
		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeleteCellPosition(ctx context.Context, id string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", CellPositionsTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", CellPositionsTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(CellPositionsTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(CellPositionsTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.conn().Query(ctx, db.deleteCellPositionStmt, CellPosition{ID: id}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		span.SetStatus(codes.Error, "not found")
		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
