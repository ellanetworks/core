// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const PoliciesTableName = "policies"

const policyPreemptionSchema = 25

const (
	listPoliciesPagedSQL           = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault, COUNT(*) OVER() FROM " + PoliciesTableName + " LIMIT ? OFFSET ?"
	getPolicySQL                   = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault FROM " + PoliciesTableName + " WHERE name==?"
	getPolicyByLookupSQL           = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault FROM " + PoliciesTableName + " WHERE profileID==? AND sliceID==? AND dataNetworkID==?"
	createPolicySQL                = "INSERT INTO " + PoliciesTableName + " (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	editPolicySQL                  = "UPDATE " + PoliciesTableName + " SET profileID=?, sliceID=?, dataNetworkID=?, var5qi=?, arp=?, preemptionCapability=?, preemptionVulnerability=?, sessionAmbrUplink=?, sessionAmbrDownlink=? WHERE name==?"
	deletePolicySQL                = "DELETE FROM " + PoliciesTableName + " WHERE name==?"
	countPoliciesSQL               = "SELECT COUNT(*) FROM " + PoliciesTableName
	countPoliciesInProfileSQL      = "SELECT COUNT(*) FROM " + PoliciesTableName + " WHERE profileID=?"
	countPoliciesInSliceSQL        = "SELECT COUNT(*) FROM " + PoliciesTableName + " WHERE sliceID=?"
	countPoliciesInDataNetworkSQL  = "SELECT COUNT(*) FROM " + PoliciesTableName + " WHERE dataNetworkID=?"
	listPolicy5QIsInDataNetworkSQL = "SELECT var5qi FROM " + PoliciesTableName + " WHERE dataNetworkID=?"

	getPolicyByProfileAndSliceSQL = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault FROM " + PoliciesTableName + " WHERE profileID==? AND sliceID==? LIMIT 1"
	listPoliciesByProfilePagedSQL = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault, COUNT(*) OVER() FROM " + PoliciesTableName + " WHERE profileID==? LIMIT ? OFFSET ?"
	listPoliciesByProfileAllSQL   = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault FROM " + PoliciesTableName + " WHERE profileID==? ORDER BY id ASC"

	// The default data-network binding for a profile (the default APN/DNN,
	// TS 23.401 §3.1 / TS 23.501 §5.15). At most one per profile, kept by the
	// atomic SetDefaultPolicy op (clear the profile's defaults, then set one).
	getDefaultPolicyByProfileSQL = "SELECT id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink, isDefault FROM " + PoliciesTableName + " WHERE profileID==? AND isDefault==true LIMIT 1"
	clearDefaultPoliciesSQL      = "UPDATE " + PoliciesTableName + " SET isDefault=false WHERE profileID==?"
	setDefaultPolicySQL          = "UPDATE " + PoliciesTableName + " SET isDefault=true WHERE name==?"
)

const policyColumnsPreV25 = "id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink, isDefault"

const (
	listPoliciesPagedPreV25SQL          = "SELECT " + policyColumnsPreV25 + ", COUNT(*) OVER() FROM " + PoliciesTableName + " LIMIT ? OFFSET ?"
	getPolicyPreV25SQL                  = "SELECT " + policyColumnsPreV25 + " FROM " + PoliciesTableName + " WHERE name==?"
	getPolicyByLookupPreV25SQL          = "SELECT " + policyColumnsPreV25 + " FROM " + PoliciesTableName + " WHERE profileID==? AND sliceID==? AND dataNetworkID==?"
	getPolicyByProfileAndSlicePreV25SQL = "SELECT " + policyColumnsPreV25 + " FROM " + PoliciesTableName + " WHERE profileID==? AND sliceID==? LIMIT 1"
	listPoliciesByProfilePagedPreV25SQL = "SELECT " + policyColumnsPreV25 + ", COUNT(*) OVER() FROM " + PoliciesTableName + " WHERE profileID==? LIMIT ? OFFSET ?"
	listPoliciesByProfileAllPreV25SQL   = "SELECT " + policyColumnsPreV25 + " FROM " + PoliciesTableName + " WHERE profileID==? ORDER BY id ASC"
	getDefaultPolicyByProfilePreV25SQL  = "SELECT " + policyColumnsPreV25 + " FROM " + PoliciesTableName + " WHERE profileID==? AND isDefault==true LIMIT 1"
	createPolicyPreV25SQL               = "INSERT INTO " + PoliciesTableName + " (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"
	editPolicyPreV25SQL                 = "UPDATE " + PoliciesTableName + " SET profileID=?, sliceID=?, dataNetworkID=?, var5qi=?, arp=?, sessionAmbrUplink=?, sessionAmbrDownlink=? WHERE name==?"
)

