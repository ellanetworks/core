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

	"github.com/google/uuid"
)

const (
	CellPositionsTableName = "cell_positions"

	// RATNR and RATEUTRA identify the radio access technology a cell position
	// applies to (NCGI vs ECGI).
	RATNR    = "nr"
	RATEUTRA = "eutra"
)

const (
	putCellPositionSQL       = `INSERT INTO ` + CellPositionsTableName + ` (id, rat, mcc, mnc, cell_identity, gnb_id, latitude, longitude, altitude, uncertainty_semi_major, uncertainty_semi_minor, orientation_major, confidence, source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(id) DO UPDATE SET rat=excluded.rat, mcc=excluded.mcc, mnc=excluded.mnc, cell_identity=excluded.cell_identity, gnb_id=excluded.gnb_id, latitude=excluded.latitude, longitude=excluded.longitude, altitude=excluded.altitude, uncertainty_semi_major=excluded.uncertainty_semi_major, uncertainty_semi_minor=excluded.uncertainty_semi_minor, orientation_major=excluded.orientation_major, confidence=excluded.confidence, updated_at=excluded.updated_at;`
	getCellPositionSQL       = `SELECT id, rat, mcc, mnc, cell_identity, gnb_id, latitude, longitude, altitude, uncertainty_semi_major, uncertainty_semi_minor, orientation_major, confidence, source, created_at, updated_at FROM ` + CellPositionsTableName + ` WHERE id==?;`
	getCellPositionByCellSQL = `SELECT id, rat, mcc, mnc, cell_identity, gnb_id, latitude, longitude, altitude, uncertainty_semi_major, uncertainty_semi_minor, orientation_major, confidence, source, created_at, updated_at FROM ` + CellPositionsTableName + ` WHERE rat==? AND mcc==? AND mnc==? AND cell_identity==?;`
	listCellPositionsSQL     = `SELECT id, rat, mcc, mnc, cell_identity, gnb_id, latitude, longitude, altitude, uncertainty_semi_major, uncertainty_semi_minor, orientation_major, confidence, source, created_at, updated_at FROM ` + CellPositionsTableName + ` ORDER BY created_at DESC;`
	// source is intentionally excluded from the SET clause: it records how the
	// row originated (currently always "provisioned") and must not change on
	// edit. CellPositionRequest has no Source field, so leaving it in the SET
	// clause would blank it on every update.
	deleteCellPositionByCellSQL = `DELETE FROM ` + CellPositionsTableName + ` WHERE id!=? AND rat==? AND mcc==? AND mnc==? AND cell_identity==?;`
	deleteCellPositionSQL       = `DELETE FROM ` + CellPositionsTableName + ` WHERE id==?;`
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
	ID                   string
	RAT                  string
	Mcc                  string
	Mnc                  string
	CellIdentity         string
	GNbID                *string
	Latitude             float64
	Longitude            float64
	Altitude             *float64
	UncertaintySemiMajor *float64
	UncertaintySemiMinor *float64
	OrientationMajor     *int
	Confidence           *int
	Source               string
	CreatedAt            string
	UpdatedAt            string
}

func (db *Database) CreateCellPosition(ctx context.Context, c *CellPosition) error {
	ctx, span, timer := startDBQuery(ctx, CellPositionsTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if c.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

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

	if _, err := opCreateCellPosition.Invoke(ctx, db, c); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) GetCellPosition(ctx context.Context, id string) (*CellPosition, error) {
	ctx, span, timer := startDBQuery(ctx, CellPositionsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	c, err := scanCellPosition(db.reader().QueryRowContext(ctx, getCellPositionSQL, id))
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &c, nil
}

// GetCellPositionByCell looks up a provisioned position by its serving-cell
// natural key. Returns ErrNotFound when no row matches.
func (db *Database) GetCellPositionByCell(ctx context.Context, rat, mcc, mnc, cellIdentity string) (*CellPosition, error) {
	ctx, span, timer := startDBQuery(ctx, CellPositionsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	filter := CellPosition{RAT: rat, Mcc: mcc, Mnc: mnc, CellIdentity: strings.ToLower(cellIdentity)}

	c, err := scanCellPosition(db.reader().QueryRowContext(ctx, getCellPositionByCellSQL, filter.RAT, filter.Mcc, filter.Mnc, filter.CellIdentity))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &c, nil
}

func (db *Database) ListCellPositions(ctx context.Context) ([]CellPosition, error) {
	ctx, span, timer := startDBQuery(ctx, CellPositionsTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	positions, err := queryAll(ctx, db.reader(), listCellPositionsSQL, scanCellPosition)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return positions, nil
}

func (db *Database) UpdateCellPosition(ctx context.Context, c *CellPosition) error {
	ctx, span, timer := startDBQuery(ctx, CellPositionsTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	c.UpdatedAt = time.Now().Format(time.RFC3339)
	c.CreatedAt = c.UpdatedAt
	c.CellIdentity = strings.ToLower(c.CellIdentity)

	if c.Source == "" {
		c.Source = "provisioned"
	}

	if _, err := opUpdateCellPosition.Invoke(ctx, db, c); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteCellPosition(ctx context.Context, id string) error {
	ctx, span, timer := startDBQuery(ctx, CellPositionsTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	if _, err := opDeleteCellPosition.Invoke(ctx, db, &stringPayload{Value: id}); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func scanCellPosition(row scanner) (CellPosition, error) {
	var v CellPosition

	if err := row.Scan(&v.ID, &v.RAT, &v.Mcc, &v.Mnc, &v.CellIdentity, &v.GNbID, &v.Latitude, &v.Longitude, &v.Altitude, &v.UncertaintySemiMajor, &v.UncertaintySemiMinor, &v.OrientationMajor, &v.Confidence, &v.Source, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return CellPosition{}, err
	}

	return v, nil
}
