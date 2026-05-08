package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

type CreateDataNetworkParams struct {
	Name     string `json:"name"`
	IPv4Pool string `json:"ipv4_pool,omitempty"`
	IPv6Pool string `json:"ipv6_pool,omitempty"`
	DNS      string `json:"dns,omitempty"`
	MTU      int32  `json:"mtu,omitempty"`
}

type UpdateDataNetworkParams struct {
	IPv4Pool string `json:"ipv4_pool,omitempty"`
	IPv6Pool string `json:"ipv6_pool,omitempty"`
	DNS      string `json:"dns,omitempty"`
	MTU      int32  `json:"mtu,omitempty"`
}

type DataNetworkStatus struct {
	Sessions int `json:"sessions"`
}

type DataNetworkIPAllocation struct {
	PoolSize  int `json:"pool_size"`
	Allocated int `json:"allocated"`
	Available int `json:"available"`
}

type DataNetwork struct {
	Name           string                   `json:"name"`
	IPv4Pool       string                   `json:"ipv4_pool"`
	IPv6Pool       string                   `json:"ipv6_pool,omitempty"`
	DNS            string                   `json:"dns,omitempty"`
	MTU            int32                    `json:"mtu,omitempty"`
	Status         DataNetworkStatus        `json:"status"`
	IPAllocation   *DataNetworkIPAllocation `json:"ip_allocation,omitempty"`
	IPv6Allocation *DataNetworkIPAllocation `json:"ipv6_allocation,omitempty"`
}

type IPAllocationItem struct {
	Address   string `json:"address"`
	IMSI      string `json:"imsi"`
	Type      string `json:"type"`
	SessionID *int   `json:"session_id"`
}

type ListIPAllocationsResponse struct {
	Items      []IPAllocationItem `json:"items"`
	Page       int                `json:"page"`
	PerPage    int                `json:"per_page"`
	TotalCount int                `json:"total_count"`
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

			items = append(items, DataNetwork{
				Name:     dbDataNetwork.Name,
				IPv4Pool: dbDataNetwork.IPv4Pool,
				IPv6Pool: dbDataNetwork.IPv6Pool,
				DNS:      dbDataNetwork.DNS,
				MTU:      dbDataNetwork.MTU,
				Status: DataNetworkStatus{
					Sessions: sessionCount,
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

		dataNetwork := DataNetwork{
			Name:     dbDataNetwork.Name,
			IPv4Pool: dbDataNetwork.IPv4Pool,
			IPv6Pool: dbDataNetwork.IPv6Pool,
			DNS:      dbDataNetwork.DNS,
			MTU:      dbDataNetwork.MTU,
			Status: DataNetworkStatus{
				Sessions: sessionCount,
			},
		}

		pool, poolErr := ipam.NewPool(dbDataNetwork.ID, dbDataNetwork.IPv4Pool)
		if poolErr != nil {
			logger.APILog.Warn("failed to parse IP pool for allocation stats", zap.String("data_network", name), zap.Error(poolErr))
		} else {
			allocated, countErr := dbInstance.CountIPv4LeasesByPool(r.Context(), dbDataNetwork.ID, pool.IPVersion)
			if countErr != nil {
				logger.APILog.Warn("failed to count IPv4 leases for allocation stats", zap.String("data_network", name), zap.Error(countErr))
			} else {
				poolSize := pool.Size()

				available := poolSize - allocated
				if available < 0 {
					available = 0
				}

				dataNetwork.IPAllocation = &DataNetworkIPAllocation{
					PoolSize:  poolSize,
					Allocated: allocated,
					Available: available,
				}
			}
		}

		if dbDataNetwork.IPv6Pool != "" {
			pool6, pool6Err := ipam.NewPool6(dbDataNetwork.ID, dbDataNetwork.IPv6Pool, 64)
			if pool6Err != nil {
				logger.APILog.Warn("failed to parse IPv6 pool for allocation stats", zap.String("data_network", name), zap.Error(pool6Err))
			} else {
				allocated6, countErr := dbInstance.CountIPv6LeasesByPool(r.Context(), dbDataNetwork.ID, pool6.IPVersion)
				if countErr != nil {
					logger.APILog.Warn("failed to count IPv6 leases for allocation stats", zap.String("data_network", name), zap.Error(countErr))
				} else {
					poolSize6 := pool6.Size()

					available6 := poolSize6 - allocated6
					if available6 < 0 {
						available6 = 0
					}

					dataNetwork.IPv6Allocation = &DataNetworkIPAllocation{
						PoolSize:  poolSize6,
						Allocated: allocated6,
						Available: available6,
					}
				}
			}
		}

		writeResponse(r.Context(), w, dataNetwork, http.StatusOK, logger.APILog)
	})
}

func ListIPv4Allocations(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

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

		dbDataNetwork, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		leases, total, err := dbInstance.ListLeasesByPoolPage(r.Context(), dbDataNetwork.ID, "ipv4", page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list IPv4 allocations", err, logger.APILog)
			return
		}

		items := make([]IPAllocationItem, 0, len(leases))
		for _, lease := range leases {
			items = append(items, IPAllocationItem{
				Address:   lease.Address().String(),
				IMSI:      lease.IMSI,
				Type:      lease.Type,
				SessionID: lease.SessionID,
			})
		}

		writeResponse(r.Context(), w, ListIPAllocationsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}, http.StatusOK, logger.APILog)
	})
}

