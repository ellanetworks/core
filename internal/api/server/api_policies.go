// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	MaxNumNetworkRulesPerDirection = 12
	DirectionUplink                = "uplink"
	DirectionDownlink              = "downlink"
)

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

func toDBPolicyRules(rules *PolicyRules) *db.PolicyRulesInput {
	if rules == nil {
		return nil
	}

	convert := func(in []PolicyRule) []db.PolicyRuleInput {
		out := make([]db.PolicyRuleInput, 0, len(in))
		for _, rule := range in {
			out = append(out, db.PolicyRuleInput{
				Description:  rule.Description,
				RemotePrefix: rule.RemotePrefix,
				Protocol:     rule.Protocol,
				PortLow:      rule.PortLow,
				PortHigh:     rule.PortHigh,
				Action:       rule.Action,
			})
		}

		return out
	}

	return &db.PolicyRulesInput{
		Uplink:   convert(rules.Uplink),
		Downlink: convert(rules.Downlink),
	}
}

type CreatePolicyParams struct {
	Name                string       `json:"name"`
	ProfileName         string       `json:"profile_name"`
	SliceName           string       `json:"slice_name"`
	DataNetworkName     string       `json:"data_network_name"`
	SessionAmbrUplink   string       `json:"session_ambr_uplink"`
	SessionAmbrDownlink string       `json:"session_ambr_downlink"`
	Var5qi              int32        `json:"var5qi,omitempty"`
	Arp                 int32        `json:"arp,omitempty"`
	Rules               *PolicyRules `json:"rules,omitempty"`
	// Default marks this the profile's default data-network binding (default
	// APN/DNN). The first policy created in a profile becomes the default
	// regardless, so a profile always has exactly one.
	Default *bool `json:"default,omitempty"`
}

type UpdatePolicyParams struct {
	ProfileName         string       `json:"profile_name"`
	SliceName           string       `json:"slice_name"`
	DataNetworkName     string       `json:"data_network_name"`
	SessionAmbrUplink   string       `json:"session_ambr_uplink"`
	SessionAmbrDownlink string       `json:"session_ambr_downlink"`
	Var5qi              int32        `json:"var5qi,omitempty"`
	Arp                 int32        `json:"arp,omitempty"`
	Rules               *PolicyRules `json:"rules,omitempty"`
	// Default, when true, makes this the profile's default binding (clearing the
	// previous default). Omitted/false leaves the current default unchanged.
	Default *bool `json:"default,omitempty"`
}

type Policy struct {
	Name                string       `json:"name"`
	ProfileName         string       `json:"profile_name"`
	SliceName           string       `json:"slice_name"`
	DataNetworkName     string       `json:"data_network_name"`
	SessionAmbrUplink   string       `json:"session_ambr_uplink"`
	SessionAmbrDownlink string       `json:"session_ambr_downlink"`
	Var5qi              int32        `json:"var5qi,omitempty"`
	Arp                 int32        `json:"arp,omitempty"`
	Rules               *PolicyRules `json:"rules,omitempty"`
	Default             bool         `json:"default"`
}

// qciCompatible5Qi is the set of standardized 5QI values that have a QCI
// counterpart (the QCI∩5QI standardized intersection, TS 23.203 Table 6.1.7 /
// TS 23.501 Table 5.7.4-1). A policy on a profile that permits 4G must use one
// of these so the value is meaningful as a QCI on S1AP; 5G-only / operator
// specific 5QIs are rejected for 4G-capable profiles.
var qciCompatible5Qi = []int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 65, 66, 67, 69, 70, 75, 79, 80, 82, 83}

func is4GCompatible5Qi(var5qi int32) bool {
	return slices.Contains(qciCompatible5Qi, var5qi)
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
	MaxNumPoliciesPerProfile = 12
)

