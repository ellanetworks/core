package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type N2Interface struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type N3Interface struct {
	Name            string `json:"name"`
	Address         string `json:"address"`
	ExternalAddress string `json:"external_address"`
}

type N6Interface struct {
	Name string `json:"name"`
}

type APIInterface struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

type NetworkInterfaces struct {
	N2  N2Interface  `json:"n2"`
	N3  N3Interface  `json:"n3"`
	N6  N6Interface  `json:"n6"`
	API APIInterface `json:"api"`
}

type UpdateN3SettingsParams struct {
	ExternalAddress string `json:"external_address"`
}

const (
	UpdateN3SettingsAction = "update_n3_settings"
)

func ListNetworkInterfaces(dbInstance *db.Database, cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n3Settings, err := dbInstance.GetN3Settings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to get N3 settings", err, logger.APILog)
			return
		}

		resp := &NetworkInterfaces{
			N2: N2Interface{
				Address: cfg.Interfaces.N2.Address,
				Port:    cfg.Interfaces.N2.Port,
			},
			N3: N3Interface{
				Name:            cfg.Interfaces.N3.Name,
				Address:         cfg.Interfaces.N3.Address,
				ExternalAddress: n3Settings.ExternalAddress,
			},
			N6: N6Interface{
				Name: cfg.Interfaces.N6.Name,
			},
			API: APIInterface{
				// Address: conf.Interfaces.API.Address,
				Port: cfg.Interfaces.API.Port,
			},
		}

		writeResponse(w, resp, http.StatusOK, logger.APILog)
	})
}

func UpdateN3Interface(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateN3SettingsParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if !isValidExternalAddress(params.ExternalAddress) {
			writeError(w, http.StatusBadRequest, "Invalid external address", nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateN3Settings(r.Context(), params.ExternalAddress); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update N3 settings", err, logger.APILog)
			return
		}

		writeResponse(w, map[string]string{"message": "N3 interface updated"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			UpdateN3SettingsAction,
			email,
			getClientIP(r),
			fmt.Sprintf("N3 settings updated: external_address=%q", params.ExternalAddress),
		)
	})
}

// isValidExternalAddress checks if the given address is a valid IP address or fqdn.
func isValidExternalAddress(address string) bool {
	if address == "" {
		return true
	}

	ip := net.ParseIP(address)
	if ip != nil {
		return true
	}

	if strings.Contains(address, " ") || strings.Contains(address, "/") {
		return false
	}

	return true
}
