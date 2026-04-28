package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type N2Interface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
	Interface string   `json:"interface,omitempty"`
}

type N3Interface struct {
	Name            string   `json:"name"`
	Addresses       []string `json:"addresses"`
	ExternalAddress string   `json:"external_address"`
	Vlan            *Vlan    `json:"vlan,omitempty"`
}

type Vlan struct {
	MasterInterface string `json:"master_interface"`
	VlanId          int    `json:"vlan_id"`
}

type N6Interface struct {
	Name string `json:"name"`
	Vlan *Vlan  `json:"vlan,omitempty"`
}

type APIInterface struct {
	Addresses []string `json:"addresses"`
	Port      int      `json:"port"`
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
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get N3 settings", err, logger.APILog)
			return
		}

		var apiAddresses []string

		if cfg.Interfaces.API.Name != "" {
			ips, err := config.GetInterfaceIPs(cfg.Interfaces.API.Name)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get API interface IPs", err, logger.APILog)
				return
			}

			apiAddresses = ips
		} else if cfg.Interfaces.API.Address != "" {
			apiAddresses = []string{cfg.Interfaces.API.Address}
		}

		var n2Addresses []string

		if cfg.Interfaces.N2.Name != "" {
			ips, err := config.GetInterfaceIPs(cfg.Interfaces.N2.Name)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get N2 interface IPs", err, logger.APILog)
				return
			}

			n2Addresses = ips
		} else if cfg.Interfaces.N2.Address != "" {
			n2Addresses = []string{cfg.Interfaces.N2.Address}
		}

		var n3Addresses []string

		if cfg.Interfaces.N3.Name != "" {
			ips, err := config.GetInterfaceIPs(cfg.Interfaces.N3.Name)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get N3 interface IPs", err, logger.APILog)
				return
			}

			n3Addresses = ips
		} else if cfg.Interfaces.N3.Address != "" {
			n3Addresses = []string{cfg.Interfaces.N3.Address}
		}

		resp := &NetworkInterfaces{
			N2: N2Interface{
				Addresses: n2Addresses,
				Port:      cfg.Interfaces.N2.Port,
				Interface: cfg.Interfaces.N2.Name,
			},
			N3: N3Interface{
				Name:            cfg.Interfaces.N3.Name,
				Addresses:       n3Addresses,
				ExternalAddress: n3Settings.ExternalAddress,
			},
			N6: N6Interface{
				Name: cfg.Interfaces.N6.Name,
			},
			API: APIInterface{
				Addresses: apiAddresses,
				Port:      cfg.Interfaces.API.Port,
			},
		}

		if cfg.Interfaces.N3.VlanConfig != nil {
			resp.N3.Vlan = &Vlan{
				MasterInterface: cfg.Interfaces.N3.VlanConfig.MasterInterface,
				VlanId:          cfg.Interfaces.N3.VlanConfig.VlanId,
			}
		}

		if cfg.Interfaces.N6.VlanConfig != nil {
			resp.N6.Vlan = &Vlan{
				MasterInterface: cfg.Interfaces.N6.VlanConfig.MasterInterface,
				VlanId:          cfg.Interfaces.N6.VlanConfig.VlanId,
			}
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func UpdateN3Interface(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateN3SettingsParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		// Empty means "use the local interface IP"; the upf reconciler
		// resolves that against each node's local config when applying.
		// A non-empty value must be a valid IP.
		if params.ExternalAddress != "" {
			if _, err := netip.ParseAddr(params.ExternalAddress); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, "Invalid external address. Must be a valid IP address", err, logger.APILog)
				return
			}
		}

		if err := dbInstance.UpdateN3Settings(r.Context(), params.ExternalAddress); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update N3 settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "N3 interface updated"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateN3SettingsAction,
			email,
			getClientIP(r),
			fmt.Sprintf("N3 settings updated: external_address=%q", params.ExternalAddress),
		)
	})
}
