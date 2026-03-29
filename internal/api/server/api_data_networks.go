package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf"
)

type CreateDataNetworkParams struct {
	Name   string `json:"name"`
	IPPool string `json:"ip_pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type UpdateDataNetworkParams struct {
	IPPool string `json:"ip_pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type DataNetworkStatus struct {
	Sessions       int `json:"sessions"`
	UsedAddresses  int `json:"used_addresses"`
	TotalAddresses int `json:"total_addresses"`
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

// poolUtilization returns the number of used and total addresses for a data network pool.
func poolUtilization(ctx context.Context, dbInstance *db.Database, dn db.DataNetwork) (used, total int) {
	pool, err := ipam.NewPool(dn.ID, dn.IPPool)
	if err != nil {
		return 0, 0
	}

	total = pool.Size()

	used, err = dbInstance.CountLeasesByPool(ctx, dn.ID)
	if err != nil {
		return 0, total
	}

	return used, total
}

func ListDataNetworks(dbInstance *db.Database, sessions smf.SessionQuerier) http.Handler {
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

		dbDataNetworks, total, err := dbInstance.ListDataNetworksPage(ctx, page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list data networks", err, logger.APILog)
			return
		}

		items := make([]DataNetwork, 0, len(dbDataNetworks))

		for _, dbDataNetwork := range dbDataNetworks {
			var sessionCount int
			if sessions != nil {
				sessionCount = len(sessions.SessionsByDNN(dbDataNetwork.Name))
			}

			used, total := poolUtilization(ctx, dbInstance, dbDataNetwork)

			items = append(items, DataNetwork{
				Name:   dbDataNetwork.Name,
				IPPool: dbDataNetwork.IPPool,
				DNS:    dbDataNetwork.DNS,
				MTU:    dbDataNetwork.MTU,
				Status: DataNetworkStatus{
					Sessions:       sessionCount,
					UsedAddresses:  used,
					TotalAddresses: total,
				},
			})
		}

		dataNetworks := ListDataNetworksResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(r.Context(), w, dataNetworks, http.StatusOK, logger.APILog)
	})
}

func GetDataNetwork(dbInstance *db.Database, sessions smf.SessionQuerier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		dbDataNetwork, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		var sessionCount int
		if sessions != nil {
			sessionCount = len(sessions.SessionsByDNN(dbDataNetwork.Name))
		}

		used, total := poolUtilization(r.Context(), dbInstance, *dbDataNetwork)

		dataNetwork := DataNetwork{
			Name:   dbDataNetwork.Name,
			IPPool: dbDataNetwork.IPPool,
			DNS:    dbDataNetwork.DNS,
			MTU:    dbDataNetwork.MTU,
			Status: DataNetworkStatus{
				Sessions:       sessionCount,
				UsedAddresses:  used,
				TotalAddresses: total,
			},
		}
		writeResponse(r.Context(), w, dataNetwork, http.StatusOK, logger.APILog)
	})
}

func DeleteDataNetwork(dbInstance *db.Database, cfg config.Config, bgpService *bgp.BGPService) http.Handler {
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

		policiesInDataNetwork, err := dbInstance.PoliciesInDataNetwork(r.Context(), name)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check policies", err, logger.APILog)

			return
		}

		if policiesInDataNetwork {
			writeError(r.Context(), w, http.StatusConflict, "Data Network has policies", nil, logger.APILog)
			return
		}

		if err := dbInstance.DeleteDataNetwork(r.Context(), name); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete data network", err, logger.APILog)

			return
		}

		rebuildBGPFilter(r.Context(), dbInstance, cfg, bgpService)

		writeResponse(r.Context(), w, SuccessResponse{Message: "DataNetwork deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteDataNetworkAction, email, getClientIP(r), "User deleted data network: "+name)
	})
}

func CreateDataNetwork(dbInstance *db.Database, cfg config.Config, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var createDataNetworkParams CreateDataNetworkParams
		if err := json.NewDecoder(r.Body).Decode(&createDataNetworkParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateDataNetworkParams(createDataNetworkParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := validateNoOverlap(r.Context(), dbInstance, createDataNetworkParams.IPPool, ""); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		numDataNetworks, err := dbInstance.CountDataNetworks(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count data networks", err, logger.APILog)
			return
		}

		if numDataNetworks >= MaxNumDataNetworks {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of data networks reached ("+strconv.Itoa(MaxNumDataNetworks)+")", nil, logger.APILog)
			return
		}

		dbDataNetwork := &db.DataNetwork{
			Name:   createDataNetworkParams.Name,
			IPPool: createDataNetworkParams.IPPool,
			DNS:    createDataNetworkParams.DNS,
			MTU:    createDataNetworkParams.MTU,
		}

		if err := dbInstance.CreateDataNetwork(r.Context(), dbDataNetwork); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "Data Network already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create data network", err, logger.APILog)

			return
		}

		rebuildBGPFilter(r.Context(), dbInstance, cfg, bgpService)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Data Network created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateDataNetworkAction, email, getClientIP(r), "User created data network: "+createDataNetworkParams.Name)
	})
}

