package server

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
)

type CreateDataNetworkParams struct {
	Name   string `json:"name"`
	IPPool string `json:"ip_pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type DataNetworkStatus struct {
	Sessions int `json:"sessions"`
}

type DataNetwork struct {
	Name   string            `json:"name"`
	IPPool string            `json:"ip_pool"`
	DNS    string            `json:"dns,omitempty"`
	MTU    int32             `json:"mtu,omitempty"`
	Status DataNetworkStatus `json:"status"`
}

type ListDataNetworksResponse struct {
	Items      []DataNetwork `json:"items"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalCount int           `json:"total_count"`
}

const (
	DeleteDataNetworkAction = "delete_data_network"
	CreateDataNetworkAction = "create_data_network"
	UpdateDataNetworkAction = "update_data_network"
)

const MaxNumDataNetworks = 12

var dnnRegex = regexp.MustCompile(`^([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$`)

func ListDataNetworks(dbInstance *db.Database) http.Handler {
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

		dbDataNetworks, total, err := dbInstance.ListDataNetworksPage(ctx, page, perPage)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to list data networks", err, logger.APILog)
			return
		}

		items := make([]DataNetwork, 0, len(dbDataNetworks))

		for _, dbDataNetwork := range dbDataNetworks {
			smfSessions := smfContext.PDUSessionsByDNN(dbDataNetwork.Name)

			items = append(items, DataNetwork{
				Name:   dbDataNetwork.Name,
				IPPool: dbDataNetwork.IPPool,
				DNS:    dbDataNetwork.DNS,
				MTU:    dbDataNetwork.MTU,
				Status: DataNetworkStatus{
					Sessions: len(smfSessions),
				},
			})
		}

		dataNetworks := ListDataNetworksResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, dataNetworks, http.StatusOK, logger.APILog)
	})
}

func GetDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}
		dbDataNetwork, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(w, http.StatusNotFound, "Data Network not found", err, logger.APILog)
			return
		}

		smfSessions := smfContext.PDUSessionsByDNN(dbDataNetwork.Name)

		dataNetwork := DataNetwork{
			Name:   dbDataNetwork.Name,
			IPPool: dbDataNetwork.IPPool,
			DNS:    dbDataNetwork.DNS,
			MTU:    dbDataNetwork.MTU,
			Status: DataNetworkStatus{
				Sessions: len(smfSessions),
			},
		}
		writeResponse(w, dataNetwork, http.StatusOK, logger.APILog)
	})
}

func DeleteDataNetwork(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}
		name := r.PathValue("name")
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

		numDataNetworks, err := dbInstance.CountDataNetworks(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to count data networks", err, logger.APILog)
			return
		}

		if numDataNetworks >= MaxNumDataNetworks {
			writeError(w, http.StatusBadRequest, "Maximum number of data networks reached ("+strconv.Itoa(MaxNumPolicies)+")", nil, logger.APILog)
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

		name := r.PathValue("name")
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
	return dnnRegex.MatchString(name)
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
		return errors.New("ip_pool is missing")
	case p.DNS == "":
		return errors.New("dns is missing")
	case p.MTU == 0:
		return errors.New("mtu is missing")

	case !isDataNetworkNameValid(p.Name):
		return errors.New("invalid name format, must be a valid DNN format")
	case !isUeIPPoolValid(p.IPPool):
		return errors.New("invalid ip_pool format, must be in CIDR format")
	case !isValidDNS(p.DNS):
		return errors.New("invalid dns format, must be a valid IP address")
	case !isValidMTU(p.MTU):
		return errors.New("invalid mtu format, must be an integer between 0 and 65535")
	}
	return nil
}
