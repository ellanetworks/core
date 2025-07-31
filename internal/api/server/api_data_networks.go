package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type CreateDataNetworkParams struct {
	Name   string `json:"name"`
	IPPool string `json:"ip-pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type GetDataNetworkResponse struct {
	Name   string `json:"name"`
	IPPool string `json:"ip-pool"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

const (
	ListDataNetworksAction  = "list_data_networks"
	GetDataNetworkAction    = "get_data_network"
	DeleteDataNetworkAction = "delete_data_network"
	CreateDataNetworkAction = "create_data_network"
	UpdateDataNetworkAction = "update_data_network"
)

func ListDataNetworks(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		ctx := r.Context()
		dbDataNetworks, err := dbInstance.ListDataNetworks(ctx)
		if err != nil {
			logger.APILog.Warn("Failed to list data networks", zap.Error(err))
			writeError(w, http.StatusInternalServerError, "Failed to list data networks", err, logger.APILog)
			return
		}

		dataNetworks := make([]GetDataNetworkResponse, 0, len(dbDataNetworks))
		for _, dbDataNetwork := range dbDataNetworks {
			dataNetworks = append(dataNetworks, GetDataNetworkResponse{
				Name:   dbDataNetwork.Name,
				IPPool: dbDataNetwork.IPPool,
				DNS:    dbDataNetwork.DNS,
				MTU:    dbDataNetwork.MTU,
			})
		}

		writeResponse(w, dataNetworks, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			ListDataNetworksAction,
			email,
			getClientIP(r),
			"User listed data networks",
		)
	})
}

func GetDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/data-networks/")
		if name == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		dbDataNetwork, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "Data Network not found", err, logger.APILog)
			return
		}
		dataNetwork := GetDataNetworkResponse{
			Name:   dbDataNetwork.Name,
			IPPool: dbDataNetwork.IPPool,
			DNS:    dbDataNetwork.DNS,
			MTU:    dbDataNetwork.MTU,
		}
		writeResponse(w, dataNetwork, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(GetDataNetworkAction, email, getClientIP(r), "User retrieved data network: "+name)
	})
}

func DeleteDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/v1/data-networks/")
		if name == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		_, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "Data Network not found", err, logger.APILog)
			return
		}
		policiesInDataNetwork, err := dbInstance.PoliciesInDataNetwork(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to check policies", err, logger.APILog)
			return
		}
		if policiesInDataNetwork {
			writeError(w, http.StatusConflict, "Data Network has policies", nil, logger.APILog)
			return
		}
		if err := dbInstance.DeleteDataNetwork(r.Context(), name); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to delete data network", err, logger.APILog)
			return
		}
		writeResponse(w, SuccessResponse{Message: "DataNetwork deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(DeleteDataNetworkAction, email, getClientIP(r), "User deleted data network: "+name)
	})
}

func CreateDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var createDataNetworkParams CreateDataNetworkParams
		if err := json.NewDecoder(r.Body).Decode(&createDataNetworkParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateDataNetworkParams(createDataNetworkParams); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetDataNetwork(r.Context(), createDataNetworkParams.Name); err == nil {
			writeError(w, http.StatusBadRequest, "Data Network already exists", nil, logger.APILog)
			return
		}

		dbDataNetwork := &db.DataNetwork{
			Name:   createDataNetworkParams.Name,
			IPPool: createDataNetworkParams.IPPool,
			DNS:    createDataNetworkParams.DNS,
			MTU:    createDataNetworkParams.MTU,
		}

		if err := dbInstance.CreateDataNetwork(r.Context(), dbDataNetwork); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Data Network created successfully"}, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(CreateDataNetworkAction, email, getClientIP(r), "User created policy: "+createDataNetworkParams.Name)
	})
}

func UpdateDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := strings.TrimPrefix(r.URL.Path, "/api/v1/data-networks/")
		if name == "" || strings.ContainsRune(name, '/') {
			writeError(w, http.StatusBadRequest, "Invalid or missing name parameter", nil, logger.APILog)
			return
		}

		var updateDataNetworkParams CreateDataNetworkParams
		if err := json.NewDecoder(r.Body).Decode(&updateDataNetworkParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateDataNetworkParams(updateDataNetworkParams); err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		policy, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "Data Network not found", err, logger.APILog)
			return
		}

		policy.Name = updateDataNetworkParams.Name
		policy.IPPool = updateDataNetworkParams.IPPool
		policy.DNS = updateDataNetworkParams.DNS
		policy.MTU = updateDataNetworkParams.MTU

		if err := dbInstance.UpdateDataNetwork(r.Context(), policy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Data Network updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateDataNetworkAction, email, getClientIP(r), "User updated policy: "+updateDataNetworkParams.Name)
	})
}

func isDataNetworkNameValid(name string) bool {
	return len(name) > 0 && len(name) < 256
}

func isUeIPPoolValid(ueIPPool string) bool {
	_, _, err := net.ParseCIDR(ueIPPool)
	return err == nil
}

func isValidDNS(dns string) bool {
	return net.ParseIP(dns) != nil
}

func isValidMTU(mtu int32) bool {
	return mtu >= 0 && mtu <= 65535
}

func validateDataNetworkParams(p CreateDataNetworkParams) error {
	switch {
	case p.Name == "":
		return errors.New("name is missing")
	case p.IPPool == "":
		return errors.New("ip-pool is missing")
	case p.DNS == "":
		return errors.New("dns is missing")
	case p.MTU == 0:
		return errors.New("mtu is missing")

	case !isDataNetworkNameValid(p.Name):
		return errors.New("invalid name format, must be less than 256 characters")
	case !isUeIPPoolValid(p.IPPool):
		return errors.New("invalid ue-ip-pool format, must be in CIDR format")
	case !isValidDNS(p.DNS):
		return errors.New("invalid dns format, must be a valid IP address")
	case !isValidMTU(p.MTU):
		return errors.New("invalid mtu format, must be an integer between 0 and 65535")
	}
	return nil
}
