// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/models"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	SliceDataNetworksTableName   = "slice_data_networks"
	SliceDataNetworkNATTableName = "slice_data_network_nat"
)

const sliceDataNetworkSchema = 26

const (
	listSliceDataNetworksStmt          = "SELECT &SliceDataNetwork.* FROM %s ORDER BY sliceID, dataNetworkID"
	getSliceDataNetworkStmt            = "SELECT &SliceDataNetwork.* FROM %s WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	createSliceDataNetworkStmt         = "INSERT INTO %s (sliceID, dataNetworkID, localSwitch, networkInstance) VALUES ($SliceDataNetwork.sliceID, $SliceDataNetwork.dataNetworkID, $SliceDataNetwork.localSwitch, $SliceDataNetwork.networkInstance)"
	updateSliceDataNetworkStmt         = "UPDATE %s SET localSwitch=$SliceDataNetwork.localSwitch, networkInstance=$SliceDataNetwork.networkInstance WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	deleteSliceDataNetworkStmt         = "DELETE FROM %s WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	deleteSliceDataNetworksOfSliceStmt = "DELETE FROM %s WHERE sliceID IN (SELECT id FROM %s WHERE name==$NetworkSlice.name)"
	deleteSliceDataNetworksOfDNStmt    = "DELETE FROM %s WHERE dataNetworkID IN (SELECT id FROM %s WHERE name==$DataNetwork.name)"
	countSliceDataNetworkPoliciesStmt  = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	listSliceDataNetworkNATStmt        = "SELECT &SliceDataNetworkNAT.* FROM %s"
	upsertSliceDataNetworkNATStmt      = "INSERT INTO %s (sliceID, dataNetworkID, enabled) VALUES ($SliceDataNetworkNAT.sliceID, $SliceDataNetworkNAT.dataNetworkID, $SliceDataNetworkNAT.enabled) ON CONFLICT(sliceID, dataNetworkID) DO UPDATE SET enabled=$SliceDataNetworkNAT.enabled"
	deleteSliceDataNetworkNATStmt      = "DELETE FROM %s WHERE sliceID==$SliceDataNetworkNAT.sliceID AND dataNetworkID==$SliceDataNetworkNAT.dataNetworkID"
)

type SliceDataNetwork struct {
	SliceID         string `db:"sliceID"`
	DataNetworkID   string `db:"dataNetworkID"`
	LocalSwitch     bool   `db:"localSwitch"`
	NetworkInstance string `db:"networkInstance"`
}

type SliceDataNetworkNAT struct {
	SliceID       string `db:"sliceID"`
	DataNetworkID string `db:"dataNetworkID"`
	Enabled       bool   `db:"enabled"`
}

type SliceDataNetworkForwarding struct {
	SliceID         string
	DataNetworkID   string
	IMS             bool
	IPv6Pool        bool
	NAT             bool
	LocalSwitch     bool
	NetworkInstance string
}

func (f SliceDataNetworkForwarding) Key() string {
	return f.SliceID + "/" + f.DataNetworkID
}

func (db *Database) ListSliceDataNetworks(ctx context.Context) ([]SliceDataNetwork, error) {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "SELECT", SliceDataNetworksTableName)
	defer done()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		pairs, err := db.legacySliceDataNetworks(ctx)
		if err != nil {
			recordSpanError(span, err)
		}

		return pairs, err
	}

	var pairs []SliceDataNetwork

	if err := db.conn().Query(ctx, db.listSliceDataNetworksStmt).GetAll(&pairs); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return pairs, nil
}

