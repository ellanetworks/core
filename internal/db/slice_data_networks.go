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

const SliceDataNetworksTableName = "slice_data_networks"

const sliceDataNetworkSchema = 26

const (
	listSliceDataNetworksStmt          = "SELECT &SliceDataNetwork.* FROM %s ORDER BY sliceID, dataNetworkID"
	getSliceDataNetworkStmt            = "SELECT &SliceDataNetwork.* FROM %s WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	createSliceDataNetworkStmt         = "INSERT INTO %s (sliceID, dataNetworkID, nat, localSwitch, networkInstance) VALUES ($SliceDataNetwork.sliceID, $SliceDataNetwork.dataNetworkID, $SliceDataNetwork.nat, $SliceDataNetwork.localSwitch, $SliceDataNetwork.networkInstance)"
	updateSliceDataNetworkStmt         = "UPDATE %s SET nat=$SliceDataNetwork.nat, localSwitch=$SliceDataNetwork.localSwitch, networkInstance=$SliceDataNetwork.networkInstance WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	deleteSliceDataNetworkStmt         = "DELETE FROM %s WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	deleteSliceDataNetworksOfSliceStmt = "DELETE FROM %s WHERE sliceID IN (SELECT id FROM %s WHERE name==$NetworkSlice.name)"
	deleteSliceDataNetworksOfDNStmt    = "DELETE FROM %s WHERE dataNetworkID IN (SELECT id FROM %s WHERE name==$DataNetwork.name)"
	countSliceDataNetworkPoliciesStmt  = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE sliceID==$SliceDataNetwork.sliceID AND dataNetworkID==$SliceDataNetwork.dataNetworkID"
	sliceDataNetworkForwardingQuery    = "SELECT p.sliceID, p.dataNetworkID, d.ims, d.ipv6Pool != '', p.nat, p.localSwitch, p.networkInstance FROM %s p JOIN %s d ON d.id = p.dataNetworkID"
)

type SliceDataNetwork struct {
	SliceID         string `db:"sliceID"`
	DataNetworkID   string `db:"dataNetworkID"`
	NAT             bool   `db:"nat"`
	LocalSwitch     bool   `db:"localSwitch"`
	NetworkInstance string `db:"networkInstance"`
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
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "SELECT")
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

	ctx, span, done := db.sliceDataNetworkSpan(ctx, "SELECT")
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
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "INSERT")
	defer done()

	normalizeSliceDataNetwork(pair)

	if _, err := opCreateSliceDataNetwork.Invoke(ctx, db, pair); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) error {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "UPDATE")
	defer done()

	normalizeSliceDataNetwork(pair)

	if _, err := opUpdateSliceDataNetwork.Invoke(ctx, db, pair); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeleteSliceDataNetwork(ctx context.Context, sliceID, dataNetworkID string) error {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "DELETE")
	defer done()

	if _, err := opDeleteSliceDataNetwork.Invoke(ctx, db, &SliceDataNetwork{SliceID: sliceID, DataNetworkID: dataNetworkID}); err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) ListSliceDataNetworkForwarding(ctx context.Context) ([]SliceDataNetworkForwarding, error) {
	return db.sliceDataNetworkForwarding(ctx, "", nil)
}

func (db *Database) GetSliceDataNetworkForwarding(ctx context.Context, sliceID, dataNetworkID string) (*SliceDataNetworkForwarding, error) {
	out, err := db.sliceDataNetworkForwarding(ctx, " WHERE p.sliceID = ? AND p.dataNetworkID = ?", []any{sliceID, dataNetworkID})
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, ErrNotFound
	}

	return &out[0], nil
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

type LeaseWithholding struct {
	IPv4 bool
	IPv6 bool
}