type Policy struct {
	ID                      string // UUIDv7
	Name                    string
	ProfileID               string
	SliceID                 string
	DataNetworkID           string
	Var5qi                  int32
	Arp                     int32
	PreemptionCapability    models.PreemptionCapability
	PreemptionVulnerability models.PreemptionVulnerability
	SessionAmbrUplink       string
	SessionAmbrDownlink     string
	// IsDefault marks this binding as the profile's default APN/DNN — used for the
	// 4G default bearer and as the 5G fallback when no DNN is requested.
	IsDefault bool
}

func (db *Database) policySchemaCurrent(ctx context.Context, p *Policy) (bool, error) {
	if db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		return true, nil
	}

	if p.PreemptionCapability != models.PreemptionCapabilityNotPreempt || p.PreemptionVulnerability != models.PreemptionVulnerabilityNotPreemptable {
		return false, ErrMigrationPending
	}

	return false, nil
}

func createPolicyQuery(p *Policy, current bool) (string, []any) {
	if current {
		return createPolicySQL, []any{p.ID, p.Name, p.ProfileID, p.SliceID, p.DataNetworkID, p.Var5qi, p.Arp, p.PreemptionCapability, p.PreemptionVulnerability, p.SessionAmbrUplink, p.SessionAmbrDownlink}
	}

	return createPolicyPreV25SQL, []any{p.ID, p.Name, p.ProfileID, p.SliceID, p.DataNetworkID, p.Var5qi, p.Arp, p.SessionAmbrUplink, p.SessionAmbrDownlink}
}

func editPolicyQuery(p *Policy, current bool) (string, []any) {
	if current {
		return editPolicySQL, []any{p.ProfileID, p.SliceID, p.DataNetworkID, p.Var5qi, p.Arp, p.PreemptionCapability, p.PreemptionVulnerability, p.SessionAmbrUplink, p.SessionAmbrDownlink, p.Name}
	}

	return editPolicyPreV25SQL, []any{p.ProfileID, p.SliceID, p.DataNetworkID, p.Var5qi, p.Arp, p.SessionAmbrUplink, p.SessionAmbrDownlink, p.Name}
}

func defaultPoliciesPreemption(policies []Policy) {
	for i := range policies {
		policies[i].defaultPreemption()
	}
}

func (p *Policy) defaultPreemption() {
	if p.PreemptionCapability == "" {
		p.PreemptionCapability = models.PreemptionCapabilityNotPreempt
	}

	if p.PreemptionVulnerability == "" {
		p.PreemptionVulnerability = models.PreemptionVulnerabilityNotPreemptable
	}
}

func (db *Database) ListPoliciesPage(ctx context.Context, page int, perPage int) ([]Policy, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (paged)", PoliciesTableName, "SELECT", "select", attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := listPoliciesPagedSQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = listPoliciesPagedPreV25SQL, scanPolicyPreV25
	}

	policies, count, err := queryPageWithTotal(ctx, db.reader(), query, scan, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	if len(policies) == 0 {
		fallbackCount, err := db.CountPolicies(ctx)
		if err != nil {
			recordSpanError(span, err)

			return nil, 0, fmt.Errorf("count failed: %w", err)
		}

		return nil, fallbackCount, nil
	}

	defaultPoliciesPreemption(policies)

	return policies, count, nil
}

