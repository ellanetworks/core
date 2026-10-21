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
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const PoliciesTableName = "policies"

const policyPreemptionSchema = 25

const (
	listPoliciesPagedStmt           = "SELECT &Policy.*, COUNT(*) OVER() AS &NumItems.count FROM %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getPolicyStmt                   = "SELECT &Policy.* FROM %s WHERE name==$Policy.name"
	getPolicyByLookupStmt           = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID AND sliceID==$Policy.sliceID AND dataNetworkID==$Policy.dataNetworkID"
	createPolicyStmt                = "INSERT INTO %s (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, preemptionCapability, preemptionVulnerability, sessionAmbrUplink, sessionAmbrDownlink) VALUES ($Policy.id, $Policy.name, $Policy.profileID, $Policy.sliceID, $Policy.dataNetworkID, $Policy.var5qi, $Policy.arp, $Policy.preemptionCapability, $Policy.preemptionVulnerability, $Policy.sessionAmbrUplink, $Policy.sessionAmbrDownlink)"
	editPolicyStmt                  = "UPDATE %s SET profileID=$Policy.profileID, sliceID=$Policy.sliceID, dataNetworkID=$Policy.dataNetworkID, var5qi=$Policy.var5qi, arp=$Policy.arp, preemptionCapability=$Policy.preemptionCapability, preemptionVulnerability=$Policy.preemptionVulnerability, sessionAmbrUplink=$Policy.sessionAmbrUplink, sessionAmbrDownlink=$Policy.sessionAmbrDownlink WHERE name==$Policy.name"
	deletePolicyStmt                = "DELETE FROM %s WHERE name==$Policy.name"
	countPoliciesStmt               = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	countPoliciesInProfileStmt      = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE profileID=$Policy.profileID"
	countPoliciesInSliceStmt        = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE sliceID=$Policy.sliceID"
	countPoliciesInDataNetworkStmt  = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE dataNetworkID=$Policy.dataNetworkID"
	listPolicy5QIsInDataNetworkStmt = "SELECT &Policy.var5qi FROM %s WHERE dataNetworkID=$Policy.dataNetworkID"

	getPolicyByProfileAndSliceStmt = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID AND sliceID==$Policy.sliceID LIMIT 1"
	listPoliciesByProfilePagedStmt = "SELECT &Policy.*, COUNT(*) OVER() AS &NumItems.count FROM %s WHERE profileID==$Policy.profileID LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	listPoliciesByProfileAllStmt   = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID ORDER BY id ASC"

	// The default data-network binding for a profile (the default APN/DNN,
	// TS 23.401 §3.1 / TS 23.501 §5.15). At most one per profile, kept by the
	// atomic SetDefaultPolicy op (clear the profile's defaults, then set one).
	getDefaultPolicyByProfileStmt = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID AND isDefault==true LIMIT 1"
	clearDefaultPoliciesStmt      = "UPDATE %s SET isDefault=false WHERE profileID==$Policy.profileID"
	setDefaultPolicyStmt          = "UPDATE %s SET isDefault=true WHERE name==$Policy.name"
)

const policyColumnsPreV25 = "&Policy.id, &Policy.name, &Policy.profileID, &Policy.sliceID, &Policy.dataNetworkID, &Policy.var5qi, &Policy.arp, &Policy.sessionAmbrUplink, &Policy.sessionAmbrDownlink, &Policy.isDefault"

