// Copyright 2024 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const PoliciesTableName = "policies"

const (
	listPoliciesPagedStmt          = "SELECT &Policy.*, COUNT(*) OVER() AS &NumItems.count FROM %s LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	getPolicyStmt                  = "SELECT &Policy.* FROM %s WHERE name==$Policy.name"
	getPolicyByLookupStmt          = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID AND sliceID==$Policy.sliceID AND dataNetworkID==$Policy.dataNetworkID"
	createPolicyStmt               = "INSERT INTO %s (name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink) VALUES ($Policy.name, $Policy.profileID, $Policy.sliceID, $Policy.dataNetworkID, $Policy.var5qi, $Policy.arp, $Policy.sessionAmbrUplink, $Policy.sessionAmbrDownlink)"
	editPolicyStmt                 = "UPDATE %s SET profileID=$Policy.profileID, sliceID=$Policy.sliceID, dataNetworkID=$Policy.dataNetworkID, var5qi=$Policy.var5qi, arp=$Policy.arp, sessionAmbrUplink=$Policy.sessionAmbrUplink, sessionAmbrDownlink=$Policy.sessionAmbrDownlink WHERE name==$Policy.name"
	deletePolicyStmt               = "DELETE FROM %s WHERE name==$Policy.name"
	countPoliciesStmt              = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	countPoliciesInProfileStmt     = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE profileID=$Policy.profileID"
	countPoliciesInSliceStmt       = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE sliceID=$Policy.sliceID"
	countPoliciesInDataNetworkStmt = "SELECT COUNT(*) AS &NumItems.count FROM %s WHERE dataNetworkID=$Policy.dataNetworkID"

	getPolicyByProfileAndSliceStmt = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID AND sliceID==$Policy.sliceID LIMIT 1"
	listPoliciesByProfilePagedStmt = "SELECT &Policy.*, COUNT(*) OVER() AS &NumItems.count FROM %s WHERE profileID==$Policy.profileID LIMIT $ListArgs.limit OFFSET $ListArgs.offset"
	listPoliciesByProfileAllStmt   = "SELECT &Policy.* FROM %s WHERE profileID==$Policy.profileID ORDER BY id ASC"
)

type Policy struct {
	ID                  int    `db:"id"`
	Name                string `db:"name"`
	ProfileID           int    `db:"profileID"`
	SliceID             int    `db:"sliceID"`
	DataNetworkID       int    `db:"dataNetworkID"`
	Var5qi              int32  `db:"var5qi"`
	Arp                 int32  `db:"arp"`
	SessionAmbrUplink   string `db:"sessionAmbrUplink"`
	SessionAmbrDownlink string `db:"sessionAmbrDownlink"`
}

func (db *Database) ListPoliciesPage(ctx context.Context, page int, perPage int) ([]Policy, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
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

	err := db.shared.Query(ctx, db.listPoliciesStmt, args).GetAll(&policies, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			fallbackCount, countErr := db.CountPolicies(ctx)
			if countErr != nil {
				return nil, 0, nil
			}

			return nil, fallbackCount, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return policies, count, nil
}

func (db *Database) ListPoliciesByProfilePage(ctx context.Context, profileID int, page int, perPage int) ([]Policy, int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (paged by profile)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("profileID", profileID),
			attribute.Int("page", page),
			attribute.Int("per_page", perPage),
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

	err := db.shared.Query(ctx, db.listPoliciesByProfileStmt, args, filter).GetAll(&policies, &counts)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			return nil, 0, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, 0, fmt.Errorf("query failed: %w", err)
	}

	count := 0
	if len(counts) > 0 {
		count = counts[0].Count
	}

	span.SetStatus(codes.Ok, "")

	return policies, count, nil
}

