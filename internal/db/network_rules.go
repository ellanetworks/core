// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
)

const NetworkRulesTableName = "network_rules"

const (
	getNetworkRuleSQL             = "SELECT id, policy_id, description, direction, remote_prefix, COALESCE(protocol, 255), COALESCE(port_low, 0), COALESCE(port_high, 0), action, precedence, created_at, updated_at FROM " + NetworkRulesTableName + " WHERE id==?"
	createNetworkRuleSQL          = "INSERT INTO " + NetworkRulesTableName + " (id, policy_id, description, direction, remote_prefix, protocol, port_low, port_high, action, precedence, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	updateNetworkRuleSQL          = "UPDATE " + NetworkRulesTableName + " SET description=?, direction=?, remote_prefix=?, protocol=?, port_low=?, port_high=?, action=?, precedence=?, updated_at=? WHERE id==?"
	deleteNetworkRuleSQL          = "DELETE FROM " + NetworkRulesTableName + " WHERE id==?"
	deleteNetworkRulesByPolicySQL = "DELETE FROM " + NetworkRulesTableName + " WHERE policy_id==?"
	listRulesForPolicySQL         = "SELECT id, policy_id, description, direction, remote_prefix, COALESCE(protocol, 255), COALESCE(port_low, 0), COALESCE(port_high, 0), action, precedence, created_at, updated_at FROM " + NetworkRulesTableName + " WHERE policy_id==? ORDER BY precedence ASC"
)

const gap = 100

type NetworkRule struct {
	ID           string // UUIDv7
	PolicyID     string // FK to policies.id (UUID)
	Description  string
	Direction    string
	RemotePrefix *string
	Protocol     int32
	PortLow      int32
	PortHigh     int32
	Action       string
	Precedence   int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateNetworkRule creates a new network rule and returns its ID.
func (db *Database) CreateNetworkRule(ctx context.Context, nr *NetworkRule) (_ string, err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "INSERT")
	defer func() { finish(err) }()

	now := time.Now().UTC()
	nr.CreatedAt = now
	nr.UpdatedAt = now

	if nr.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generate network rule id: %w", err)
		}

		nr.ID = id.String()
	}

	_, err = opCreateNetworkRule.Invoke(ctx, db, nr)
	if err != nil {
		return "", err
	}

	return nr.ID, nil
}

// GetNetworkRule retrieves a network rule by ID.
func (db *Database) GetNetworkRule(ctx context.Context, id string) (_ *NetworkRule, err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "SELECT")
	defer func() { finish(err) }()

	row, err := scanNetworkRule(db.reader().QueryRowContext(ctx, getNetworkRuleSQL, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return row, nil
}

// UpdateNetworkRule updates an existing network rule.
func (db *Database) UpdateNetworkRule(ctx context.Context, nr *NetworkRule) (err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "UPDATE")
	defer func() { finish(err) }()

	nr.UpdatedAt = time.Now().UTC()

	_, err = opUpdateNetworkRule.Invoke(ctx, db, nr)
	if err != nil {
		return err
	}

	return nil
}

// ReorderRulesForPolicy moves a rule to a new position within its policy and normalizes all precedence values.
func (db *Database) ReorderRulesForPolicy(ctx context.Context, policyID string, movedRuleID string, newIndex int, direction string) (err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "UPDATE")
	defer func() { finish(err) }()

	rules, err := db.ListRulesForPolicy(ctx, policyID)
	if err != nil {
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
		return fmt.Errorf("failed to update rule precedence for rule %s: %w", movedRule.ID, err)
	}

	for i, rule := range reorderedRules {
		rule.Precedence = int32((i+1)*gap) + offset
		if err := db.UpdateNetworkRule(ctx, rule); err != nil {
			return fmt.Errorf("failed to update rule precedence for rule %s: %w", rule.ID, err)
		}
	}

	return nil
}

// DeleteNetworkRule deletes a network rule by ID.
func (db *Database) DeleteNetworkRule(ctx context.Context, id string) (err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteNetworkRule.Invoke(ctx, db, &stringPayload{Value: id})
	if err != nil {
		return err
	}

	return nil
}

// ListRulesForPolicy retrieves all network rules associated with a policy.
func (db *Database) ListRulesForPolicy(ctx context.Context, policyID string) (_ []*NetworkRule, err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "SELECT")
	defer func() { finish(err) }()

	rules, err := queryAll(ctx, db.reader(), listRulesForPolicySQL, scanNetworkRule, policyID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if rules == nil {
		return []*NetworkRule{}, nil
	}

	return rules, nil
}

// DeleteNetworkRulesByPolicyID deletes all network rules for a given policy ID.
func (db *Database) DeleteNetworkRulesByPolicyID(ctx context.Context, policyID string) (err error) {
	ctx, finish := startDBQuery(ctx, NetworkRulesTableName, "DELETE")
	defer func() { finish(err) }()

	_, err = opDeleteNetworkRulesByPolicy.Invoke(ctx, db, &stringPayload{Value: policyID})
	if err != nil {
		return err
	}

	return nil
}

func scanNetworkRule(row scanner) (*NetworkRule, error) {
	var v NetworkRule

	if err := row.Scan(&v.ID, &v.PolicyID, &v.Description, &v.Direction, &v.RemotePrefix, &v.Protocol, &v.PortLow, &v.PortHigh, &v.Action, &v.Precedence, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}

	return &v, nil
}