func (db *Database) ListPoliciesByProfilePage(ctx context.Context, profileID string, page int, perPage int) ([]Policy, int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (paged by profile)", PoliciesTableName, "SELECT", "select", attrs.ProfileID(profileID), attribute.Int("ella.db.page", page), attribute.Int("ella.db.page_size", perPage))
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := listPoliciesByProfilePagedSQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = listPoliciesByProfilePagedPreV25SQL, scanPolicyPreV25
	}

	policies, count, err := queryPageWithTotal(ctx, db.reader(), query, scan, profileID, perPage, (page-1)*perPage)
	if err != nil {
		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	defaultPoliciesPreemption(policies)

	return policies, count, nil
}

func (db *Database) ListPoliciesByProfile(ctx context.Context, profileID string) ([]Policy, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (all by profile)", PoliciesTableName, "SELECT", "select", attrs.ProfileID(profileID))
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := listPoliciesByProfileAllSQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = listPoliciesByProfileAllPreV25SQL, scanPolicyPreV25
	}

	policies, err := queryAll(ctx, db.reader(), query, scan, profileID)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	defaultPoliciesPreemption(policies)

	return policies, nil
}

func (db *Database) GetPolicy(ctx context.Context, name string) (*Policy, error) {
	ctx, span, timer := startDBQuery(ctx, PoliciesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := getPolicySQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = getPolicyPreV25SQL, scanPolicyPreV25
	}

	row, err := scan(db.reader().QueryRowContext(ctx, query, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	row.defaultPreemption()

	return &row, nil
}

// GetPolicyByLookup finds a policy by its profileID, sliceID, and dataNetworkID.
func (db *Database) GetPolicyByLookup(ctx context.Context, profileID, sliceID, dataNetworkID string) (*Policy, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (lookup)", PoliciesTableName, "SELECT", "select", attrs.ProfileID(profileID), attrs.SliceID(sliceID), attrs.DataNetworkID(dataNetworkID))
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := getPolicyByLookupSQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = getPolicyByLookupPreV25SQL, scanPolicyPreV25
	}

	row, err := scan(db.reader().QueryRowContext(ctx, query, profileID, sliceID, dataNetworkID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	row.defaultPreemption()

	return &row, nil
}

// GetPolicyByProfileAndSlice finds the first policy for a given profile and slice.
// Used by the AMF to resolve a default DNN when the UE does not specify one.
// GetDefaultPolicyByProfile returns the profile's default data-network binding
// (the default APN/DNN). Returns ErrNotFound when the profile has no default.
func (db *Database) GetDefaultPolicyByProfile(ctx context.Context, profileID string) (*Policy, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (default by profile)", PoliciesTableName, "SELECT", "select", attrs.ProfileID(profileID))
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := getDefaultPolicyByProfileSQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = getDefaultPolicyByProfilePreV25SQL, scanPolicyPreV25
	}

	row, err := scan(db.reader().QueryRowContext(ctx, query, profileID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	row.defaultPreemption()

	return &row, nil
}

// SetDefaultPolicy makes the named policy the sole default binding of the given
// profile (the default APN/DNN), clearing any previous default in one atomic op.
func (db *Database) SetDefaultPolicy(ctx context.Context, profileID, name string) error {
	ctx, span, timer := startDBQueryNamed(ctx, "UPDATE "+PoliciesTableName+" (set default)", PoliciesTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opSetDefaultPolicy.Invoke(ctx, db, &Policy{ProfileID: profileID, Name: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*Policy, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (by profile+slice)", PoliciesTableName, "SELECT", "select", attrs.ProfileID(profileID), attrs.SliceID(sliceID))
	defer span.End()
	defer timer.ObserveDuration()

	query, scan := getPolicyByProfileAndSliceSQL, scanPolicy
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		query, scan = getPolicyByProfileAndSlicePreV25SQL, scanPolicyPreV25
	}

	row, err := scan(db.reader().QueryRowContext(ctx, query, profileID, sliceID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	row.defaultPreemption()

	return &row, nil
}

// GetSessionPolicy resolves a subscriber's policy for a given slice (sst+sd) and DNN.
// It follows the chain: subscriber → profileID → policy (by profile, slice, DNN).
func (db *Database) GetSessionPolicy(ctx context.Context, imsi string, sst int32, sd string, dnn string) (*Policy, error) {
	ctx, span := tracer.Start(ctx, "db/get_session_policy",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attrs.SUPIFromIMSI(imsi),
			attrs.SST(sst),
			attrs.SD(sd),
			attrs.DNN(dnn),
		),
	)
	defer span.End()

	sub, err := db.GetSubscriberOrVisitor(ctx, imsi)
	if err != nil {
		recordSpanError(span, err)

		if errors.Is(err, ErrNotFound) {
			return nil, ErrSubscriberNotFound
		}

		return nil, fmt.Errorf("subscriber not found: %w", err)
	}

	policies, err := db.ListPoliciesByProfile(ctx, sub.ProfileID)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("list policies for profile %s: %w", sub.ProfileID, err)
	}

	// Batch-fetch all referenced network slices.
	sliceIDSet := make(map[string]struct{})
	for _, p := range policies {
		sliceIDSet[p.SliceID] = struct{}{}
	}

	sliceIDs := make([]string, 0, len(sliceIDSet))
	for id := range sliceIDSet {
		sliceIDs = append(sliceIDs, id)
	}

	sliceList, err := db.ListNetworkSlicesByIDs(ctx, sliceIDs)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("list slices by IDs: %w", err)
	}

	sliceMap := make(map[string]NetworkSlice, len(sliceList))
	for _, s := range sliceList {
		sliceMap[s.ID] = s
	}

	sliceMatched := false

	for _, p := range policies {
		slice, ok := sliceMap[p.SliceID]
		if !ok {
			continue
		}

		sliceSd := ""
		if slice.Sd != nil {
			sliceSd = *slice.Sd
		}

		// A slice provisioned before SDs were stored canonical still holds the
		// operator's spelling.
		if slice.Sst != sst || models.NormalizeSD(sliceSd) != models.NormalizeSD(sd) {
			continue
		}

		sliceMatched = true

		dataNetwork, err := db.GetDataNetworkByID(ctx, p.DataNetworkID)
		if err != nil {
			recordSpanError(span, err)

			return nil, fmt.Errorf("couldn't get data network %s: %w", p.DataNetworkID, err)
		}

		if dataNetwork.Name != dnn {
			continue
		}

		return &p, nil
	}

	// A matched slice with no policy for the DNN is a DNN problem; an unmatched
	// slice is a slice problem (TS 24.501 §9.11.4.2 #70 vs the generic reject).
	err = fmt.Errorf("no policy matching sst=%d sd=%q dnn=%q for profile %s: %w", sst, sd, dnn, sub.ProfileID, ErrNoMatchingPolicy)
	if sliceMatched {
		err = fmt.Errorf("no data network %q in slice sst=%d sd=%q for profile %s: %w", dnn, sst, sd, sub.ProfileID, ErrDNNNotInSlice)
	}

	recordSpanError(span, err)

	return nil, err
}

func (db *Database) CreatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span, timer := startDBQuery(ctx, PoliciesTableName, "INSERT", "insert")
	defer span.End()
	defer timer.ObserveDuration()

	if policy.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			recordSpanError(span, err)

			return fmt.Errorf("generate policy id: %w", err)
		}

		policy.ID = id.String()
	}

	_, err := opCreatePolicy.Invoke(ctx, db, policy)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) UpdatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span, timer := startDBQuery(ctx, PoliciesTableName, "UPDATE", "update")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opUpdatePolicy.Invoke(ctx, db, policy)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeletePolicy(ctx context.Context, name string) error {
	ctx, span, timer := startDBQuery(ctx, PoliciesTableName, "DELETE", "delete")
	defer span.End()
	defer timer.ObserveDuration()

	_, err := opDeletePolicy.Invoke(ctx, db, &stringPayload{Value: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// CountPolicies returns policy count
func (db *Database) CountPolicies(ctx context.Context) (int, error) {
	ctx, span, timer := startDBQuery(ctx, PoliciesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countPoliciesSQL)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountPoliciesInProfile(ctx context.Context, profileID string) (int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (by profile)", PoliciesTableName, "SELECT", "select", attrs.ProfileID(profileID))
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countPoliciesInProfileSQL, profileID)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountPoliciesInSlice(ctx context.Context, sliceID string) (int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (by slice)", PoliciesTableName, "SELECT", "select", attrs.SliceID(sliceID))
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countPoliciesInSliceSQL, sliceID)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) CountPoliciesInDataNetwork(ctx context.Context, dataNetworkID string) (int, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (by data network)", PoliciesTableName, "SELECT", "select", attrs.DataNetworkID(dataNetworkID))
	defer span.End()
	defer timer.ObserveDuration()

	count, err := queryCount(ctx, db.reader(), countPoliciesInDataNetworkSQL, dataNetworkID)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return count, nil
}

func (db *Database) DataNetworkPolicy5QIs(ctx context.Context, dataNetworkID string) ([]int32, error) {
	ctx, span, timer := startDBQueryNamed(ctx, "SELECT "+PoliciesTableName+" (5QIs in data network)", PoliciesTableName, "SELECT", "select")
	defer span.End()
	defer timer.ObserveDuration()

	fiveQIs, err := queryAll(ctx, db.reader(), listPolicy5QIsInDataNetworkSQL, scanValue[int32], dataNetworkID)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if fiveQIs == nil {
		return []int32{}, nil
	}

	return fiveQIs, nil
}

func (db *Database) PoliciesInDataNetwork(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(ctx, "db/policies_in_data_network",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	dataNetwork, err := db.GetDataNetwork(ctx, name)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("data network not found: %w", err)
	}

	count, err := db.CountPoliciesInDataNetwork(ctx, dataNetwork.ID)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("counting failed: %w", err)
	}

	return count > 0, nil
}

func (db *Database) PoliciesInSlice(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(ctx, "db/policies_in_slice",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	slice, err := db.GetNetworkSlice(ctx, name)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("slice not found: %w", err)
	}

	count, err := db.CountPoliciesInSlice(ctx, slice.ID)
	if err != nil {
		recordSpanError(span, err)

		return false, fmt.Errorf("counting failed: %w", err)
	}

	return count > 0, nil
}

func scanPolicy(row scanner) (Policy, error) {
	var v Policy

	if err := row.Scan(&v.ID, &v.Name, &v.ProfileID, &v.SliceID, &v.DataNetworkID, &v.Var5qi, &v.Arp, &v.PreemptionCapability, &v.PreemptionVulnerability, &v.SessionAmbrUplink, &v.SessionAmbrDownlink, &v.IsDefault); err != nil {
		return Policy{}, err
	}

	return v, nil
}

func scanPolicyPreV25(row scanner) (Policy, error) {
	var v Policy

	if err := row.Scan(&v.ID, &v.Name, &v.ProfileID, &v.SliceID, &v.DataNetworkID, &v.Var5qi, &v.Arp, &v.SessionAmbrUplink, &v.SessionAmbrDownlink, &v.IsDefault); err != nil {
		return Policy{}, err
	}

	return v, nil
}