func ListIPv6Allocations(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

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

		dbDataNetwork, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		if dbDataNetwork.IPv6Pool == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Data network has no IPv6 pool", nil, logger.APILog)
			return
		}

		leases, total, err := dbInstance.ListLeasesByPoolPage(r.Context(), dbDataNetwork.ID, "ipv6", page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list IPv6 IP allocations", err, logger.APILog)
			return
		}

		items := make([]IPAllocationItem, 0, len(leases))
		for _, lease := range leases {
			items = append(items, IPAllocationItem{
				Address:   lease.Address().String(),
				IMSI:      lease.IMSI,
				Type:      lease.Type,
				SessionID: lease.SessionID,
			})
		}

		writeResponse(r.Context(), w, ListIPAllocationsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}, http.StatusOK, logger.APILog)
	})
}

func DeleteDataNetwork(dbInstance *db.Database) http.Handler {
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

		writeResponse(r.Context(), w, SuccessResponse{Message: "DataNetwork deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteDataNetworkAction, email, getClientIP(r), "User deleted data network: "+name)
	})
}

func CreateDataNetwork(dbInstance *db.Database) http.Handler {
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

		if createDataNetworkParams.IPv4Pool != "" {
			if err := validateNoOverlap(r.Context(), dbInstance, createDataNetworkParams.IPv4Pool, ""); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
				return
			}
		}

		if createDataNetworkParams.IPv6Pool != "" {
			if err := validateNoIPv6Overlap(r.Context(), dbInstance, createDataNetworkParams.IPv6Pool, ""); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
				return
			}
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
			Name:     createDataNetworkParams.Name,
			IPv4Pool: createDataNetworkParams.IPv4Pool,
			IPv6Pool: createDataNetworkParams.IPv6Pool,
			DNS:      createDataNetworkParams.DNS,
			MTU:      createDataNetworkParams.MTU,
		}

		if err := dbInstance.CreateDataNetwork(r.Context(), dbDataNetwork); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "Data Network already exists", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create data network", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Data Network created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateDataNetworkAction, email, getClientIP(r), "User created data network: "+createDataNetworkParams.Name)
	})
}

func UpdateDataNetwork(dbInstance *db.Database) http.Handler {
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

		if updateDataNetworkParams.IPv4Pool != "" {
			if err := validateNoOverlap(r.Context(), dbInstance, updateDataNetworkParams.IPv4Pool, name); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
				return
			}
		}

		if updateDataNetworkParams.IPv6Pool != "" {
			if err := validateNoIPv6Overlap(r.Context(), dbInstance, updateDataNetworkParams.IPv6Pool, name); err != nil {
				writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
				return
			}
		}

		dn := &db.DataNetwork{
			Name:     name,
			IPv4Pool: updateDataNetworkParams.IPv4Pool,
			IPv6Pool: updateDataNetworkParams.IPv6Pool,
			DNS:      updateDataNetworkParams.DNS,
			MTU:      updateDataNetworkParams.MTU,
		}

		if err := dbInstance.UpdateDataNetwork(r.Context(), dn); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update data network", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Data Network updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateDataNetworkAction, email, getClientIP(r), "User updated data network: "+name)
	})
}

func isDataNetworkNameValid(name string) bool {
	return dnnRegex.MatchString(name)
}

