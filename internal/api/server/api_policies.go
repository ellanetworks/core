package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreatePolicyParams struct {
	Name string `json:"name"`

	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
	DataNetworkName string `json:"data-network-name,omitempty"`
}

type GetPolicyResponse struct {
	Name string `json:"name"`

	BitrateUplink   string `json:"bitrate-uplink,omitempty"`
	BitrateDownlink string `json:"bitrate-downlink,omitempty"`
	Var5qi          int32  `json:"var5qi,omitempty"`
	PriorityLevel   int32  `json:"priority-level,omitempty"`
	DataNetworkName string `json:"data-network-name,omitempty"`
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

func isValid5Qi(var5qi int32) bool {
	return var5qi >= 1 && var5qi <= 255
}

func isValidPriorityLevel(priorityLevel int32) bool {
	return priorityLevel >= 1 && priorityLevel <= 255
}

func ListPolicies(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		dbPolicies, err := dbInstance.ListPolicies(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Policies not found", err, logger.APILog)
			return
		}
		policyList := make([]GetPolicyResponse, 0)
		for _, dbPolicy := range dbPolicies {
			dataNetwork, err := dbInstance.GetDataNetworkByID(ctx, dbPolicy.DataNetworkID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to retrieve policy", err, logger.APILog)
				return
			}
			policyList = append(policyList, GetPolicyResponse{
				Name:            dbPolicy.Name,
				BitrateDownlink: dbPolicy.BitrateDownlink,
				BitrateUplink:   dbPolicy.BitrateUplink,
				Var5qi:          dbPolicy.Var5qi,
				PriorityLevel:   dbPolicy.PriorityLevel,
				DataNetworkName: dataNetwork.Name,
			})
		}
		writeResponse(w, policyList, http.StatusOK, logger.APILog)
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
		policy := GetPolicyResponse{
			Name:            dbPolicy.Name,
			BitrateDownlink: dbPolicy.BitrateDownlink,
			BitrateUplink:   dbPolicy.BitrateUplink,
			Var5qi:          dbPolicy.Var5qi,
			PriorityLevel:   dbPolicy.PriorityLevel,
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

		numPolicies, err := dbInstance.NumPolicies(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve policies", err, logger.APILog)
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
			PriorityLevel:   createPolicyParams.PriorityLevel,
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
		policy.PriorityLevel = updatePolicyParams.PriorityLevel
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
		return errors.New("data-network-name is missing")
	case p.BitrateUplink == "":
		return errors.New("bitrate-uplink is missing")
	case p.BitrateDownlink == "":
		return errors.New("bitrate-downlink is missing")
	case p.Var5qi == 0:
		return errors.New("Var5qi is missing")
	case p.PriorityLevel == 0:
		return errors.New("priority-level is missing")
	case !isPolicyNameValid(p.Name):
		return errors.New("Invalid name format. Must be less than 256 characters")
	case !isValidBitrate(p.BitrateUplink):
		return errors.New("Invalid bitrate-uplink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
	case !isValidBitrate(p.BitrateDownlink):
		return errors.New("Invalid bitrate-downlink format. Must be in the format `<number> <unit>`. Allowed units are Mbps, Gbps")
	case !isValid5Qi(p.Var5qi):
		return errors.New("Invalid Var5qi format. Must be an integer between 1 and 255")
	case !isValidPriorityLevel(p.PriorityLevel):
		return errors.New("Invalid priority-level format. Must be an integer between 1 and 255")
	}
	return nil
}
