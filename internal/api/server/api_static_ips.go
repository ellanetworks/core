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

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	CreateStaticIPAction = "create_static_ip"
	UpdateStaticIPAction = "update_static_ip"
	DeleteStaticIPAction = "delete_static_ip"
)

type StaticIPItem struct {
	IMSI        string `json:"imsi"`
	DataNetwork string `json:"data_network"`
	IPVersion   string `json:"ip_version"`
	Address     string `json:"address"`
	Status      string `json:"status"`
	SessionID   *int   `json:"session_id"`
}

type ListStaticIPsResponse struct {
	Items      []StaticIPItem `json:"items"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalCount int            `json:"total_count"`
}

type CreateStaticIPParams struct {
	IMSI    string `json:"imsi"`
	Address string `json:"address"`
}

type UpdateStaticIPParams struct {
	Address string `json:"address"`
}

// familyOfAddr maps a parsed address to its pool family ("ipv4"|"ipv6").
func familyOfAddr(addr netip.Addr) (string, bool) {
	addr = addr.Unmap()

	switch {
	case addr.Is4():
		return "ipv4", true
	case addr.Is6():
		return "ipv6", true
	default:
		return "", false
	}
}

// poolForFamily returns the data network's pool for the given family, or an
// error (surfaced as 400) when the network has no pool for it.
func poolForFamily(dn *db.DataNetwork, family string) (ipam.Pool, error) {
	if family == "ipv6" {
		if dn.IPv6Pool == "" {
			return ipam.Pool{}, errors.New("data network has no IPv6 pool")
		}

		return ipam.NewPool6(dn.ID, dn.IPv6Pool, 64)
	}

	if dn.IPv4Pool == "" {
		return ipam.Pool{}, errors.New("data network has no IPv4 pool")
	}

	return ipam.NewPool(dn.ID, dn.IPv4Pool)
}

// validateStaticAddress rejects addresses that are not /64-aligned (IPv6) or
// fall outside the pool's usable range.
func validateStaticAddress(pool ipam.Pool, addr netip.Addr, family string) error {
	if family == "ipv6" && netip.PrefixFrom(addr, 64).Masked().Addr() != addr {
		return errors.New("IPv6 address must be /64-aligned")
	}

	off := pool.OffsetOf(addr)
	if off < pool.FirstUsable() || off-pool.FirstUsable() >= pool.Size() {
		return errors.New("address is outside the data network pool")
	}

	return nil
}

// dataNetworkBoundToProfile reports whether any policy on the profile reaches
// the data network.
func dataNetworkBoundToProfile(ctx context.Context, dbInstance *db.Database, profileID, dnID string) (bool, error) {
	policies, err := dbInstance.ListPoliciesByProfile(ctx, profileID)
	if err != nil {
		return false, err
	}

	for _, p := range policies {
		if p.DataNetworkID == dnID {
			return true, nil
		}
	}

	return false, nil
}

func staticLeaseStatus(sessionID *int) string {
	if sessionID == nil {
		return "reserved"
	}

	return "active"
}

func ListDataNetworkStaticIps(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name parameter", nil, logger.APILog)
			return
		}

		dn, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		leases, err := dbInstance.ListStaticLeasesByDataNetwork(r.Context(), dn.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list static IPs", err, logger.APILog)
			return
		}

		items := make([]StaticIPItem, 0, len(leases))
		for i := range leases {
			items = append(items, StaticIPItem{
				IMSI:        leases[i].IMSI,
				DataNetwork: name,
				IPVersion:   leases[i].PoolType,
				Address:     leases[i].Address().String(),
				Status:      staticLeaseStatus(leases[i].SessionID),
				SessionID:   leases[i].SessionID,
			})
		}

		writeResponse(r.Context(), w, ListStaticIPsResponse{
			Items:      items,
			Page:       1,
			PerPage:    len(items),
			TotalCount: len(items),
		}, http.StatusOK, logger.APILog)
	})
}

func CreateDataNetworkStaticIp(dbInstance *db.Database) http.Handler {
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

		var params CreateStaticIPParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.IMSI == "" || params.Address == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "imsi and address are required", nil, logger.APILog)
			return
		}

		addr, err := netip.ParseAddr(params.Address)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid address format", nil, logger.APILog)
			return
		}

		addr = addr.Unmap()

		family, ok := familyOfAddr(addr)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid address format", nil, logger.APILog)
			return
		}

		dn, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		sub, err := dbInstance.GetSubscriber(r.Context(), params.IMSI)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get subscriber", err, logger.APILog)

			return
		}

		pool, err := poolForFamily(dn, family)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := validateStaticAddress(pool, addr, family); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		bound, err := dataNetworkBoundToProfile(r.Context(), dbInstance, sub.ProfileID, dn.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to resolve subscriber policies", err, logger.APILog)
			return
		}

		if !bound {
			writeError(r.Context(), w, http.StatusConflict, "data network is not bound to the subscriber's profile", nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetStaticLease(r.Context(), dn.ID, family, params.IMSI); err == nil {
			writeError(r.Context(), w, http.StatusConflict, "a static IP is already assigned for this data network and IP version", nil, logger.APILog)
			return
		} else if !errors.Is(err, db.ErrNotFound) {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check existing static IP", err, logger.APILog)
			return
		}

		if err := dbInstance.CreateStaticLease(r.Context(), params.IMSI, dn.ID, family, addr); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "address is already in use", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create static IP", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Static IP created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateStaticIPAction, email, getClientIP(r), fmt.Sprintf("User pinned %s to subscriber %s on data network %s", addr, params.IMSI, name))
	})
}

func UpdateDataNetworkStaticIp(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := r.PathValue("name")
		imsi := r.PathValue("imsi")
		ipVersion := r.PathValue("ip_version")

		if name == "" || imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name or imsi parameter", nil, logger.APILog)
			return
		}

		if ipVersion != "ipv4" && ipVersion != "ipv6" {
			writeError(r.Context(), w, http.StatusBadRequest, "ip_version must be ipv4 or ipv6", nil, logger.APILog)
			return
		}

		var params UpdateStaticIPParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Address == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "address is required", nil, logger.APILog)
			return
		}

		addr, err := netip.ParseAddr(params.Address)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid address format", nil, logger.APILog)
			return
		}

		addr = addr.Unmap()

		family, ok := familyOfAddr(addr)
		if !ok || family != ipVersion {
			writeError(r.Context(), w, http.StatusBadRequest, "address family does not match ip_version", nil, logger.APILog)
			return
		}

		dn, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		lease, err := dbInstance.GetStaticLease(r.Context(), dn.ID, ipVersion, imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "static IP reservation not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get static IP", err, logger.APILog)

			return
		}

		pool, err := poolForFamily(dn, family)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := validateStaticAddress(pool, addr, family); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateStaticLeaseAddress(r.Context(), lease.ID, addr); err != nil {
			switch {
			case errors.Is(err, db.ErrNotFound):
				writeError(r.Context(), w, http.StatusNotFound, "Static IP not found", nil, logger.APILog)
			case errors.Is(err, db.ErrAlreadyExists):
				writeError(r.Context(), w, http.StatusConflict, "address is already in use", nil, logger.APILog)
			default:
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update static IP", err, logger.APILog)
			}

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Static IP updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateStaticIPAction, email, getClientIP(r), fmt.Sprintf("User repinned subscriber %s (%s) to %s on data network %s", imsi, ipVersion, addr, name))
	})
}

func DeleteDataNetworkStaticIp(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := r.PathValue("name")
		imsi := r.PathValue("imsi")
		ipVersion := r.PathValue("ip_version")

		if name == "" || imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name or imsi parameter", nil, logger.APILog)
			return
		}

		if ipVersion != "ipv4" && ipVersion != "ipv6" {
			writeError(r.Context(), w, http.StatusBadRequest, "ip_version must be ipv4 or ipv6", nil, logger.APILog)
			return
		}

		dn, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		lease, err := dbInstance.GetStaticLease(r.Context(), dn.ID, ipVersion, imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "static IP reservation not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get static IP", err, logger.APILog)

			return
		}

		if err := dbInstance.DeleteStaticLease(r.Context(), lease.ID); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "static IP reservation not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete static IP", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Static IP deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteStaticIPAction, email, getClientIP(r), fmt.Sprintf("User removed static IP for subscriber %s (%s) on data network %s", imsi, ipVersion, name))
	})
}