const (
	listPoliciesPagedPreV25Stmt          = "SELECT " + policyColumnsPreV25 + ", COUNT(*) OVER() AS &NumItems.count FROM %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getPolicyPreV25Stmt                  = "SELECT " + policyColumnsPreV25 + " FROM %s WHERE name==$Policy.name"
	getPolicyByLookupPreV25Stmt          = "SELECT " + policyColumnsPreV25 + " FROM %s WHERE profileID==$Policy.profileID AND sliceID==$Policy.sliceID AND dataNetworkID==$Policy.dataNetworkID"
	getPolicyByProfileAndSlicePreV25Stmt = "SELECT " + policyColumnsPreV25 + " FROM %s WHERE profileID==$Policy.profileID AND sliceID==$Policy.sliceID LIMIT 1"
	listPoliciesByProfilePagedPreV25Stmt = "SELECT " + policyColumnsPreV25 + ", COUNT(*) OVER() AS &NumItems.count FROM %s WHERE profileID==$Policy.profileID LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	listPoliciesByProfileAllPreV25Stmt   = "SELECT " + policyColumnsPreV25 + " FROM %s WHERE profileID==$Policy.profileID ORDER BY id ASC"
	getDefaultPolicyByProfilePreV25Stmt  = "SELECT " + policyColumnsPreV25 + " FROM %s WHERE profileID==$Policy.profileID AND isDefault==true LIMIT 1"
	createPolicyPreV25Stmt               = "INSERT INTO %s (id, name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink) VALUES ($Policy.id, $Policy.name, $Policy.profileID, $Policy.sliceID, $Policy.dataNetworkID, $Policy.var5qi, $Policy.arp, $Policy.sessionAmbrUplink, $Policy.sessionAmbrDownlink)"
	editPolicyPreV25Stmt                 = "UPDATE %s SET profileID=$Policy.profileID, sliceID=$Policy.sliceID, dataNetworkID=$Policy.dataNetworkID, var5qi=$Policy.var5qi, arp=$Policy.arp, sessionAmbrUplink=$Policy.sessionAmbrUplink, sessionAmbrDownlink=$Policy.sessionAmbrDownlink WHERE name==$Policy.name"
)

type Policy struct {
	ID                      string                         `db:"id"` // UUIDv7
	Name                    string                         `db:"name"`
	ProfileID               string                         `db:"profileID"`
	SliceID                 string                         `db:"sliceID"`
	DataNetworkID           string                         `db:"dataNetworkID"`
	Var5qi                  int32                          `db:"var5qi"`
	Arp                     int32                          `db:"arp"`
	PreemptionCapability    models.PreemptionCapability    `db:"preemptionCapability"`
	PreemptionVulnerability models.PreemptionVulnerability `db:"preemptionVulnerability"`
	SessionAmbrUplink       string                         `db:"sessionAmbrUplink"`
	SessionAmbrDownlink     string                         `db:"sessionAmbrDownlink"`
	// IsDefault marks this binding as the profile's default APN/DNN — used for the
	// 4G default bearer and as the 5G fallback when no DNN is requested.
	IsDefault bool `db:"isDefault"`
}

