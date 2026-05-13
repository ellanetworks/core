package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PolicyRuleInput struct {
	Description  string  `json:"description" db:"description"`
	RemotePrefix *string `json:"remote_prefix" db:"remote_prefix"`
	Protocol     int32   `json:"protocol" db:"protocol"`
	PortLow      int32   `json:"port_low" db:"port_low"`
	PortHigh     int32   `json:"port_high" db:"port_high"`
	Action       string  `json:"action" db:"action"`
}

type PolicyRulesInput struct {
	Uplink   []PolicyRuleInput `json:"uplink,omitempty"`
	Downlink []PolicyRuleInput `json:"downlink,omitempty"`
}

type policyWithRulesPayload struct {
	Policy Policy            `json:"policy"`
	Rules  *PolicyRulesInput `json:"rules,omitempty"`
}

func (db *Database) CreatePolicyWithRules(ctx context.Context, policy *Policy, rules *PolicyRulesInput) error {
	if policy.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("generate policy id: %w", err)
		}

		policy.ID = id.String()
	}

	_, err := opCreatePolicyWithRules.Invoke(db, &policyWithRulesPayload{Policy: *policy, Rules: rules})

	return err
}

func (db *Database) UpdatePolicyWithRules(ctx context.Context, policy *Policy, rules *PolicyRulesInput) error {
	_, err := opUpdatePolicyWithRules.Invoke(db, &policyWithRulesPayload{Policy: *policy, Rules: rules})

	return err
}

func (db *Database) applyCreatePolicyWithRules(ctx context.Context, payload *policyWithRulesPayload) (any, error) {
	policy := payload.Policy
	if policy.ID == "" {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate policy id: %w", err)
		}

		policy.ID = id.String()
	}

	if err := db.runner(ctx).Query(ctx, db.createPolicyStmt, &policy).Run(); err != nil {
		if isUniqueNameError(err) {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("query failed: %w", err)
	}

	if err := db.applyPolicyRulesPayload(ctx, policy.ID, payload.Rules); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) applyUpdatePolicyWithRules(ctx context.Context, payload *policyWithRulesPayload) (any, error) {
	if _, err := db.applyUpdatePolicy(ctx, &payload.Policy); err != nil {
		return nil, err
	}

	if err := db.applyPolicyRulesPayload(ctx, payload.Policy.ID, payload.Rules); err != nil {
		return nil, err
	}

	return nil, nil
}

func (db *Database) applyPolicyRulesPayload(ctx context.Context, policyID string, rules *PolicyRulesInput) error {
	if err := db.runner(ctx).Query(ctx, db.deleteNetworkRulesByPolicyStmt, NetworkRule{PolicyID: policyID}).Run(); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if rules == nil {
		return nil
	}

	now := time.Now().UTC()
	if err := db.insertPolicyRules(ctx, policyID, "uplink", rules.Uplink, now); err != nil {
		return err
	}

	if err := db.insertPolicyRules(ctx, policyID, "downlink", rules.Downlink, now); err != nil {
		return err
	}

	return nil
}

func (db *Database) insertPolicyRules(ctx context.Context, policyID, direction string, rules []PolicyRuleInput, now time.Time) error {
	for i, rule := range rules {
		nr := &NetworkRule{
			PolicyID:     policyID,
			Description:  rule.Description,
			Direction:    direction,
			RemotePrefix: rule.RemotePrefix,
			Protocol:     rule.Protocol,
			PortLow:      rule.PortLow,
			PortHigh:     rule.PortHigh,
			Action:       rule.Action,
			Precedence:   int32(i + 1),
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if nr.ID == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("generate network rule id: %w", err)
			}

			nr.ID = id.String()
		}

		if err := db.runner(ctx).Query(ctx, db.createNetworkRuleStmt, nr).Run(); err != nil {
			if isUniqueNameError(err) {
				return ErrAlreadyExists
			}

			return fmt.Errorf("query failed: %w", err)
		}
	}

	return nil
}