func (db *Database) WithheldLeaseOwners(ctx context.Context, nodeID string) (map[string]LeaseWithholding, error) {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "SELECT")
	defer done()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return nil, nil
	}

	query := fmt.Sprintf(`SELECT s.imsi, p.dataNetworkID, MIN((sdn.nat AND NOT d.ims) OR sdn.networkInstance NOT IN ('', ?)), MIN(sdn.networkInstance NOT IN ('', ?))
		FROM (SELECT imsi, profileID FROM %s UNION ALL SELECT v.imsi, r.profileID FROM %s v JOIN %s r ON r.id = v.partnerID) s
		JOIN %s p ON p.profileID = s.profileID
		JOIN %s sdn ON sdn.sliceID = p.sliceID AND sdn.dataNetworkID = p.dataNetworkID
		JOIN %s d ON d.id = p.dataNetworkID
		WHERE EXISTS (SELECT 1 FROM %s l WHERE l.imsi = s.imsi AND l.poolID = p.dataNetworkID AND l.nodeID = ?)
		GROUP BY s.imsi, p.dataNetworkID`,
		SubscribersTableName, VisitorsTableName, RoamingPartnersTableName, PoliciesTableName, SliceDataNetworksTableName, DataNetworksTableName, IPLeasesTableName) // #nosec: G201 -- table names are constants

	rows, err := db.conn().PlainDB().QueryContext(ctx, query, models.DefaultNetworkInstanceName, models.DefaultNetworkInstanceName, nodeID)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	defer func() { _ = rows.Close() }()

	out := make(map[string]LeaseWithholding)

	for rows.Next() {
		var (
			imsi, dataNetworkID string
			withheld            LeaseWithholding
		)

		if err := rows.Scan(&imsi, &dataNetworkID, &withheld.IPv4, &withheld.IPv6); err != nil {
			recordSpanError(span, err)

			return nil, fmt.Errorf("scan: %w", err)
		}

		out[imsi+"/"+dataNetworkID] = withheld
	}

	return out, rows.Err()
}

func (db *Database) sliceDataNetworkForwarding(ctx context.Context, where string, args []any) ([]SliceDataNetworkForwarding, error) {
	ctx, span, done := db.sliceDataNetworkSpan(ctx, "SELECT")
	defer done()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		out, err := db.legacySliceDataNetworkForwarding(ctx, args)
		if err != nil {
			recordSpanError(span, err)
		}

		return out, err
	}

	query := fmt.Sprintf(sliceDataNetworkForwardingQuery, SliceDataNetworksTableName, DataNetworksTableName) + where + " ORDER BY p.sliceID, p.dataNetworkID"

	rows, err := db.conn().PlainDB().QueryContext(ctx, query, args...)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	defer func() { _ = rows.Close() }()

	var out []SliceDataNetworkForwarding

	for rows.Next() {
		var f SliceDataNetworkForwarding

		if err := rows.Scan(&f.SliceID, &f.DataNetworkID, &f.IMS, &f.IPv6Pool, &f.NAT, &f.LocalSwitch, &f.NetworkInstance); err != nil {
			recordSpanError(span, err)

			return nil, fmt.Errorf("scan: %w", err)
		}

		out = append(out, f.effective())
	}

	return out, rows.Err()
}

func (f SliceDataNetworkForwarding) effective() SliceDataNetworkForwarding {
	f.NAT = f.NAT && !f.IMS
	f.LocalSwitch = f.LocalSwitch || f.IMS

	return f
}

func (db *Database) legacySliceDataNetworkForwarding(ctx context.Context, match []any) ([]SliceDataNetworkForwarding, error) {
	pairs, err := db.legacySliceDataNetworks(ctx)
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

	out := make([]SliceDataNetworkForwarding, 0, len(pairs))

	for _, p := range pairs {
		if len(match) == 2 && (p.SliceID != match[0] || p.DataNetworkID != match[1]) {
			continue
		}

		dn, ok := byID[p.DataNetworkID]
		if !ok {
			continue
		}

		out = append(out, SliceDataNetworkForwarding{
			SliceID:         p.SliceID,
			DataNetworkID:   p.DataNetworkID,
			IMS:             dn.IMS,
			IPv6Pool:        dn.IPv6Pool != "",
			NAT:             p.NAT,
			LocalSwitch:     p.LocalSwitch,
			NetworkInstance: p.NetworkInstance,
		}.effective())
	}

	return out, nil
}

func (db *Database) legacySliceDataNetworks(ctx context.Context) ([]SliceDataNetwork, error) {
	localSwitch, err := readLegacyToggle(ctx, db.conn().PlainDB(), LocalSwitchSettingsTableName, false)
	if err != nil {
		return nil, err
	}

	nat, err := readLegacyToggle(ctx, db.conn().PlainDB(), NATSettingsTableName, NATDefaultEnabled)
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

		p.NAT = nat && !isIMS
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

func (db *Database) sliceDataNetworkSpan(ctx context.Context, operation string) (context.Context, trace.Span, func()) {
	table := SliceDataNetworksTableName

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
