package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const MaxNumNetworkRulesPerDirection = 12

type PolicyRule struct {
	Description  string  `json:"description"`
	RemotePrefix *string `json:"remote_prefix"`
	Protocol     int32   `json:"protocol"`
	PortLow      int32   `json:"port_low"`
	PortHigh     int32   `json:"port_high"`
	Action       string  `json:"action"`
}

type PolicyRules struct {
	Uplink   []PolicyRule `json:"uplink,omitempty"`
	Downlink []PolicyRule `json:"downlink,omitempty"`
}

type CreatePolicyParams struct {
	Name            string       `json:"name"`
	BitrateUplink   string       `json:"bitrate_uplink,omitempty"`
	BitrateDownlink string       `json:"bitrate_downlink,omitempty"`
	Var5qi          int32        `json:"var5qi,omitempty"`
	Arp             int32        `json:"arp,omitempty"`
	DataNetworkName string       `json:"data_network_name,omitempty"`
	Rules           *PolicyRules `json:"rules,omitempty"`
}

type UpdatePolicyParams struct {
	BitrateUplink   string       `json:"bitrate_uplink,omitempty"`
	BitrateDownlink string       `json:"bitrate_downlink,omitempty"`
	Var5qi          int32        `json:"var5qi,omitempty"`
	Arp             int32        `json:"arp,omitempty"`
	DataNetworkName string       `json:"data_network_name,omitempty"`
	Rules           *PolicyRules `json:"rules,omitempty"`
}

type Policy struct {
	Name            string       `json:"name"`
	BitrateUplink   string       `json:"bitrate_uplink,omitempty"`
	BitrateDownlink string       `json:"bitrate_downlink,omitempty"`
	Var5qi          int32        `json:"var5qi,omitempty"`
	Arp             int32        `json:"arp,omitempty"`
	DataNetworkName string       `json:"data_network_name,omitempty"`
	Rules           *PolicyRules `json:"rules,omitempty"`
}