func (db *Database) ListPoliciesByProfile(ctx context.Context, profileID int) ([]Policy, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (all by profile)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("profileID", profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var policies []Policy

	filter := Policy{ProfileID: profileID}

	err := db.shared.Query(ctx, db.listPoliciesByProfileAllStmt, filter).GetAll(&policies)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			return nil, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return policies, nil
}

func (db *Database) GetPolicy(ctx context.Context, name string) (*Policy, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{Name: name}

	err := db.shared.Query(ctx, db.getPolicyStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

// GetPolicyByLookup finds a policy by its profileID, sliceID, and dataNetworkID.
func (db *Database) GetPolicyByLookup(ctx context.Context, profileID, sliceID, dataNetworkID int) (*Policy, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (lookup)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("profile_id", profileID),
			attribute.Int("slice_id", sliceID),
			attribute.Int("data_network_id", dataNetworkID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{ProfileID: profileID, SliceID: sliceID, DataNetworkID: dataNetworkID}

	err := db.shared.Query(ctx, db.getPolicyByLookupStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

// GetPolicyByProfileAndSlice finds the first policy for a given profile and slice.
// Used by the AMF to resolve a default DNN when the UE does not specify one.
func (db *Database) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID int) (*Policy, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by profile+slice)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("profile_id", profileID),
			attribute.Int("slice_id", sliceID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	row := Policy{ProfileID: profileID, SliceID: sliceID}

	err := db.shared.Query(ctx, db.getPolicyByProfileAndSliceStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")

			return nil, ErrNotFound
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return &row, nil
}

// GetSessionPolicy resolves a subscriber's policy for a given slice (sst+sd) and DNN.
// It follows the chain: subscriber → profileID → policy (by profile, slice, DNN) → network rules.
func (db *Database) GetSessionPolicy(ctx context.Context, imsi string, sst int32, sd string, dnn string) (*Policy, []*NetworkRule, *DataNetwork, error) {
	ctx, span := tracer.Start(
		ctx,
		"GetSessionPolicy",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			attribute.String("imsi", imsi),
			attribute.Int("sst", int(sst)),
			attribute.String("sd", sd),
			attribute.String("dnn", dnn),
		),
	)
	defer span.End()

	sub, err := db.GetSubscriber(ctx, imsi)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "subscriber not found")

		return nil, nil, nil, fmt.Errorf("subscriber not found: %w", err)
	}

	policies, err := db.ListPoliciesByProfile(ctx, sub.ProfileID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "list policies failed")

		return nil, nil, nil, fmt.Errorf("list policies for profile %d: %w", sub.ProfileID, err)
	}

	// Batch-fetch all referenced network slices.
	sliceIDSet := make(map[int]struct{})
	for _, p := range policies {
		sliceIDSet[p.SliceID] = struct{}{}
	}

	sliceIDs := make([]int, 0, len(sliceIDSet))
	for id := range sliceIDSet {
		sliceIDs = append(sliceIDs, id)
	}

	sliceList, err := db.ListNetworkSlicesByIDs(ctx, sliceIDs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "list slices failed")

		return nil, nil, nil, fmt.Errorf("list slices by IDs: %w", err)
	}

	sliceMap := make(map[int]NetworkSlice, len(sliceList))
	for _, s := range sliceList {
		sliceMap[s.ID] = s
	}

	for _, p := range policies {
		slice, ok := sliceMap[p.SliceID]
		if !ok {
			continue
		}

		sliceSd := ""
		if slice.Sd != nil {
			sliceSd = *slice.Sd
		}

		if slice.Sst != sst || sliceSd != sd {
			continue
		}

		dataNetwork, err := db.GetDataNetworkByID(ctx, p.DataNetworkID)
		if err != nil {
			span.RecordError(err)
			return nil, nil, nil, fmt.Errorf("couldn't get data network %d: %w", p.DataNetworkID, err)
		}

		if dataNetwork.Name != dnn {
			continue
		}

		rules, err := db.ListRulesForPolicy(ctx, int64(p.ID))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "list rules failed")

			return nil, nil, nil, fmt.Errorf("list rules for policy %d: %w", p.ID, err)
		}

		span.SetStatus(codes.Ok, "")

		return &p, rules, dataNetwork, nil
	}

	span.SetStatus(codes.Error, "no matching policy")

	return nil, nil, nil, fmt.Errorf("no policy matching sst=%d sd=%q dnn=%q for profile %d", sst, sd, dnn, sub.ProfileID)
}