func (db *Database) policyWriteStmt(ctx context.Context, p *Policy, current, preV25 *sqlair.Statement) (*sqlair.Statement, error) {
	p.defaultPreemption()

	if db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		return current, nil
	}

	if p.PreemptionCapability != models.PreemptionCapabilityNotPreempt || p.PreemptionVulnerability != models.PreemptionVulnerabilityNotPreemptable {
		return nil, ErrMigrationPending
	}

	return preV25, nil
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
	querySummary := fmt.Sprintf("%s %s (paged)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attribute.Int("ella.db.page", page),
			attribute.Int("ella.db.page_size", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var policies []Policy

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	stmt := db.listPoliciesStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.listPoliciesPreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, args).GetAll(&policies, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			fallbackCount, countErr := db.CountPolicies(ctx)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	defaultPoliciesPreemption(policies)

	return policies, count, nil
}

func (db *Database) ListPoliciesByProfilePage(ctx context.Context, profileID string, page int, perPage int) ([]Policy, int, error) {
	querySummary := fmt.Sprintf("%s %s (paged by profile)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.ProfileID(profileID),
			attribute.Int("ella.db.page", page),
			attribute.Int("ella.db.page_size", perPage),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var policies []Policy

	var counts []NumItems

	args := ListArgs{
		Limit:  perPage,
		Offset: (page - 1) * perPage,
	}

	filter := Policy{ProfileID: profileID}

	stmt := db.listPoliciesByProfileStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.listPoliciesByProfilePreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, args, filter).GetAll(&policies, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, 0, nil
		}

		recordSpanError(span, err)

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	defaultPoliciesPreemption(policies)

	return policies, count, nil
}

func (db *Database) ListPoliciesByProfile(ctx context.Context, profileID string) ([]Policy, error) {
	querySummary := fmt.Sprintf("%s %s (all by profile)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.ProfileID(profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var policies []Policy

	filter := Policy{ProfileID: profileID}

	stmt := db.listPoliciesByProfileAllStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.listPoliciesByProfileAllPreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, filter).GetAll(&policies)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	defaultPoliciesPreemption(policies)

	return policies, nil
}

func (db *Database) GetPolicy(ctx context.Context, name string) (*Policy, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{Name: name}

	stmt := db.getPolicyStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.getPolicyPreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, row).Get(&row)
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
	querySummary := fmt.Sprintf("%s %s (lookup)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.ProfileID(profileID),
			attrs.SliceID(sliceID),
			attrs.DataNetworkID(dataNetworkID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{ProfileID: profileID, SliceID: sliceID, DataNetworkID: dataNetworkID}

	stmt := db.getPolicyByLookupStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.getPolicyByLookupPreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, row).Get(&row)
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
	querySummary := fmt.Sprintf("%s %s (default by profile)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.ProfileID(profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{ProfileID: profileID}

	stmt := db.getDefaultPolicyByProfileStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.getDefaultPolicyByProfilePreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, row).Get(&row)
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
	querySummary := fmt.Sprintf("%s %s (set default)", "UPDATE", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "update").Inc()

	_, err := opSetDefaultPolicy.Invoke(ctx, db, &Policy{ProfileID: profileID, Name: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*Policy, error) {
	querySummary := fmt.Sprintf("%s %s (by profile+slice)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.ProfileID(profileID),
			attrs.SliceID(sliceID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{ProfileID: profileID, SliceID: sliceID}

	stmt := db.getPolicyByProfileAndSliceStmt
	if !db.appliedSchemaAtLeast(ctx, policyPreemptionSchema) {
		stmt = db.getPolicyByProfileAndSlicePreV25Stmt
	}

	err := db.conn().Query(ctx, stmt, row).Get(&row)
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

	sub, err := db.GetSubscriber(ctx, imsi)
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
	querySummary := fmt.Sprintf("%s %s", "INSERT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "insert").Inc()

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
	querySummary := fmt.Sprintf("%s %s", "UPDATE", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "update").Inc()

	_, err := opUpdatePolicy.Invoke(ctx, db, policy)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) DeletePolicy(ctx context.Context, name string) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "delete").Inc()

	_, err := opDeletePolicy.Invoke(ctx, db, &stringPayload{Value: name})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// CountPolicies returns policy count
func (db *Database) CountPolicies(ctx context.Context) (int, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countPoliciesStmt).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) CountPoliciesInProfile(ctx context.Context, profileID string) (int, error) {
	querySummary := fmt.Sprintf("%s %s (by profile)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.ProfileID(profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	policy := Policy{ProfileID: profileID}

	err := db.conn().Query(ctx, db.countPoliciesInProfileStmt, policy).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) CountPoliciesInSlice(ctx context.Context, sliceID string) (int, error) {
	querySummary := fmt.Sprintf("%s %s (by slice)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.SliceID(sliceID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	policy := Policy{SliceID: sliceID}

	err := db.conn().Query(ctx, db.countPoliciesInSliceStmt, policy).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) CountPoliciesInDataNetwork(ctx context.Context, dataNetworkID string) (int, error) {
	querySummary := fmt.Sprintf("%s %s (by data network)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
			attrs.DataNetworkID(dataNetworkID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	policy := Policy{DataNetworkID: dataNetworkID}

	err := db.conn().Query(ctx, db.countPoliciesInDataNetworkStmt, policy).Get(&result)
	if err != nil {
		recordSpanError(span, err)

		return 0, fmt.Errorf("query failed: %w", err)
	}

	return result.Count, nil
}

func (db *Database) DataNetworkPolicy5QIs(ctx context.Context, dataNetworkID string) ([]int32, error) {
	querySummary := fmt.Sprintf("%s %s (5QIs in data network)", "SELECT", PoliciesTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var policies []Policy

	if err := db.conn().Query(ctx, db.listPolicy5QIsInDataNetworkStmt, Policy{DataNetworkID: dataNetworkID}).GetAll(&policies); err != nil && !errors.Is(err, sql.ErrNoRows) {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	fiveQIs := make([]int32, 0, len(policies))
	for _, p := range policies {
		fiveQIs = append(fiveQIs, p.Var5qi)
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