type ListPoliciesResponse struct {
	Items      []Policy `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

const (
	CreatePolicyAction = "create_policy"
	UpdatePolicyAction = "update_policy"
	DeletePolicyAction = "delete_policy"
)

const (
	MaxNumPolicies = 12
)

func isPolicyNameValid(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func isValidBitrate(bitrate string) bool {
	s := strings.Split(bitrate, " ")
	if len(s) != 2 {
		return false
	}

	value := s[0]

	unit := s[1]
	if unit != "Mbps" && unit != "Gbps" {
		return false
	}

	valueInt, err := strconv.Atoi(value)
	if err != nil {
		return false
	}

	return valueInt > 0 && valueInt <= 1000000
}

var valid5Qi = []int32{5, 6, 7, 8, 9, 69, 70, 79, 80} // only non-gbr 5Qi are supported for now

func isValid5Qi(var5qi int32) bool {
	return slices.Contains(valid5Qi, var5qi)
}

func isValidArp(arp int32) bool {
	return arp >= 1 && arp <= 15
}

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

func validatePolicyRule(rule PolicyRule) error {
	if rule.Description == "" {
		return errors.New("rule description is missing")
	}

	if err := validateAction(rule.Action); err != nil {
		return errors.New("rule action must be 'allow' or 'deny'")
	}

	if err := validateRemotePrefix(rule.RemotePrefix); err != nil {
		return fmt.Errorf("invalid rule remote_prefix: %w", err)
	}

	if err := validateProtocol(rule.Protocol); err != nil {
		return fmt.Errorf("invalid rule protocol: %w", err)
	}

	if err := validatePorts(rule.PortLow, rule.PortHigh); err != nil {
		return fmt.Errorf("invalid rule ports: %w", err)
	}

	return nil
}

func validatePolicyRules(rules *PolicyRules) error {
	if rules == nil {
		return nil
	}

	if len(rules.Uplink) > MaxNumNetworkRulesPerDirection {
		return fmt.Errorf("uplink rules exceed maximum of %d", MaxNumNetworkRulesPerDirection)
	}

	if len(rules.Downlink) > MaxNumNetworkRulesPerDirection {
		return fmt.Errorf("downlink rules exceed maximum of %d", MaxNumNetworkRulesPerDirection)
	}

	for i, rule := range rules.Uplink {
		if err := validatePolicyRule(rule); err != nil {
			return fmt.Errorf("uplink rule %d: %w", i, err)
		}
	}

	for i, rule := range rules.Downlink {
		if err := validatePolicyRule(rule); err != nil {
			return fmt.Errorf("downlink rule %d: %w", i, err)
		}
	}

	return nil
}

func createNetworkRulesForPolicy(ctx context.Context, dbInstance *db.Database, policy *db.Policy, rules *PolicyRules) error {
	if rules == nil {
		return nil
	}

	// Create uplink rules with precedence
	for i, rule := range rules.Uplink {
		dbRule := &db.NetworkRule{
			PolicyID:     int64(policy.ID),
			Description:  rule.Description,
			Direction:    "uplink",
			RemotePrefix: rule.RemotePrefix,
			Protocol:     rule.Protocol,
			PortLow:      rule.PortLow,
			PortHigh:     rule.PortHigh,
			Action:       rule.Action,
			Precedence:   int32(i + 1), // 1-indexed
		}

		_, err := dbInstance.CreateNetworkRule(ctx, dbRule)
		if err != nil {
			return fmt.Errorf("failed to create uplink rule %d: %w", i, err)
		}
	}

	// Create downlink rules with precedence
	for i, rule := range rules.Downlink {
		dbRule := &db.NetworkRule{
			PolicyID:     int64(policy.ID),
			Description:  rule.Description,
			Direction:    "downlink",
			RemotePrefix: rule.RemotePrefix,
			Protocol:     rule.Protocol,
			PortLow:      rule.PortLow,
			PortHigh:     rule.PortHigh,
			Action:       rule.Action,
			Precedence:   int32(i + 1), // 1-indexed
		}

		_, err := dbInstance.CreateNetworkRule(ctx, dbRule)
		if err != nil {
			return fmt.Errorf("failed to create downlink rule %d: %w", i, err)
		}
	}

	return nil
}

func ListPolicies(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		dbPolicies, total, err := dbInstance.ListPoliciesPage(ctx, page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Policies not found", err, logger.APILog)
			return
		}

		policyList := make([]Policy, 0)

		for _, dbPolicy := range dbPolicies {
			dataNetwork, err := dbInstance.GetDataNetworkByID(ctx, dbPolicy.DataNetworkID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}

			policyList = append(policyList, Policy{
				Name:            dbPolicy.Name,
				BitrateDownlink: dbPolicy.BitrateDownlink,
				BitrateUplink:   dbPolicy.BitrateUplink,
				Var5qi:          dbPolicy.Var5qi,
				Arp:             dbPolicy.Arp,
				DataNetworkName: dataNetwork.Name,
			})
		}

		resp := ListPoliciesResponse{
			Items:      policyList,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func getPolicyRulesForPolicy(ctx context.Context, dbInstance *db.Database, policyID int64) (*PolicyRules, error) {
	rules, err := dbInstance.ListRulesForPolicy(ctx, policyID)
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		return nil, nil
	}

	policyRules := &PolicyRules{
		Uplink:   []PolicyRule{},
		Downlink: []PolicyRule{},
	}

	for _, rule := range rules {
		apiRule := PolicyRule{
			Description:  rule.Description,
			RemotePrefix: rule.RemotePrefix,
			Protocol:     rule.Protocol,
			PortLow:      rule.PortLow,
			PortHigh:     rule.PortHigh,
			Action:       rule.Action,
		}

		switch rule.Direction {
		case "uplink":
			policyRules.Uplink = append(policyRules.Uplink, apiRule)
		case "downlink":
			policyRules.Downlink = append(policyRules.Downlink, apiRule)
		}
	}

	// Return nil if no rules in either direction
	if len(policyRules.Uplink) == 0 && len(policyRules.Downlink) == 0 {
		return nil, nil
	}

	return policyRules, nil
}

func GetPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		dbPolicy, err := dbInstance.GetPolicy(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetworkByID(r.Context(), dbPolicy.DataNetworkID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		rules, err := getPolicyRulesForPolicy(r.Context(), dbInstance, int64(dbPolicy.ID))
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy rules", err, logger.APILog)
			return
		}

		policy := Policy{
			Name:            dbPolicy.Name,
			BitrateDownlink: dbPolicy.BitrateDownlink,
			BitrateUplink:   dbPolicy.BitrateUplink,
			Var5qi:          dbPolicy.Var5qi,
			Arp:             dbPolicy.Arp,
			DataNetworkName: dataNetwork.Name,
			Rules:           rules,
		}
		writeResponse(r.Context(), w, policy, http.StatusOK, logger.APILog)
	})
}

func DeletePolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		_, err := dbInstance.GetPolicy(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		subsInPolicy, err := dbInstance.SubscribersInPolicy(r.Context(), name)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check subscribers", err, logger.APILog)

			return
		}

		if subsInPolicy {
			writeError(r.Context(), w, http.StatusConflict, "Policy has subscribers", nil, logger.APILog)
			return
		}

		if err := dbInstance.DeletePolicy(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete policy", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Policy deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeletePolicyAction, email, getClientIP(r), "User deleted policy: "+name)
	})
}

func CreatePolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var createPolicyParams CreatePolicyParams
		if err := json.NewDecoder(r.Body).Decode(&createPolicyParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validatePolicyParams(createPolicyParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		numPolicies, err := dbInstance.CountPolicies(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count policies", err, logger.APILog)
			return
		}

		if numPolicies >= MaxNumPolicies {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of policies reached ("+strconv.Itoa(MaxNumPolicies)+")", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), createPolicyParams.DataNetworkName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		dbPolicy := &db.Policy{
			Name:            createPolicyParams.Name,
			BitrateDownlink: createPolicyParams.BitrateDownlink,
			BitrateUplink:   createPolicyParams.BitrateUplink,
			Var5qi:          createPolicyParams.Var5qi,
			Arp:             createPolicyParams.Arp,
			DataNetworkID:   dataNetwork.ID,
		}

		if err := dbInstance.CreatePolicy(r.Context(), dbPolicy); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "Policy already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create policy", err, logger.APILog)

			return
		}

		// Get the created policy to get its ID
		createdPolicy, err := dbInstance.GetPolicy(r.Context(), createPolicyParams.Name)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve created policy", err, logger.APILog)
			return
		}

		// Create network rules if provided
		if createPolicyParams.Rules != nil {
			if err := createNetworkRulesForPolicy(r.Context(), dbInstance, createdPolicy, createPolicyParams.Rules); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create policy rules", err, logger.APILog)
				return
			}
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Policy created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreatePolicyAction, email, getClientIP(r), "User created policy: "+createPolicyParams.Name)
	})
}

func UpdatePolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		policyName := r.PathValue("name")
		if policyName == "" || strings.ContainsRune(policyName, '/') {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid or missing name parameter", nil, logger.APILog)
			return
		}

		var updatePolicyParams UpdatePolicyParams

		if err := json.NewDecoder(r.Body).Decode(&updatePolicyParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateUpdatePolicyParams(updatePolicyParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		policy, err := dbInstance.GetPolicy(r.Context(), policyName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), updatePolicyParams.DataNetworkName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		policy.Name = policyName
		policy.BitrateDownlink = updatePolicyParams.BitrateDownlink
		policy.BitrateUplink = updatePolicyParams.BitrateUplink
		policy.Var5qi = updatePolicyParams.Var5qi
		policy.Arp = updatePolicyParams.Arp
		policy.DataNetworkID = dataNetwork.ID

		if err := dbInstance.UpdatePolicy(r.Context(), policy); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update policy", err, logger.APILog)
			return
		}

		// Update network rules if provided
		if updatePolicyParams.Rules != nil {
			// Delete existing rules for this policy
			if err := dbInstance.DeleteNetworkRulesByPolicyID(r.Context(), int64(policy.ID)); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete existing policy rules", err, logger.APILog)
				return
			}

			// Create new rules
			if err := createNetworkRulesForPolicy(r.Context(), dbInstance, policy, updatePolicyParams.Rules); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create policy rules", err, logger.APILog)
				return
			}
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(r.Context(), UpdatePolicyAction, email, getClientIP(r), "User updated policy: "+policyName)
	})
}

func validatePolicyParams(p CreatePolicyParams) error {
	switch {
	case p.Name == "":
		return errors.New("name is missing")
	case p.DataNetworkName == "":
		return errors.New("data_network_name is missing")
	case p.BitrateUplink == "":
		return errors.New("bitrate_uplink is missing")
	case p.BitrateDownlink == "":
		return errors.New("bitrate_downlink is missing")
	case p.Var5qi == 0:
		return errors.New("Var5qi is missing")
	case p.Arp == 0:
		return errors.New("arp is missing")
	case !isPolicyNameValid(p.Name):
		return errors.New("invalid name format - must be less than 256 characters")
	case !isValidBitrate(p.BitrateUplink):
		return errors.New("invalid bitrate_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValidBitrate(p.BitrateDownlink):
		return errors.New("invalid bitrate_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("invalid Var5qi format - must be an integer associated with a non-GBR 5QI")
	case !isValidArp(p.Arp):
		return errors.New("invalid arp format - must be an integer between 1 and 255")
	}

	if err := validatePolicyRules(p.Rules); err != nil {
		return err
	}

	return nil
}

func validateUpdatePolicyParams(p UpdatePolicyParams) error {
	switch {
	case p.DataNetworkName == "":
		return errors.New("data_network_name is missing")
	case p.BitrateUplink == "":
		return errors.New("bitrate_uplink is missing")
	case p.BitrateDownlink == "":
		return errors.New("bitrate_downlink is missing")
	case p.Var5qi == 0:
		return errors.New("Var5qi is missing")
	case p.Arp == 0:
		return errors.New("arp is missing")
	case !isValidBitrate(p.BitrateUplink):
		return errors.New("invalid bitrate_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValidBitrate(p.BitrateDownlink):
		return errors.New("invalid bitrate_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("invalid Var5qi format - must be an integer associated with a non-GBR 5QI")
	case !isValidArp(p.Arp):
		return errors.New("invalid arp format - must be an integer between 1 and 255")
	}

	return nil
}