func (db *Database) CreatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "insert").Inc()

	err := db.shared.Query(ctx, db.createPolicyStmt, policy).Run()
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) UpdatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "update").Inc()

	var outcome sqlair.Outcome

	err := db.shared.Query(ctx, db.editPolicyStmt, policy).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (t *Transaction) CreatePolicy(ctx context.Context, policy *Policy) (int64, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "insert").Inc()

	var outcome sqlair.Outcome

	err := t.tx.Query(ctx, t.db.createPolicyStmt, policy).Get(&outcome)
	if err != nil {
		if isUniqueNameError(err) {
			span.RecordError(ErrAlreadyExists)
			span.SetStatus(codes.Error, "unique constraint failed")

			return 0, ErrAlreadyExists
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	id, err := outcome.Result().LastInsertId()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving insert ID failed")

		return 0, fmt.Errorf("retrieving insert ID failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return id, nil
}

func (t *Transaction) UpdatePolicy(ctx context.Context, policy *Policy) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "update").Inc()

	var outcome sqlair.Outcome

	err := t.tx.Query(ctx, t.db.editPolicyStmt, policy).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

func (db *Database) DeletePolicy(ctx context.Context, name string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := db.shared.Query(ctx, db.deletePolicyStmt, Policy{Name: name}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "retrieving rows affected failed")

		return fmt.Errorf("retrieving rows affected failed: %w", err)
	}

	if rowsAffected == 0 {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "not found")

		return ErrNotFound
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// CountPolicies returns policy count
func (db *Database) CountPolicies(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	err := db.shared.Query(ctx, db.countPoliciesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) CountPoliciesInProfile(ctx context.Context, profileID int) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by profile)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("profile_id", profileID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	policy := Policy{ProfileID: profileID}

	err := db.shared.Query(ctx, db.countPoliciesInProfileStmt, policy).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) CountPoliciesInSlice(ctx context.Context, sliceID int) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by slice)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("slice_id", sliceID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	policy := Policy{SliceID: sliceID}

	err := db.shared.Query(ctx, db.countPoliciesInSliceStmt, policy).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) CountPoliciesInDataNetwork(ctx context.Context, dataNetworkID int) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s (by data network)", "SELECT", PoliciesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", PoliciesTableName),
			attribute.Int("data_network_id", dataNetworkID),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(PoliciesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(PoliciesTableName, "select").Inc()

	var result NumItems

	policy := Policy{DataNetworkID: dataNetworkID}

	err := db.shared.Query(ctx, db.countPoliciesInDataNetworkStmt, policy).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

func (db *Database) PoliciesInDataNetwork(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"PoliciesInDataNetwork",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
		),
	)
	defer span.End()

	dataNetwork, err := db.GetDataNetwork(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "data network not found")

		return false, fmt.Errorf("data network not found: %w", err)
	}

	count, err := db.CountPoliciesInDataNetwork(ctx, dataNetwork.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "counting failed")

		return false, fmt.Errorf("counting failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return count > 0, nil
}

func (db *Database) PoliciesInSlice(ctx context.Context, name string) (bool, error) {
	ctx, span := tracer.Start(
		ctx,
		"PoliciesInSlice",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
		),
	)
	defer span.End()

	slice, err := db.GetNetworkSlice(ctx, name)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "slice not found")

		return false, fmt.Errorf("slice not found: %w", err)
	}

	count, err := db.CountPoliciesInSlice(ctx, slice.ID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "counting failed")

		return false, fmt.Errorf("counting failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return count > 0, nil
}