func UpdateDataNetwork(dbInstance *db.Database, cfg config.Config, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := r.PathValue("name")
		if name == "" || strings.ContainsRune(name, '/') {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid or missing name parameter", nil, logger.APILog)
			return
		}

		var updateDataNetworkParams UpdateDataNetworkParams

		if err := json.NewDecoder(r.Body).Decode(&updateDataNetworkParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateUpdateDataNetworkParams(updateDataNetworkParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := validateNoOverlap(r.Context(), dbInstance, updateDataNetworkParams.IPPool, name); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		dn := &db.DataNetwork{
			Name:   name,
			IPPool: updateDataNetworkParams.IPPool,
			DNS:    updateDataNetworkParams.DNS,
			MTU:    updateDataNetworkParams.MTU,
		}

		if err := dbInstance.UpdateDataNetwork(r.Context(), dn); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update data network", err, logger.APILog)

			return
		}

		rebuildBGPFilter(r.Context(), dbInstance, cfg, bgpService)

		writeResponse(r.Context(), w, SuccessResponse{Message: "Data Network updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateDataNetworkAction, email, getClientIP(r), "User updated data network: "+name)
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

func validateUpdateDataNetworkParams(p UpdateDataNetworkParams) error {
	switch {
	case p.IPPool == "":
		return errors.New("ip_pool is missing")
	case p.DNS == "":
		return errors.New("dns is missing")
	case p.MTU == 0:
		return errors.New("mtu is missing")
	case !isUeIPPoolValid(p.IPPool):
		return errors.New("invalid ip_pool format, must be in CIDR format")
	case !isValidDNS(p.DNS):
		return errors.New("invalid dns format, must be a valid IP address")
	case !isValidMTU(p.MTU):
		return errors.New("invalid mtu format, must be an integer between 0 and 65535")
	}

	return nil
}

// validateNoOverlap checks that cidr does not overlap with any existing data
// network pool. excludeName is the name of the data network being updated
// (empty for create) — its own pool is excluded from the comparison.
func validateNoOverlap(ctx context.Context, dbInstance *db.Database, cidr string, excludeName string) error {
	newPrefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	newPrefix = netip.PrefixFrom(newPrefix.Masked().Addr(), newPrefix.Bits())

	existing, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		return fmt.Errorf("failed to list data networks: %w", err)
	}

	for _, dn := range existing {
		if dn.Name == excludeName {
			continue
		}

		if dn.IPPool == "" {
			continue
		}

		existingPrefix, parseErr := netip.ParsePrefix(dn.IPPool)
		if parseErr != nil {
			continue
		}

		existingPrefix = netip.PrefixFrom(existingPrefix.Masked().Addr(), existingPrefix.Bits())

		if newPrefix.Overlaps(existingPrefix) {
			return fmt.Errorf("pool %s overlaps with data network %q (%s)", newPrefix, dn.Name, existingPrefix)
		}
	}

	return nil
}

// rebuildBGPFilter rebuilds the BGP safety rejection filter from the current
// data networks and interface configuration and applies it to the BGP service.
// It is called after data network create/update/delete operations.
func rebuildBGPFilter(ctx context.Context, dbInstance *db.Database, cfg config.Config, bgpService *bgp.BGPService) {
	if bgpService == nil {
		return
	}

	uePools := collectUEPools(ctx, dbInstance)
	filter := bgp.BuildRouteFilter(uePools, net.ParseIP(cfg.Interfaces.N3.Address), cfg.Interfaces.N6.Name)
	bgpService.UpdateFilter(filter)
}

// collectUEPools returns the UE IP pool CIDRs from all data networks.
func collectUEPools(ctx context.Context, dbInstance *db.Database) []*net.IPNet {
	dataNetworks, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		logger.APILog.Warn("failed to list data networks for BGP filter rebuild")

		return nil
	}

	var pools []*net.IPNet

	for _, dn := range dataNetworks {
		_, network, err := net.ParseCIDR(dn.IPPool)
		if err != nil {
			continue
		}

		pools = append(pools, network)
	}

	return pools
}