func (db *Database) GetSliceDataNetwork(ctx context.Context, sliceID, dataNetworkID string) (*SliceDataNetwork, error) {
	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		pairs, err := db.ListSliceDataNetworks(ctx)
		if err != nil {
			return nil, err
		}

		for i := range pairs {
			if pairs[i].SliceID == sliceID && pairs[i].DataNetworkID == dataNetworkID {
				return &pairs[i], nil
			}
		}

		return nil, ErrNotFound
	}

	ctx, span, done := db.sliceDataNetworkSpan(ctx, "SELECT", SliceDataNetworksTableName)
	defer done()

	row := SliceDataNetwork{SliceID: sliceID, DataNetworkID: dataNetworkID}

	if err := db.conn().Query(ctx, db.getSliceDataNetworkStmt, row).Get(&row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) error {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "INSERT", SliceDataNetworksTableName)
	defer done()

	normalizeSliceDataNetwork(pair)

	if _, err := opCreateSliceDataNetwork.Invoke(ctx, db, pair); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) error {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "UPDATE", SliceDataNetworksTableName)
	defer done()

	normalizeSliceDataNetwork(pair)

	if _, err := opUpdateSliceDataNetwork.Invoke(ctx, db, pair); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteSliceDataNetwork(ctx context.Context, sliceID, dataNetworkID string) error {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "DELETE", SliceDataNetworksTableName)
	defer done()

	if _, err := opDeleteSliceDataNetwork.Invoke(ctx, db, &SliceDataNetwork{SliceID: sliceID, DataNetworkID: dataNetworkID}); err != nil {
		recordSpanError(span, err)

		return err
	}

	if err := db.conn().Query(ctx, db.deleteSliceDataNetworkNATStmt, SliceDataNetworkNAT{SliceID: sliceID, DataNetworkID: dataNetworkID}).Run(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("delete the NAT setting: %w", err)
	}

	db.publishOpTopics([]Topic{TopicSliceDataNetworkNAT})

	return nil
}

func (db *Database) SetSliceDataNetworkNAT(ctx context.Context, sliceID, dataNetworkID string, enabled bool) error {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "UPSERT", SliceDataNetworkNATTableName)
	defer done()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return ErrMigrationPending
	}

	row := SliceDataNetworkNAT{SliceID: sliceID, DataNetworkID: dataNetworkID, Enabled: enabled}

	if err := db.conn().Query(ctx, db.upsertSliceDataNetworkNATStmt, row).Run(); err != nil {
		recordSpanError(span, err)

		return fmt.Errorf("query failed: %w", err)
	}

	db.publishOpTopics([]Topic{TopicSliceDataNetworkNAT})

	return nil
}

func (db *Database) ListSliceDataNetworkForwarding(ctx context.Context) ([]SliceDataNetworkForwarding, error) {
	pairs, err := db.ListSliceDataNetworks(ctx)
	if err != nil {
		return nil, err
	}

	dataNetworks, err := db.ListAllDataNetworks(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*DataNetwork, len(dataNetworks))
	for i := range dataNetworks {
		byID[dataNetworks[i].ID] = &dataNetworks[i]
	}

	natOverrides, natFallback, err := db.sliceDataNetworkNAT(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]SliceDataNetworkForwarding, 0, len(pairs))

	for _, p := range pairs {
		dn, ok := byID[p.DataNetworkID]
		if !ok {
			continue
		}

		nat, ok := natOverrides[p.SliceID+"/"+p.DataNetworkID]
		if !ok {
			nat = natFallback
		}

		out = append(out, SliceDataNetworkForwarding{
			SliceID:         p.SliceID,
			DataNetworkID:   p.DataNetworkID,
			IMS:             dn.IMS,
			IPv6Pool:        dn.IPv6Pool != "",
			NAT:             nat && !dn.IMS,
			LocalSwitch:     p.LocalSwitch || dn.IMS,
			NetworkInstance: p.NetworkInstance,
		})
	}

	return out, nil
}

func (db *Database) GetSliceDataNetworkForwarding(ctx context.Context, sliceID, dataNetworkID string) (*SliceDataNetworkForwarding, error) {
	all, err := db.ListSliceDataNetworkForwarding(ctx)
	if err != nil {
		return nil, err
	}

	for i := range all {
		if all[i].SliceID == sliceID && all[i].DataNetworkID == dataNetworkID {
			return &all[i], nil
		}
	}

	return nil, ErrNotFound
}

func (db *Database) NATCoversEveryDataNetwork(ctx context.Context) (bool, error) {
	forwarding, err := db.ListSliceDataNetworkForwarding(ctx)
	if err != nil {
		return false, err
	}

	for _, f := range forwarding {
		if f.IPv6Pool || !f.NAT {
			return false, nil
		}
	}

	return true, nil
}