func isUeIPPoolValid(ueIPPool string) bool {
	_, err := netip.ParsePrefix(ueIPPool)
	return err == nil
}

// isIPv6PoolValid validates an IPv6 prefix delegation pool CIDR.
// The prefix must be valid IPv6 with a prefix length between /48 and /60
// (we delegate /64s from within the pool).
func isIPv6PoolValid(pool string) bool {
	prefix, err := netip.ParsePrefix(pool)
	if err != nil {
		return false
	}

	if !prefix.Addr().Is6() || prefix.Addr().Is4In6() {
		return false
	}

	if prefix.Bits() < 48 || prefix.Bits() > 60 {
		return false
	}

	return true
}

func isValidDNS(dns string) bool {
	_, err := netip.ParseAddr(dns)
	return err == nil
}

func isValidMTU(mtu int32) bool {
	return mtu >= 0 && mtu <= 65535
}

func validateDataNetworkParams(p CreateDataNetworkParams) error {
	switch {
	case p.Name == "":
		return errors.New("name is missing")
	case p.IPv4Pool == "" && p.IPv6Pool == "":
		return errors.New("at least one IP pool (IPv4 or IPv6) is required")
	case p.DNS == "":
		return errors.New("dns is missing")
	case p.MTU == 0:
		return errors.New("mtu is missing")

	case !isDataNetworkNameValid(p.Name):
		return errors.New("invalid name format, must be a valid DNN format")
	case p.IPv4Pool != "" && !isUeIPPoolValid(p.IPv4Pool):
		return errors.New("invalid ipv4_pool format, must be in CIDR format")
	case p.IPv6Pool != "" && !isIPv6PoolValid(p.IPv6Pool):
		return errors.New("invalid ipv6_pool format, must be a valid IPv6 CIDR with prefix length between /48 and /60")
	case !isValidDNS(p.DNS):
		return errors.New("invalid dns format, must be a valid IP address")
	case !isValidMTU(p.MTU):
		return errors.New("invalid mtu format, must be an integer between 0 and 65535")
	}

	return nil
}

func validateUpdateDataNetworkParams(p UpdateDataNetworkParams) error {
	switch {
	case p.IPv4Pool == "" && p.IPv6Pool == "":
		return errors.New("at least one IP pool (IPv4 or IPv6) is required")
	case p.DNS == "":
		return errors.New("dns is missing")
	case p.MTU == 0:
		return errors.New("mtu is missing")
	case p.IPv4Pool != "" && !isUeIPPoolValid(p.IPv4Pool):
		return errors.New("invalid ipv4_pool format, must be in CIDR format")
	case p.IPv6Pool != "" && !isIPv6PoolValid(p.IPv6Pool):
		return errors.New("invalid ipv6_pool format, must be a valid IPv6 CIDR with prefix length between /48 and /60")
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

		if dn.IPv4Pool == "" {
			continue
		}

		existingPrefix, parseErr := netip.ParsePrefix(dn.IPv4Pool)
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

// validateNoIPv6Overlap checks that cidr does not overlap with any existing data
// network IPv6 pool. excludeName is the name of the data network being updated
// (empty for create) — its own pool is excluded from the comparison.
func validateNoIPv6Overlap(ctx context.Context, dbInstance *db.Database, cidr string, excludeName string) error {
	newPrefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("invalid IPv6 CIDR %q: %w", cidr, err)
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

		if dn.IPv6Pool == "" {
			continue
		}

		existingPrefix, parseErr := netip.ParsePrefix(dn.IPv6Pool)
		if parseErr != nil {
			continue
		}

		existingPrefix = netip.PrefixFrom(existingPrefix.Masked().Addr(), existingPrefix.Bits())

		if newPrefix.Overlaps(existingPrefix) {
			return fmt.Errorf("IPv6 pool %s overlaps with data network %q (%s)", newPrefix, dn.Name, existingPrefix)
		}
	}

	return nil
}

// CollectUEPools returns the UE IP pool CIDRs from all data networks.
func CollectUEPools(ctx context.Context, dbInstance *db.Database) []netip.Prefix {
	dataNetworks, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		logger.APILog.Warn("failed to list data networks for BGP filter rebuild")

		return nil
	}

	var pools []netip.Prefix

	for _, dn := range dataNetworks {
		prefix, err := netip.ParsePrefix(dn.IPv4Pool)
		if err != nil {
			continue
		}

		pools = append(pools, prefix)
	}

	return pools
}