func isResourceNameValid(name string) bool {
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

	_, err := netip.ParsePrefix(*prefix)
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

	if len(rule.Description) > 256 {
		return errors.New("rule description must be 256 characters or fewer")
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

func ListPolicies(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)
		profileName := q.Get("profile_name")

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		var (
			dbPolicies []db.Policy
			total      int
			err        error
		)

		if profileName != "" {
			profile, profileErr := dbInstance.GetProfile(ctx, profileName)
			if profileErr != nil {
				writeError(r.Context(), w, http.StatusNotFound, "Profile not found", profileErr, logger.APILog)
				return
			}

			dbPolicies, total, err = dbInstance.ListPoliciesByProfilePage(ctx, profile.ID, page, perPage)
		} else {
			dbPolicies, total, err = dbInstance.ListPoliciesPage(ctx, page, perPage)
		}

		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Policies not found", err, logger.APILog)
			return
		}

		policyList := make([]Policy, 0)

		for _, dbPolicy := range dbPolicies {
			profile, err := dbInstance.GetProfileByID(ctx, dbPolicy.ProfileID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}

			slice, err := dbInstance.GetNetworkSliceByID(ctx, dbPolicy.SliceID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}

			dataNetwork, err := dbInstance.GetDataNetworkByID(ctx, dbPolicy.DataNetworkID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}

			policyList = append(policyList, Policy{
				Name:                dbPolicy.Name,
				ProfileName:         profile.Name,
				SliceName:           slice.Name,
				DataNetworkName:     dataNetwork.Name,
				SessionAmbrDownlink: dbPolicy.SessionAmbrDownlink,
				SessionAmbrUplink:   dbPolicy.SessionAmbrUplink,
				Var5qi:              dbPolicy.Var5qi,
				Arp:                 dbPolicy.Arp,
				Default:             dbPolicy.IsDefault,
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

func getPolicyRulesForPolicy(ctx context.Context, dbInstance *db.Database, policyID string) (*PolicyRules, error) {
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
		case DirectionUplink:
			policyRules.Uplink = append(policyRules.Uplink, apiRule)
		case DirectionDownlink:
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
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Policy not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)

			return
		}

		profile, err := dbInstance.GetProfileByID(r.Context(), dbPolicy.ProfileID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		slice, err := dbInstance.GetNetworkSliceByID(r.Context(), dbPolicy.SliceID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetworkByID(r.Context(), dbPolicy.DataNetworkID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}

		rules, err := getPolicyRulesForPolicy(r.Context(), dbInstance, dbPolicy.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to retrieve policy rules", err, logger.APILog)
			return
		}

		policy := Policy{
			Name:                dbPolicy.Name,
			ProfileName:         profile.Name,
			SliceName:           slice.Name,
			DataNetworkName:     dataNetwork.Name,
			SessionAmbrDownlink: dbPolicy.SessionAmbrDownlink,
			SessionAmbrUplink:   dbPolicy.SessionAmbrUplink,
			Var5qi:              dbPolicy.Var5qi,
			Arp:                 dbPolicy.Arp,
			Rules:               rules,
			Default:             dbPolicy.IsDefault,
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

// checkPolicyBindingFree reports whether a policy may bind dataNetworkID under
// profile/slice, excluding excludeName (empty on create).
//
// A subscription may configure a given data network once per slice: 5G keys a
// slice's DNN configurations by DNN (TS 29.503 §6.1.6.2.8). When the profile
// also permits 4G the rule tightens to once per profile, because EPS has no
// slice to disambiguate and requires the APN to be unique across a subscriber's
// configurations (TS 29.272 §7.3.35).
func checkPolicyBindingFree(ctx context.Context, dbInstance *db.Database, profile *db.Profile, sliceID, dataNetworkID, dataNetworkName, excludeName string) error {
	if profile.Allow4G {
		policies, err := dbInstance.ListPoliciesByProfile(ctx, profile.ID)
		if err != nil {
			return fmt.Errorf("list policies: %w", err)
		}

		for i := range policies {
			if policies[i].Name == excludeName || policies[i].DataNetworkID != dataNetworkID {
				continue
			}

			return fmt.Errorf("policy %q already uses data network %q; a profile that allows 4G may use each data network only once", policies[i].Name, dataNetworkName)
		}

		return nil
	}

	existing, err := dbInstance.GetPolicyByLookup(ctx, profile.ID, sliceID, dataNetworkID)
	if errors.Is(err, db.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("look up policy: %w", err)
	}

	if existing.Name == excludeName {
		return nil
	}

	return fmt.Errorf("policy %q already binds this slice to data network %q", existing.Name, dataNetworkName)
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

		profile, err := dbInstance.GetProfile(r.Context(), createPolicyParams.ProfileName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
			return
		}

		// A 4G-capable profile's bindings must use a QCI-compatible 5QI so the
		// value is valid on S1AP (TS 23.203 Table 6.1.7).
		if profile.Allow4G && !is4GCompatible5Qi(createPolicyParams.Var5qi) {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("5QI %d is not valid for a 4G-capable profile (no QCI counterpart)", createPolicyParams.Var5qi),
				nil, logger.APILog)

			return
		}

		numPolicies, err := dbInstance.CountPoliciesInProfile(r.Context(), profile.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count policies", err, logger.APILog)
			return
		}

		if numPolicies >= MaxNumPoliciesPerProfile {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of policies per profile reached ("+strconv.Itoa(MaxNumPoliciesPerProfile)+")", nil, logger.APILog)
			return
		}

		slice, err := dbInstance.GetNetworkSlice(r.Context(), createPolicyParams.SliceName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), createPolicyParams.DataNetworkName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		if err := checkPolicyBindingFree(r.Context(), dbInstance, profile, slice.ID, dataNetwork.ID, createPolicyParams.DataNetworkName, ""); err != nil {
			writeError(r.Context(), w, http.StatusConflict, err.Error(), nil, logger.APILog)
			return
		}

		dbPolicy := &db.Policy{
			Name:                createPolicyParams.Name,
			SessionAmbrDownlink: createPolicyParams.SessionAmbrDownlink,
			SessionAmbrUplink:   createPolicyParams.SessionAmbrUplink,
			Var5qi:              createPolicyParams.Var5qi,
			Arp:                 createPolicyParams.Arp,
			DataNetworkID:       dataNetwork.ID,
			ProfileID:           profile.ID,
			SliceID:             slice.ID,
		}

		if createPolicyParams.Rules != nil {
			if err := dbInstance.CreatePolicyWithRules(r.Context(), dbPolicy, toDBPolicyRules(createPolicyParams.Rules)); err != nil {
				if errors.Is(err, db.ErrAlreadyExists) {
					writeError(r.Context(), w, http.StatusConflict, "Policy already exists", nil, logger.APILog)
					return
				}

				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create policy", err, logger.APILog)

				return
			}
		} else {
			if err := dbInstance.CreatePolicy(r.Context(), dbPolicy); err != nil {
				if errors.Is(err, db.ErrAlreadyExists) {
					writeError(r.Context(), w, http.StatusConflict, "Policy already exists", nil, logger.APILog)
					return
				}

				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create policy", err, logger.APILog)

				return
			}
		}

		// The first policy in a profile becomes its default binding; a later one
		// becomes default only when explicitly requested (TS 23.401 §3.1 default APN).
		if numPolicies == 0 || boolOr(createPolicyParams.Default, false) {
			if err := dbInstance.SetDefaultPolicy(r.Context(), profile.ID, createPolicyParams.Name); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to set default policy", err, logger.APILog)
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

		profile, err := dbInstance.GetProfile(r.Context(), updatePolicyParams.ProfileName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Profile not found", nil, logger.APILog)
			return
		}

		if profile.Allow4G && !is4GCompatible5Qi(updatePolicyParams.Var5qi) {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("5QI %d is not valid for a 4G-capable profile (no QCI counterpart)", updatePolicyParams.Var5qi),
				nil, logger.APILog)

			return
		}

		slice, err := dbInstance.GetNetworkSlice(r.Context(), updatePolicyParams.SliceName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Slice not found", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), updatePolicyParams.DataNetworkName)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		if err := checkPolicyBindingFree(r.Context(), dbInstance, profile, slice.ID, dataNetwork.ID, updatePolicyParams.DataNetworkName, policyName); err != nil {
			writeError(r.Context(), w, http.StatusConflict, err.Error(), nil, logger.APILog)
			return
		}

		policy.Name = policyName
		policy.SessionAmbrDownlink = updatePolicyParams.SessionAmbrDownlink
		policy.SessionAmbrUplink = updatePolicyParams.SessionAmbrUplink
		policy.Var5qi = updatePolicyParams.Var5qi
		policy.Arp = updatePolicyParams.Arp
		policy.ProfileID = profile.ID
		policy.SliceID = slice.ID
		policy.DataNetworkID = dataNetwork.ID

		if err := dbInstance.UpdatePolicyWithRules(r.Context(), policy, toDBPolicyRules(updatePolicyParams.Rules)); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update policy", err, logger.APILog)
			return
		}

		if boolOr(updatePolicyParams.Default, false) {
			if err := dbInstance.SetDefaultPolicy(r.Context(), profile.ID, policyName); err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to set default policy", err, logger.APILog)
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
	case p.ProfileName == "":
		return errors.New("profile_name is missing")
	case p.SliceName == "":
		return errors.New("slice_name is missing")
	case p.DataNetworkName == "":
		return errors.New("data_network_name is missing")
	case p.SessionAmbrUplink == "":
		return errors.New("session_ambr_uplink is missing")
	case p.SessionAmbrDownlink == "":
		return errors.New("session_ambr_downlink is missing")
	case p.Var5qi == 0:
		return errors.New("Var5qi is missing")
	case p.Arp == 0:
		return errors.New("arp is missing")
	case !isResourceNameValid(p.Name):
		return errors.New("invalid name format - must be less than 256 characters")
	case !isValidBitrate(p.SessionAmbrUplink):
		return errors.New("invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValidBitrate(p.SessionAmbrDownlink):
		return errors.New("invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("invalid Var5qi format - must be an integer associated with a non-GBR 5QI")
	case !isValidArp(p.Arp):
		return errors.New("invalid arp format - must be an integer between 1 and 15")
	}

	if err := validatePolicyRules(p.Rules); err != nil {
		return err
	}

	return nil
}

func validateUpdatePolicyParams(p UpdatePolicyParams) error {
	switch {
	case p.ProfileName == "":
		return errors.New("profile_name is missing")
	case p.SliceName == "":
		return errors.New("slice_name is missing")
	case p.DataNetworkName == "":
		return errors.New("data_network_name is missing")
	case p.SessionAmbrUplink == "":
		return errors.New("session_ambr_uplink is missing")
	case p.SessionAmbrDownlink == "":
		return errors.New("session_ambr_downlink is missing")
	case p.Var5qi == 0:
		return errors.New("Var5qi is missing")
	case p.Arp == 0:
		return errors.New("arp is missing")
	case !isValidBitrate(p.SessionAmbrUplink):
		return errors.New("invalid session_ambr_uplink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValidBitrate(p.SessionAmbrDownlink):
		return errors.New("invalid session_ambr_downlink format - must be in the format `<number> <unit>`, allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("invalid Var5qi format - must be an integer associated with a non-GBR 5QI")
	case !isValidArp(p.Arp):
		return errors.New("invalid arp format - must be an integer between 1 and 15")
	}

	if err := validatePolicyRules(p.Rules); err != nil {
		return err
	}

	return nil
}
