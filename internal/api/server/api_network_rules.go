// Copyright 2026 Ella Networks

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type NetworkRule struct {
	ID           int64   `json:"id,omitempty"`
	PolicyID     int64   `json:"policy_id,omitempty"`
	Description  string  `json:"description"`
	Direction    string  `json:"direction"`
	RemotePrefix *string `json:"remote_prefix"`
	Protocol     int32   `json:"protocol"`
	PortLow      int32   `json:"port_low"`
	PortHigh     int32   `json:"port_high"`
	Action       string  `json:"action"`
	Precedence   int32   `json:"precedence"`
	CreatedAt    string  `json:"created_at,omitempty"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
}

type ListNetworkRulesResponse struct {
	Items      []NetworkRule `json:"items"`
	Page       int           `json:"page,omitempty"`
	PerPage    int           `json:"per_page,omitempty"`
	TotalCount int           `json:"total_count,omitempty"`
}

const MaxNumNetworkRulesPerDirection = 12

func validateRemotePrefix(prefix *string) error {
	if prefix == nil || *prefix == "" {
		return nil
	}

	_, _, err := net.ParseCIDR(*prefix)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %w", err)
	}

	return nil
}

func validatePorts(portLow, portHigh int32) error {
	if portLow < 0 || portHigh < 0 {
		return errors.New("port values must be >= 0")
	}

	if portLow > portHigh {
		return errors.New("port_low must be <= port_high")
	}

	if portHigh > 65535 {
		return errors.New("port values must be <= 65535")
	}

	return nil
}

func validateProtocol(protocol int32) error {
	if protocol < 0 || protocol > 255 {
		return errors.New("protocol must be between 0 and 255")
	}

	return nil
}

func validateAction(action string) error {
	if action != "allow" && action != "deny" {
		return errors.New("action must be 'allow' or 'deny'")
	}

	return nil
}

func validateDirection(direction string) error {
	if direction != "uplink" && direction != "downlink" && direction != "both" {
		return errors.New("direction must be 'uplink', 'downlink', or 'both'")
	}

	return nil
}

func validatePrecedence(precedence int32) error {
	if precedence <= 0 {
		return errors.New("precedence must be > 0")
	}

	return nil
}

func validateNetworkRule(rule *NetworkRule) error {
	if err := validateDirection(rule.Direction); err != nil {
		return err
	}

	if err := validateRemotePrefix(rule.RemotePrefix); err != nil {
		return err
	}

	if err := validateProtocol(rule.Protocol); err != nil {
		return err
	}

	if err := validatePorts(rule.PortLow, rule.PortHigh); err != nil {
		return err
	}

	if err := validateAction(rule.Action); err != nil {
		return err
	}

	if err := validatePrecedence(rule.Precedence); err != nil {
		return err
	}

	return nil
}

func dbNetworkRuleToAPI(nr *db.NetworkRule) NetworkRule {
	return NetworkRule{
		ID:           nr.ID,
		PolicyID:     nr.PolicyID,
		Description:  nr.Description,
		Direction:    nr.Direction,
		RemotePrefix: nr.RemotePrefix,
		Protocol:     nr.Protocol,
		PortLow:      nr.PortLow,
		PortHigh:     nr.PortHigh,
		Action:       nr.Action,
		Precedence:   nr.Precedence,
		CreatedAt:    nr.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    nr.UpdatedAt.Format(time.RFC3339),
	}
}

func CreateNetworkRuleForPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		policyName := r.PathValue("name")
		if policyName == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)

			return
		}

		policy, err := dbInstance.GetPolicy(ctx, policyName)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Policy not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)

			return
		}

		var ruleAPI NetworkRule
		if err := json.NewDecoder(r.Body).Decode(&ruleAPI); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)

			return
		}

		if err := validateNetworkRule(&ruleAPI); err != nil {
			writeError(ctx, w, http.StatusBadRequest, fmt.Sprintf("Validation error: %s", err.Error()), nil, logger.APILog)

			return
		}

		// Enforce maximum number of network rules per policy per direction
		rules, err := dbInstance.ListRulesForPolicy(ctx, int64(policy.ID))
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list network rules", err, logger.APILog)

			return
		}

		var count int

		for _, r := range rules {
			if r.Direction == ruleAPI.Direction {
				count++
			}
		}

		if count >= MaxNumNetworkRulesPerDirection {
			writeError(ctx, w, http.StatusBadRequest, "Maximum number of network rules reached ("+strconv.Itoa(MaxNumNetworkRulesPerDirection)+")", nil, logger.APILog)

			return
		}

		rule := &db.NetworkRule{
			PolicyID:     int64(policy.ID),
			Description:  ruleAPI.Description,
			Direction:    ruleAPI.Direction,
			RemotePrefix: ruleAPI.RemotePrefix,
			Protocol:     ruleAPI.Protocol,
			PortLow:      ruleAPI.PortLow,
			PortHigh:     ruleAPI.PortHigh,
			Action:       ruleAPI.Action,
			Precedence:   ruleAPI.Precedence,
		}

		id, err := dbInstance.CreateNetworkRule(ctx, rule)
		if err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(ctx, w, http.StatusConflict, "Rule with this name already exists for the policy", err, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to create network rule", err, logger.APILog)

			return
		}

		writeResponse(ctx, w, CreateSuccessResponse{Message: "Network rule created", ID: id}, http.StatusCreated, logger.APILog)
	})
}

func ListNetworkRulesForPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		policyName := r.PathValue("name")
		if policyName == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)

			return
		}

		policy, err := dbInstance.GetPolicy(ctx, policyName)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Policy not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)

			return
		}

		rules, err := dbInstance.ListRulesForPolicy(ctx, int64(policy.ID))
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list network rules", err, logger.APILog)

			return
		}

		items := make([]NetworkRule, 0)
		for _, rule := range rules {
			items = append(items, dbNetworkRuleToAPI(rule))
		}

		resp := ListNetworkRulesResponse{
			Items: items,
		}

		writeResponse(ctx, w, resp, http.StatusOK, logger.APILog)
	})
}

func GetNetworkRule(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		idStr := r.PathValue("id")
		if idStr == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing id parameter", nil, logger.APILog)

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid id", err, logger.APILog)

			return
		}

		rule, err := dbInstance.GetNetworkRule(ctx, id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Network rule not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve network rule", err, logger.APILog)

			return
		}

		writeResponse(ctx, w, dbNetworkRuleToAPI(rule), http.StatusOK, logger.APILog)
	})
}

func UpdateNetworkRule(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		idStr := r.PathValue("id")
		if idStr == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing id parameter", nil, logger.APILog)

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid id", err, logger.APILog)

			return
		}

		var updateAPI NetworkRule
		if err := json.NewDecoder(r.Body).Decode(&updateAPI); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)

			return
		}

		if err := validateNetworkRule(&updateAPI); err != nil {
			writeError(ctx, w, http.StatusBadRequest, fmt.Sprintf("Validation error: %s", err.Error()), nil, logger.APILog)

			return
		}

		existing, err := dbInstance.GetNetworkRule(ctx, id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Network rule not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve network rule", err, logger.APILog)

			return
		}

		if updateAPI.PolicyID != 0 && updateAPI.PolicyID != existing.PolicyID {
			writeError(ctx, w, http.StatusBadRequest, "Cannot change policy_id of an existing rule", nil, logger.APILog)

			return
		}

		rule := &db.NetworkRule{
			ID:           existing.ID,
			PolicyID:     existing.PolicyID,
			Description:  existing.Description,
			Direction:    updateAPI.Direction,
			RemotePrefix: updateAPI.RemotePrefix,
			Protocol:     updateAPI.Protocol,
			PortLow:      updateAPI.PortLow,
			PortHigh:     updateAPI.PortHigh,
			Action:       updateAPI.Action,
			Precedence:   updateAPI.Precedence,
		}

		if err := dbInstance.UpdateNetworkRule(ctx, rule); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Network rule not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to update network rule", err, logger.APILog)

			return
		}

		writeResponse(ctx, w, SuccessResponse{Message: "Network rule updated"}, http.StatusOK, logger.APILog)
	})
}

func DeleteNetworkRule(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		idStr := r.PathValue("id")
		if idStr == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing id parameter", nil, logger.APILog)

			return
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid id", err, logger.APILog)

			return
		}

		if err := dbInstance.DeleteNetworkRule(ctx, id); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Network rule not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to delete network rule", err, logger.APILog)

			return
		}

		writeResponse(ctx, w, SuccessResponse{Message: "Network rule deleted"}, http.StatusOK, logger.APILog)
	})
}

func ReorderNetworkRule(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		policyName := r.PathValue("name")
		if policyName == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)

			return
		}

		policy, err := dbInstance.GetPolicy(ctx, policyName)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Policy not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)

			return
		}

		idStr := r.PathValue("id")
		if idStr == "" {
			writeError(ctx, w, http.StatusBadRequest, "Missing id parameter", nil, logger.APILog)

			return
		}

		ruleID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid id", err, logger.APILog)

			return
		}

		var reqBody struct {
			NewIndex int `json:"new_index"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)

			return
		}

		if reqBody.NewIndex < 0 {
			writeError(ctx, w, http.StatusBadRequest, "new_index must be >= 0", nil, logger.APILog)

			return
		}

		rule, err := dbInstance.GetNetworkRule(ctx, ruleID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(ctx, w, http.StatusNotFound, "Network rule not found", nil, logger.APILog)

				return
			}

			writeError(ctx, w, http.StatusInternalServerError, "Failed to retrieve network rule", err, logger.APILog)

			return
		}

		if rule.PolicyID != int64(policy.ID) {
			writeError(ctx, w, http.StatusBadRequest, "Rule does not belong to the specified policy", nil, logger.APILog)

			return
		}

		if err := dbInstance.ReorderRulesForPolicy(ctx, int64(policy.ID), ruleID, reqBody.NewIndex, rule.Direction); err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to reorder network rule", err, logger.APILog)

			return
		}

		writeResponse(ctx, w, SuccessResponse{Message: "Rule reordered successfully"}, http.StatusOK, logger.APILog)
	})
}
