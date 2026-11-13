// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

const SliceDataNetworksTableName = "slice_data_networks"

const sliceDataNetworkSchema = 26

const (
	listSliceDataNetworksSQL          = "SELECT sliceID, dataNetworkID, nat, localSwitch, networkInstance FROM " + SliceDataNetworksTableName + " ORDER BY sliceID, dataNetworkID"
	getSliceDataNetworkSQL            = "SELECT sliceID, dataNetworkID, nat, localSwitch, networkInstance FROM " + SliceDataNetworksTableName + " WHERE sliceID==? AND dataNetworkID==?"
	createSliceDataNetworkSQL         = "INSERT INTO " + SliceDataNetworksTableName + " (sliceID, dataNetworkID, nat, localSwitch, networkInstance) VALUES (?, ?, ?, ?, ?)"
	updateSliceDataNetworkSQL         = "UPDATE " + SliceDataNetworksTableName + " SET nat=?, localSwitch=?, networkInstance=? WHERE sliceID==? AND dataNetworkID==?"
	deleteSliceDataNetworkSQL         = "DELETE FROM " + SliceDataNetworksTableName + " WHERE sliceID==? AND dataNetworkID==?"
	deleteSliceDataNetworksOfSliceSQL = "DELETE FROM " + SliceDataNetworksTableName + " WHERE sliceID IN (SELECT id FROM " + NetworkSlicesTableName + " WHERE name==?)"
	deleteSliceDataNetworksOfDNSQL    = "DELETE FROM " + SliceDataNetworksTableName + " WHERE dataNetworkID IN (SELECT id FROM " + DataNetworksTableName + " WHERE name==?)"
	countSliceDataNetworkPoliciesSQL  = "SELECT COUNT(*) FROM " + PoliciesTableName + " WHERE sliceID==? AND dataNetworkID==?"
	sliceDataNetworkForwardingQuery   = "SELECT p.sliceID, p.dataNetworkID, d.ims, d.ipv6Pool != '', p.nat, p.localSwitch, p.networkInstance FROM %s p JOIN %s d ON d.id = p.dataNetworkID"
)

type SliceDataNetwork struct {
	SliceID         string
	DataNetworkID   string
	NAT             bool
	LocalSwitch     bool
	NetworkInstance string
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

func (db *Database) ListSliceDataNetworks(ctx context.Context) (_ []SliceDataNetwork, err error) {
	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		pairs, err := db.legacySliceDataNetworks(ctx)
		return pairs, err
	}

	pairs, err := queryAll(ctx, db.reader(), listSliceDataNetworksSQL, scanSliceDataNetwork)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return pairs, nil
}

func (db *Database) GetSliceDataNetwork(ctx context.Context, sliceID, dataNetworkID string) (_ *SliceDataNetwork, err error) {
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

	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanSliceDataNetwork(db.reader().QueryRowContext(ctx, getSliceDataNetworkSQL, sliceID, dataNetworkID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &row, nil
}

func (db *Database) CreateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (err error) {
	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "INSERT")
	defer func() { finish(err) }()

	normalizeSliceDataNetwork(pair)

	if _, err := opCreateSliceDataNetwork.Invoke(ctx, db, pair); err != nil {
		return err
	}

	return nil
}

func (db *Database) UpdateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (err error) {
	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "UPDATE")
	defer func() { finish(err) }()

	normalizeSliceDataNetwork(pair)

	if _, err := opUpdateSliceDataNetwork.Invoke(ctx, db, pair); err != nil {
		return err
	}

	return nil
}

