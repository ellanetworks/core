// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/canonical/sqlair"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const NetworkRulesTableName = "network_rules"

const (
	getNetworkRuleStmt             = "SELECT &NetworkRule.* FROM %s WHERE id==$NetworkRule.id"
	createNetworkRuleStmt          = "INSERT INTO %s (policy_id, description, direction, remote_prefix, protocol, port_low, port_high, action, precedence, created_at, updated_at) VALUES ($NetworkRule.policy_id, $NetworkRule.description, $NetworkRule.direction, $NetworkRule.remote_prefix, $NetworkRule.protocol, $NetworkRule.port_low, $NetworkRule.port_high, $NetworkRule.action, $NetworkRule.precedence, $NetworkRule.created_at, $NetworkRule.updated_at)"
	updateNetworkRuleStmt          = "UPDATE %s SET description=$NetworkRule.description, direction=$NetworkRule.direction, remote_prefix=$NetworkRule.remote_prefix, protocol=$NetworkRule.protocol, port_low=$NetworkRule.port_low, port_high=$NetworkRule.port_high, action=$NetworkRule.action, precedence=$NetworkRule.precedence, updated_at=$NetworkRule.updated_at WHERE id==$NetworkRule.id"
	deleteNetworkRuleStmt          = "DELETE FROM %s WHERE id==$NetworkRule.id"
	deleteNetworkRulesByPolicyStmt = "DELETE FROM %s WHERE policy_id==$NetworkRule.policy_id"
	countNetworkRulesStmt          = "SELECT COUNT(*) AS &NumItems.count FROM %s"
	listRulesForPolicyStmt         = "SELECT &NetworkRule.* FROM %s WHERE policy_id==$NetworkRule.policy_id ORDER BY precedence ASC"
)

const gap = 100

