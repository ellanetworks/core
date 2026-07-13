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
	"sort"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

const (
	CreateFramedRouteAction = "create_framed_route"
	UpdateFramedRouteAction = "update_framed_route"
	DeleteFramedRouteAction = "delete_framed_route"
)

// MaxFramedRoutesPerFamily bounds the framed prefixes a subscriber may hold per
// family on one data network. Each prefix is a UPF LPM entry, so the cap also
// bounds datapath map growth.
const MaxFramedRoutesPerFamily = 8

type FramedRoute struct {
	IMSI string   `json:"imsi"`
	IPv4 []string `json:"ipv4,omitempty"`
	IPv6 []string `json:"ipv6,omitempty"`
}

type FramedRouteList struct {
	Items      []FramedRoute `json:"items"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalCount int           `json:"total_count"`
}

type CreateFramedRouteParams struct {
	IMSI string   `json:"imsi"`
	IPv4 []string `json:"ipv4,omitempty"`
	IPv6 []string `json:"ipv6,omitempty"`
}

type UpdateFramedRouteParams struct {
	IPv4 []string `json:"ipv4,omitempty"`
	IPv6 []string `json:"ipv6,omitempty"`
}

func isIPv4Prefix(p netip.Prefix) bool { return p.Addr().Is4() }

func isIPv6Prefix(p netip.Prefix) bool { return p.Addr().Is6() && !p.Addr().Is4In6() }

// parseFramedPrefixes validates and normalizes the request's CIDRs: correct
// family per list, at most MaxFramedRoutesPerFamily per family, no duplicate
// across the request, and each masked to its network form. It returns the
// combined set.
func parseFramedPrefixes(ipv4, ipv6 []string) ([]netip.Prefix, error) {
	if len(ipv4) > MaxFramedRoutesPerFamily {
		return nil, fmt.Errorf("at most %d IPv4 framed routes are allowed", MaxFramedRoutesPerFamily)
	}

	if len(ipv6) > MaxFramedRoutesPerFamily {
		return nil, fmt.Errorf("at most %d IPv6 framed routes are allowed", MaxFramedRoutesPerFamily)
	}

	out := make([]netip.Prefix, 0, len(ipv4)+len(ipv6))

	add := func(raw string, wantV4 bool) error {
		p, err := netip.ParsePrefix(raw)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q", raw)
		}

		p = p.Masked()

		if wantV4 && !isIPv4Prefix(p) {
			return fmt.Errorf("%q is not an IPv4 CIDR", raw)
		}

		if !wantV4 && !isIPv6Prefix(p) {
			return fmt.Errorf("%q is not an IPv6 CIDR", raw)
		}

		// Reject duplicates and overlaps within the request, matching the
		// cross-subscriber non-overlap invariant (Overlaps also catches exact
		// equality).
		for _, existing := range out {
			if p.Overlaps(existing) {
				return fmt.Errorf("framed route %s overlaps %s in the same request", p, existing)
			}
		}

		out = append(out, p)

		return nil
	}

	for _, raw := range ipv4 {
		if err := add(raw, true); err != nil {
			return nil, err
		}
	}

	for _, raw := range ipv6 {
		if err := add(raw, false); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// validateRoutableFramedPrefix rejects prefixes that must never be advertised as
// framed routes: the default route and the BGP safety-reject set (link-local,
// multicast, loopback).
func validateRoutableFramedPrefix(p netip.Prefix) error {
	if p.Bits() == 0 {
		return fmt.Errorf("prefix %s is a default route", p)
	}

	for _, reserved := range bgp.BuildRejectPrefixes(nil) {
		if p.Overlaps(reserved) {
			return fmt.Errorf("prefix %s overlaps the reserved range %s", p, reserved)
		}
	}

	return nil
}

// occupiedPrefixes collects every prefix a new framed route must not overlap:
// framed routes on other (subscriber, data network) pairs, every data network's
// UE pool, and every global route. The pair (excludeIMSI, excludeDNID) is
// omitted so a replace does not conflict with the set it replaces.
func occupiedPrefixes(ctx context.Context, dbInstance *db.Database, excludeIMSI, excludeDNID string) ([]netip.Prefix, error) {
	var occupied []netip.Prefix

	framed, err := dbInstance.ListAllFramedRoutes(ctx)
	if err != nil {
		return nil, err
	}

	for i := range framed {
		if framed[i].Imsi == excludeIMSI && framed[i].DataNetworkID == excludeDNID {
			continue
		}

		p, err := netip.ParsePrefix(framed[i].Prefix)
		if err != nil {
			continue
		}

		occupied = append(occupied, p)
	}

	dns, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		return nil, err
	}

	for i := range dns {
		for _, pool := range []string{dns[i].IPv4Pool, dns[i].IPv6Pool} {
			if pool == "" {
				continue
			}

			if p, err := netip.ParsePrefix(pool); err == nil {
				occupied = append(occupied, p.Masked())
			}
		}
	}

	routes, err := dbInstance.ListAllRoutes(ctx)
	if err != nil {
		return nil, err
	}

	for i := range routes {
		if p, err := netip.ParsePrefix(routes[i].Destination); err == nil {
			occupied = append(occupied, p.Masked())
		}
	}

	return occupied, nil
}

// firstOverlap returns the first (new, existing) prefix pair that overlaps, or
// ok=false. netip.Prefix.Overlaps is family-aware: prefixes of different
// families never overlap.
func firstOverlap(candidates, occupied []netip.Prefix) (netip.Prefix, netip.Prefix, bool) {
	for _, c := range candidates {
		for _, o := range occupied {
			if c.Overlaps(o) {
				return c, o, true
			}
		}
	}

	return netip.Prefix{}, netip.Prefix{}, false
}

// validateFramedRouteSet runs the shared create/update validation: family/cap,
// guardrails, and the global non-overlap invariant. It returns the parsed set
// and a typed HTTP status/message on failure.
func validateFramedRouteSet(ctx context.Context, dbInstance *db.Database, imsi, dnID string, ipv4, ipv6 []string) ([]netip.Prefix, int, string) {
	prefixes, err := parseFramedPrefixes(ipv4, ipv6)
	if err != nil {
		return nil, http.StatusBadRequest, err.Error()
	}

	for _, p := range prefixes {
		if err := validateRoutableFramedPrefix(p); err != nil {
			return nil, http.StatusBadRequest, err.Error()
		}
	}

	occupied, err := occupiedPrefixes(ctx, dbInstance, imsi, dnID)
	if err != nil {
		return nil, http.StatusInternalServerError, "Failed to resolve existing prefixes"
	}

	if c, o, ok := firstOverlap(prefixes, occupied); ok {
		return nil, http.StatusConflict, fmt.Sprintf("framed route %s overlaps %s", c, o)
	}

	return prefixes, 0, ""
}

func framedRoutesToItems(routes []db.SubscriberFramedRoute) []FramedRoute {
	byIMSI := map[string]*FramedRoute{}
	order := []string{}

	for i := range routes {
		fr, ok := byIMSI[routes[i].Imsi]
		if !ok {
			fr = &FramedRoute{IMSI: routes[i].Imsi}
			byIMSI[routes[i].Imsi] = fr
			order = append(order, routes[i].Imsi)
		}

		p, err := netip.ParsePrefix(routes[i].Prefix)
		if err != nil {
			continue
		}

		if isIPv4Prefix(p) {
			fr.IPv4 = append(fr.IPv4, routes[i].Prefix)
		} else {
			fr.IPv6 = append(fr.IPv6, routes[i].Prefix)
		}
	}

	sort.Strings(order)

	items := make([]FramedRoute, 0, len(order))
	for _, imsi := range order {
		items = append(items, *byIMSI[imsi])
	}

	return items
}

// framedRouteConflict reports the first existing framed route that prefix
// overlaps, for the reverse-direction guard: a UE pool or a global route must
// not overlap a framed route already in place. Empty string means no conflict.
func framedRouteConflict(ctx context.Context, dbInstance *db.Database, prefix netip.Prefix) (string, error) {
	framed, err := dbInstance.ListAllFramedRoutes(ctx)
	if err != nil {
		return "", err
	}

	for i := range framed {
		fp, err := netip.ParsePrefix(framed[i].Prefix)
		if err != nil {
			continue
		}

		if prefix.Overlaps(fp) {
			return fmt.Sprintf("%s (subscriber %s)", fp, framed[i].Imsi), nil
		}
	}

	return "", nil
}

func ListDataNetworkFramedRoutes(dbInstance *db.Database) http.Handler {
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

		routes, err := dbInstance.ListFramedRoutesByDataNetwork(r.Context(), dn.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list framed routes", err, logger.APILog)
			return
		}

		items := framedRoutesToItems(routes)

		writeResponse(r.Context(), w, FramedRouteList{
			Items:      items,
			Page:       1,
			PerPage:    len(items),
			TotalCount: len(items),
		}, http.StatusOK, logger.APILog)
	})
}

func CreateDataNetworkFramedRoute(dbInstance *db.Database) http.Handler {
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

		var params CreateFramedRouteParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.IMSI == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "imsi is required", nil, logger.APILog)
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

		if status, msg := framedRoutePreconditions(r.Context(), dbInstance, sub.ProfileID, dn.ID); status != 0 {
			writeError(r.Context(), w, status, msg, nil, logger.APILog)
			return
		}

		existing, err := dbInstance.ListFramedRoutesBySubscriberDataNetwork(r.Context(), params.IMSI, dn.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check existing framed routes", err, logger.APILog)
			return
		}

		if len(existing) > 0 {
			writeError(r.Context(), w, http.StatusConflict, "framed routes already exist for this subscriber and data network", nil, logger.APILog)
			return
		}

		prefixes, status, msg := validateFramedRouteSet(r.Context(), dbInstance, params.IMSI, dn.ID, params.IPv4, params.IPv6)
		if status != 0 {
			writeError(r.Context(), w, status, msg, nil, logger.APILog)
			return
		}

		if err := dbInstance.ReplaceFramedRoutes(r.Context(), params.IMSI, dn.ID, prefixes); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "a framed route is already assigned to another subscriber", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create framed routes", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Framed routes created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateFramedRouteAction, email, getClientIP(r), fmt.Sprintf("User set %d framed route(s) for subscriber %s on data network %s", len(prefixes), params.IMSI, name))
	})
}

func UpdateDataNetworkFramedRoute(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := r.PathValue("name")
		imsi := r.PathValue("imsi")

		if name == "" || imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name or imsi parameter", nil, logger.APILog)
			return
		}

		var params UpdateFramedRouteParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		dn, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		sub, err := dbInstance.GetSubscriber(r.Context(), imsi)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "Subscriber not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get subscriber", err, logger.APILog)

			return
		}

		existing, err := dbInstance.ListFramedRoutesBySubscriberDataNetwork(r.Context(), imsi, dn.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get framed routes", err, logger.APILog)
			return
		}

		if len(existing) == 0 {
			writeError(r.Context(), w, http.StatusNotFound, "framed routes not found for this subscriber and data network", nil, logger.APILog)
			return
		}

		if status, msg := framedRoutePreconditions(r.Context(), dbInstance, sub.ProfileID, dn.ID); status != 0 {
			writeError(r.Context(), w, status, msg, nil, logger.APILog)
			return
		}

		prefixes, status, msg := validateFramedRouteSet(r.Context(), dbInstance, imsi, dn.ID, params.IPv4, params.IPv6)
		if status != 0 {
			writeError(r.Context(), w, status, msg, nil, logger.APILog)
			return
		}

		if err := dbInstance.ReplaceFramedRoutes(r.Context(), imsi, dn.ID, prefixes); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "a framed route is already assigned to another subscriber", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update framed routes", err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Framed routes updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateFramedRouteAction, email, getClientIP(r), fmt.Sprintf("User replaced framed routes for subscriber %s on data network %s (%d route(s))", imsi, name, len(prefixes)))
	})
}

func DeleteDataNetworkFramedRoute(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		name := r.PathValue("name")
		imsi := r.PathValue("imsi")

		if name == "" || imsi == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "Missing name or imsi parameter", nil, logger.APILog)
			return
		}

		dn, err := dbInstance.GetDataNetwork(r.Context(), name)
		if err != nil {
			writeError(r.Context(), w, http.StatusNotFound, "Data Network not found", nil, logger.APILog)
			return
		}

		existing, err := dbInstance.ListFramedRoutesBySubscriberDataNetwork(r.Context(), imsi, dn.ID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get framed routes", err, logger.APILog)
			return
		}

		if len(existing) == 0 {
			writeError(r.Context(), w, http.StatusNotFound, "framed routes not found for this subscriber and data network", nil, logger.APILog)
			return
		}

		if err := dbInstance.DeleteFramedRoutes(r.Context(), imsi, dn.ID); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete framed routes", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Framed routes deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteFramedRouteAction, email, getClientIP(r), fmt.Sprintf("User removed framed routes for subscriber %s on data network %s", imsi, name))
	})
}

// framedRoutePreconditions enforces the requirements shared by create and
// update: the subscriber's profile must bind the data network, and NAT must be
// disabled (framed routes have no function under NAT). Returns a non-zero HTTP
// status on failure.
func framedRoutePreconditions(ctx context.Context, dbInstance *db.Database, profileID, dnID string) (int, string) {
	bound, err := dataNetworkBoundToProfile(ctx, dbInstance, profileID, dnID)
	if err != nil {
		return http.StatusInternalServerError, "Failed to resolve subscriber policies"
	}

	if !bound {
		return http.StatusConflict, "data network is not bound to the subscriber's profile"
	}

	natEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return http.StatusInternalServerError, "Failed to read NAT settings"
	}

	if natEnabled {
		return http.StatusConflict, "framed routes cannot be configured while NAT is enabled"
	}

	return 0, ""
}