func (db *Database) DeleteSliceDataNetwork(ctx context.Context, sliceID, dataNetworkID string) (err error) {
	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "DELETE")
	defer func() { finish(err) }()

	if _, err := opDeleteSliceDataNetwork.Invoke(ctx, db, &SliceDataNetwork{SliceID: sliceID, DataNetworkID: dataNetworkID}); err != nil {
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

func (db *Database) WithheldLeaseOwners(ctx context.Context, nodeID string) (_ map[string]LeaseWithholding, err error) {
	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return nil, nil
	}

	owners := SubscribersTableName
	if db.appliedSchemaAtLeast(ctx, roamingSchema) {
		owners = fmt.Sprintf("(SELECT imsi, profileID FROM %s UNION ALL SELECT v.imsi, r.profileID FROM %s v JOIN %s r ON r.id = v.partnerID)", SubscribersTableName, VisitorsTableName, RoamingPartnersTableName)
	}

	query := fmt.Sprintf(`SELECT s.imsi, p.dataNetworkID, MIN((sdn.nat AND NOT d.ims) OR sdn.networkInstance NOT IN ('', ?)), MIN(sdn.networkInstance NOT IN ('', ?))
		FROM %s s
		JOIN %s p ON p.profileID = s.profileID
		JOIN %s sdn ON sdn.sliceID = p.sliceID AND sdn.dataNetworkID = p.dataNetworkID
		JOIN %s d ON d.id = p.dataNetworkID
		WHERE EXISTS (SELECT 1 FROM %s l WHERE l.imsi = s.imsi AND l.poolID = p.dataNetworkID AND l.nodeID = ?)
		GROUP BY s.imsi, p.dataNetworkID`,
		owners, PoliciesTableName, SliceDataNetworksTableName, DataNetworksTableName, IPLeasesTableName) // #nosec: G201 -- table names are constants

	rows, err := queryAll(ctx, db.reader(), query, scanWithheldLease, models.DefaultNetworkInstanceName, models.DefaultNetworkInstanceName, nodeID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	out := make(map[string]LeaseWithholding, len(rows))

	for _, r := range rows {
		out[r.imsi+"/"+r.dataNetworkID] = r.withheld
	}

	return out, nil
}

func (db *Database) sliceDataNetworkForwarding(ctx context.Context, where string, args []any) (_ []SliceDataNetworkForwarding, err error) {
	ctx, finish := startDBQuery(ctx, SliceDataNetworksTableName, "SELECT")
	defer func() { finish(err) }()

	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		out, err := db.legacySliceDataNetworkForwarding(ctx, args)
		return out, err
	}

	query := fmt.Sprintf(sliceDataNetworkForwardingQuery, SliceDataNetworksTableName, DataNetworksTableName) + where + " ORDER BY p.sliceID, p.dataNetworkID"

	out, err := queryAll(ctx, db.reader(), query, scanSliceDataNetworkForwarding, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return out, nil
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
	localSwitch, err := readLegacyToggle(ctx, db.reader(), LocalSwitchSettingsTableName, false)
	if err != nil {
		return nil, err
	}

	nat, err := readLegacyToggle(ctx, db.reader(), NATSettingsTableName, NATDefaultEnabled)
	if err != nil {
		return nil, err
	}

	ims := "d.name = '" + models.IMSDataNetworkName + "'"
	if db.appliedSchemaAtLeast(ctx, dataNetworkIMSSchema) {
		ims = "d.ims"
	}

	query := fmt.Sprintf("SELECT DISTINCT p.sliceID, p.dataNetworkID, %s FROM %s p JOIN %s d ON d.id = p.dataNetworkID ORDER BY p.sliceID, p.dataNetworkID", ims, PoliciesTableName, DataNetworksTableName) // #nosec: G201 -- table names and the IMS expression are constants

	pairs, err := queryAll(ctx, db.reader(), query, func(row scanner) (SliceDataNetwork, error) {
		var (
			p     SliceDataNetwork
			isIMS bool
		)

		if err := row.Scan(&p.SliceID, &p.DataNetworkID, &isIMS); err != nil {
			return SliceDataNetwork{}, err
		}

		p.NAT = nat && !isIMS
		p.LocalSwitch = isIMS || localSwitch
		p.NetworkInstance = models.DefaultNetworkInstanceName

		return p, nil
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return pairs, nil
}

func (db *Database) applyCreateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, createSliceDataNetworkSQL, pair.SliceID, pair.DataNetworkID, pair.NAT, pair.LocalSwitch, pair.NetworkInstance); err != nil {
		if isUniqueNameError(err) || isPrimaryKeyError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyUpdateSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, updateSliceDataNetworkSQL, pair.NAT, pair.LocalSwitch, pair.NetworkInstance, pair.SliceID, pair.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, requireRowAffected(res)
}

func (db *Database) applyDeleteSliceDataNetwork(ctx context.Context, pair *SliceDataNetwork) (any, error) {
	count, err := queryCount(ctx, db.runner(ctx), countSliceDataNetworkPoliciesSQL, pair.SliceID, pair.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("count policies: %w", err)
	}

	if count > 0 {
		return nil, ErrInUse
	}

	res, err := db.runner(ctx).ExecContext(ctx, deleteSliceDataNetworkSQL, pair.SliceID, pair.DataNetworkID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, requireRowAffected(res)
}

func (db *Database) deleteSliceDataNetworksOfSlice(ctx context.Context, name string) error {
	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return nil
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSliceDataNetworksOfSliceSQL, name); err != nil {
		return fmt.Errorf("delete the data networks of slice %s: %w", name, err)
	}

	return nil
}

func (db *Database) deleteSliceDataNetworksOfDataNetwork(ctx context.Context, name string) error {
	if !db.appliedSchemaAtLeast(ctx, sliceDataNetworkSchema) {
		return nil
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteSliceDataNetworksOfDNSQL, name); err != nil {
		return fmt.Errorf("delete the slices of data network %s: %w", name, err)
	}

	return nil
}

func requireRowAffected(res sql.Result) error {
	rowsAffected, err := res.RowsAffected()
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

func scanSliceDataNetwork(row scanner) (SliceDataNetwork, error) {
	var v SliceDataNetwork

	if err := row.Scan(&v.SliceID, &v.DataNetworkID, &v.NAT, &v.LocalSwitch, &v.NetworkInstance); err != nil {
		return SliceDataNetwork{}, err
	}

	return v, nil
}

type withheldLease struct {
	imsi          string
	dataNetworkID string
	withheld      LeaseWithholding
}

func scanWithheldLease(row scanner) (withheldLease, error) {
	var v withheldLease

	if err := row.Scan(&v.imsi, &v.dataNetworkID, &v.withheld.IPv4, &v.withheld.IPv6); err != nil {
		return withheldLease{}, err
	}

	return v, nil
}

func scanSliceDataNetworkForwarding(row scanner) (SliceDataNetworkForwarding, error) {
	var f SliceDataNetworkForwarding

	if err := row.Scan(&f.SliceID, &f.DataNetworkID, &f.IMS, &f.IPv6Pool, &f.NAT, &f.LocalSwitch, &f.NetworkInstance); err != nil {
		return SliceDataNetworkForwarding{}, err
	}

	return f.effective(), nil
}