func (db *Database) sliceDataNetworkNAT(ctx context.Context) (map[string]bool, bool, error) {
	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		enabled, err := readLegacyToggle(ctx, db.conn().PlainDB(), NATSettingsTableName, NATDefaultEnabled)

		return nil, enabled, err
	}

	var rows []SliceDataNetworkNAT

	if err := db.conn().Query(ctx, db.listSliceDataNetworkNATStmt).GetAll(&rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("query failed: %w", err)
	}

	overrides := make(map[string]bool, len(rows))
	for _, r := range rows {
		overrides[r.SliceID+"/"+r.DataNetworkID] = r.Enabled
	}

	return overrides, NATDefaultEnabled, nil
}

func (db *Database) legacySliceDataNetworks(ctx context.Context) ([]SliceDataNetwork, error) {
	localSwitch, err := readLegacyToggle(ctx, db.conn().PlainDB(), LocalSwitchSettingsTableName, false)
	if err != nil {
		return nil, err
	}

	ims := "d.name = '" + models.IMSDataNetworkName + "'"
	if db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		ims = "d.ims"
	}

	query := fmt.Sprintf("SELECT DISTINCT p.sliceID, p.dataNetworkID, %s FROM %s p JOIN %s d ON d.id = p.dataNetworkID ORDER BY p.sliceID, p.dataNetworkID", ims, PoliciesTableName, DataNetworksTableName) // #nosec: G201 -- table names and the IMS expression are constants

	rows, err := db.conn().PlainDB().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var pairs []SliceDataNetwork

	for rows.Next() {
		var (
			p     SliceDataNetwork
			isIMS bool
		)

		if err := rows.Scan(&p.SliceID, &p.DataNetworkID, &isIMS); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		p.LocalSwitch = isIMS || localSwitch
		p.NetworkInstance = models.DefaultNetworkInstanceName
		pairs = append(pairs, p)
	}

	return pairs, rows.Err()
}

func (db *Database) applyCreateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (any, error) {
	if err := db.runner(ctx).Query(ctx, db.createSliceDataNetworkStmt, pair).Run(); err != nil {
		if isUniqueNameError(err) || isPrimaryKeyError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.updateSliceDataNetworkStmt, pair).Get(&outcome); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, requireRowAffected(outcome)
}

func (db *Database) applyDeleteSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (any, error) {
	var count NumItems

	if err := db.runner(ctx).Query(ctx, db.countSliceDataNetworkPoliciesStmt, pair).Get(&count); err != nil {
		return nil, fmt.Errorf("count policies: %w", err)
	}

	if count.Count > 0 {
		return nil, ErrInUse
	}

	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteSliceDataNetworkStmt, pair).Get(&outcome); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, requireRowAffected(outcome)
}

func (db *Database) deleteSliceDataNetworksOfSlice(ctx context.Context, name string) error {
	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return nil
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSliceDataNetworksOfSliceStmt, NetworkSlice{Name: name}).Run(); err != nil {
		return fmt.Errorf("delete the data networks of slice %s: %w", name, err)
	}

	return nil
}

func (db *Database) deleteSliceDataNetworksOfDataNetwork(ctx context.Context, name string) error {
	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return nil
	}

	if err := db.runner(ctx).Query(ctx, db.deleteSliceDataNetworksOfDNStmt, DataNetwork{Name: name}).Run(); err != nil {
		return fmt.Errorf("delete the slices of data network %s: %w", name, err)
	}

	return nil
}

func requireRowAffected(outcome sqlair.Outcome) error {
	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func normalizeSliceDataNetwork(pair *SliceDataNetwork) {
	if pair.NetworkInstance == "" {
		pair.NetworkInstance = models.DefaultNetworkInstanceName
	}
}

func (db *Database) sliceDataNetworkSpan(ctx context.Context, operation, table string) (context.Context, trace.Span, func()) {
	querySummary := fmt.Sprintf("%s %s", operation, table)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(table),
		),
	)

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(table, operation))

	DBQueriesTotal.WithLabelValues(table, operation).Inc()

	return ctx, span, func() {
		timer.ObserveDuration()
		span.End()
	}
}