type NetworkRule struct {
	ID           int64     `db:"id"`
	PolicyID     int64     `db:"policy_id"`
	Description  string    `db:"description"`
	Direction    string    `db:"direction"`
	RemotePrefix *string   `db:"remote_prefix"`
	Protocol     int32     `db:"protocol"`
	PortLow      int32     `db:"port_low"`
	PortHigh     int32     `db:"port_high"`
	Action       string    `db:"action"`
	Precedence   int32     `db:"precedence"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

// CreateNetworkRule creates a new network rule and returns its ID.
func (db *Database) CreateNetworkRule(ctx context.Context, nr *NetworkRule) (int64, error) {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "insert").Inc()

	now := time.Now().UTC()
	nr.CreatedAt = now
	nr.UpdatedAt = now

	result, err := opCreateNetworkRule.Invoke(db, nr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return 0, err
	}

	span.SetStatus(codes.Ok, "")

	return result.(int64), nil
}

// GetNetworkRule retrieves a network rule by ID.
func (db *Database) GetNetworkRule(ctx context.Context, id int64) (*NetworkRule, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "select").Inc()

	row := NetworkRule{ID: id}

	err := db.conn().Query(ctx, db.getNetworkRuleStmt, row).Get(&row)
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

// UpdateNetworkRule updates an existing network rule.
func (db *Database) UpdateNetworkRule(ctx context.Context, nr *NetworkRule) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "update").Inc()

	nr.UpdatedAt = time.Now().UTC()

	_, err := opUpdateNetworkRule.Invoke(db, nr)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// ReorderRulesForPolicy moves a rule to a new position within its policy and normalizes all precedence values.
func (db *Database) ReorderRulesForPolicy(ctx context.Context, policyID int64, movedRuleID int64, newIndex int, direction string) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "UPDATE", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "update"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "update").Inc()

	rules, err := db.ListRulesForPolicy(ctx, policyID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to list rules for policy")

		return fmt.Errorf("failed to list rules for policy: %w", err)
	}

	var filteredRules []*NetworkRule

	for _, rule := range rules {
		if rule.Direction == direction {
			filteredRules = append(filteredRules, rule)
		}
	}

	rules = filteredRules

	var (
		movedRule      *NetworkRule
		remainingRules []*NetworkRule
	)

	for _, rule := range rules {
		if rule.ID == movedRuleID {
			movedRule = rule
		} else {
			remainingRules = append(remainingRules, rule)
		}
	}

	if movedRule == nil {
		span.RecordError(ErrNotFound)
		span.SetStatus(codes.Error, "rule not found in policy")

		return ErrNotFound
	}

	var offset int32
	if movedRule.Precedence%gap == 0 {
		offset = 1
	}

	if newIndex <= 0 {
		newIndex = 0
		movedRule.Precedence = 0
	} else if newIndex >= len(remainingRules) {
		newIndex = len(remainingRules)
		movedRule.Precedence = math.MaxInt32
	} else {
		movedRule.Precedence = remainingRules[newIndex-1].Precedence + (gap / 2)
	}

	var reorderedRules []*NetworkRule

	reorderedRules = append(reorderedRules, remainingRules[:newIndex]...)
	reorderedRules = append(reorderedRules, movedRule)
	reorderedRules = append(reorderedRules, remainingRules[newIndex:]...)

	if err := db.UpdateNetworkRule(ctx, movedRule); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, fmt.Sprintf("failed to update rule precedence for rule %d", movedRule.ID))

		return fmt.Errorf("failed to update rule precedence for rule %d: %w", movedRule.ID, err)
	}

	for i, rule := range reorderedRules {
		rule.Precedence = int32((i+1)*gap) + offset
		if err := db.UpdateNetworkRule(ctx, rule); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("failed to update rule precedence for rule %d", rule.ID))

			return fmt.Errorf("failed to update rule precedence for rule %d: %w", rule.ID, err)
		}
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// DeleteNetworkRule deletes a network rule by ID.
func (db *Database) DeleteNetworkRule(ctx context.Context, id int64) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "delete").Inc()

	_, err := opDeleteNetworkRule.Invoke(db, &int64Payload{Value: id})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// CountNetworkRules returns the total count of network rules.
func (db *Database) CountNetworkRules(ctx context.Context) (int, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "select").Inc()

	var result NumItems

	err := db.conn().Query(ctx, db.countNetworkRulesStmt).Get(&result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return 0, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return result.Count, nil
}

// ListRulesForPolicy retrieves all network rules associated with a policy.
func (db *Database) ListRulesForPolicy(ctx context.Context, policyID int64) ([]*NetworkRule, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "SELECT", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "select").Inc()

	var rules []*NetworkRule

	params := NetworkRule{PolicyID: policyID}

	err := db.conn().Query(ctx, db.listRulesForPolicyStmt, params).GetAll(&rules)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Ok, "no rows")
			return []*NetworkRule{}, nil
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return nil, fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return rules, nil
}

// CreateNetworkRule creates a new network rule in the transaction and returns its ID.
func (t *Transaction) CreateNetworkRule(ctx context.Context, nr *NetworkRule) (int64, error) {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "INSERT", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "insert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "insert").Inc()

	now := time.Now().UTC()
	nr.CreatedAt = now
	nr.UpdatedAt = now

	var outcome sqlair.Outcome

	err := t.tx.Query(ctx, t.db.createNetworkRuleStmt, nr).Get(&outcome)
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

// DeleteNetworkRulesByPolicyID deletes all network rules for a given policy ID within a transaction.
func (t *Transaction) DeleteNetworkRulesByPolicyID(ctx context.Context, policyID int64) error {
	ctx, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "delete").Inc()

	var outcome sqlair.Outcome

	err := t.tx.Query(ctx, t.db.deleteNetworkRulesByPolicyStmt, NetworkRule{PolicyID: policyID}).Get(&outcome)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")

		return fmt.Errorf("query failed: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}

// DeleteNetworkRulesByPolicyID deletes all network rules for a given policy ID.
func (db *Database) DeleteNetworkRulesByPolicyID(ctx context.Context, policyID int64) error {
	_, span := tracer.Start(
		ctx,
		fmt.Sprintf("%s %s", "DELETE", NetworkRulesTableName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			attribute.String("db.collection", NetworkRulesTableName),
		),
	)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(NetworkRulesTableName, "delete"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(NetworkRulesTableName, "delete").Inc()

	_, err := opDeleteNetworkRulesByPolicy.Invoke(db, &int64Payload{Value: policyID})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return err
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
