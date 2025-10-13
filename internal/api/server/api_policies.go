package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreatePolicyParams struct {
	Name            string `json:"name"`
	BitrateUplink   string `json:"bitrate_uplink,omitempty"`
	BitrateDownlink string `json:"bitrate_downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	Arp             int32  `json:"arp,omitempty"`
	DataNetworkName string `json:"data_network_name,omitempty"`
}

type Policy struct {
	Name            string `json:"name"`
	BitrateUplink   string `json:"bitrate_uplink,omitempty"`
	BitrateDownlink string `json:"bitrate_downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	Arp             int32  `json:"arp,omitempty"`
	DataNetworkName string `json:"data_network_name,omitempty"`
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
	return valueInt > 0 && valueInt <= 1000
}

var valid5Qi = []int32{5, 6, 7, 8, 9, 69, 70, 79, 80} // only non-gbr 5Qi are supported for now

func isValid5Qi(var5qi int32) bool {
	return slices.Contains(valid5Qi, var5qi)
}

func isValidArp(arp int32) bool {
	return arp >= 1 && arp <= 15
}

func ListPolicies(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		dbPolicies, total, err := dbInstance.ListPoliciesPage(ctx, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Policies not found", err, logger.APILog)
			return
		}

		policyList := make([]Policy, 0)
		for _, dbPolicy := range dbPolicies {
			dataNetwork, err := dbInstance.GetDataNetworkByID(ctx, dbPolicy.DataNetworkID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
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

		writeResponse(w, resp, http.StatusOK, logger.APILog)
	})
}

func GetPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/policies/")
		if name == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		dbPolicy, err := dbInstance.GetPolicy(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", err, logger.APILog)
			return
		}
		dataNetwork, err := dbInstance.GetDataNetworkByID(r.Context(), dbPolicy.DataNetworkID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
			return
		}
		policy := Policy{
			Name:            dbPolicy.Name,
			BitrateDownlink: dbPolicy.BitrateDownlink,
			BitrateUplink:   dbPolicy.BitrateUplink,
			Var5qi:          dbPolicy.Var5qi,
			Arp:             dbPolicy.Arp,
			DataNetworkName: dataNetwork.Name,
		}
		writeResponse(w, policy, http.StatusOK, logger.APILog)
	})
}

func DeletePolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/policies/")
		if name == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		_, err := dbInstance.GetPolicy(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", err, logger.APILog)
			return
		}
		subsInPolicy, err := dbInstance.SubscribersInPolicy(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to check subscribers", err, logger.APILog)
			return
		}
		if subsInPolicy {
			writeError(w, http.StatusConflict, "Policy has subscribers", nil, logger.APILog)
			return
		}
		if err := dbInstance.DeletePolicy(r.Context(), name); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to delete policy", err, logger.APILog)
			return
		}
		writeResponse(w, SuccessResponse{Message: "Policy deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeletePolicyAction, email, getClientIP(r), "User deleted policy: "+name)
	})
}

func CreatePolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var createPolicyParams CreatePolicyParams
		if err := json.NewDecoder(r.Body).Decode(&createPolicyParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validatePolicyParams(createPolicyParams); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetPolicy(r.Context(), createPolicyParams.Name); err == nil {
			writeError(w, http.StatusBadRequest, "Policy already exists", nil, logger.APILog)
			return
		}

		numPolicies, err := dbInstance.CountPolicies(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count policies", err, logger.APILog)
			return
		}

		if numPolicies >= MaxNumPolicies {
			writeError(w, http.StatusBadRequest, "Maximum number of policies reached ("+strconv.Itoa(MaxNumPolicies)+")", nil, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), createPolicyParams.DataNetworkName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", err, logger.APILog)
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
			writeError(w, http.StatusInternalServerError, "Failed to create policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Policy created successfully"}, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(CreatePolicyAction, email, getClientIP(r), "User created policy: "+createPolicyParams.Name)
	})
}

func UpdatePolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		groupName := strings.TrimPrefix(r.URL.Path, "/api/v1/policies/")
		if groupName == "" || strings.ContainsRune(groupName, '/') {
			writeError(w, http.StatusBadRequest, "Invalid or missing name parameter", nil, logger.APILog)
			return
		}

		var updatePolicyParams CreatePolicyParams
		if err := json.NewDecoder(r.Body).Decode(&updatePolicyParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validatePolicyParams(updatePolicyParams); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		policy, err := dbInstance.GetPolicy(r.Context(), groupName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Policy not found", err, logger.APILog)
			return
		}

		dataNetwork, err := dbInstance.GetDataNetwork(r.Context(), updatePolicyParams.DataNetworkName)
		if err != nil {
			writeError(w, http.StatusNotFound, "Data Network not found", err, logger.APILog)
			return
		}

		policy.Name = updatePolicyParams.Name
		policy.BitrateDownlink = updatePolicyParams.BitrateDownlink
		policy.BitrateUplink = updatePolicyParams.BitrateUplink
		policy.Var5qi = updatePolicyParams.Var5qi
		policy.Arp = updatePolicyParams.Arp
		policy.DataNetworkID = dataNetwork.ID

		if err := dbInstance.UpdatePolicy(r.Context(), policy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdatePolicyAction, email, getClientIP(r), "User updated policy: "+updatePolicyParams.Name)
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
		return errors.New("Invalid name format. Must be less than 256 characters")
	case !isValidBitrate(p.BitrateUplink):
		return errors.New("Invalid bitrate_uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
	case !isValidBitrate(p.BitrateDownlink):
		return errors.New("Invalid bitrate_downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("Invalid Var5qi format. Must be an integer associated with a non-GBR 5QI")
	case !isValidArp(p.Arp):
		return errors.New("Invalid arp format. Must be an integer between 1 and 255")
	}
	return nil
}
