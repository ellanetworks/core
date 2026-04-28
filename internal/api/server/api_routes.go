package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type CreateRouteParams struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type Route struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
	Source      string `json:"source"`
}

type ListRoutesResponse struct {
	Items      []Route `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

const (
	CreateRouteAction = "create_route"
	DeleteRouteAction = "delete_route"
)

const (
	MaxNumRoutes = 12
)

// isRouteDestinationValid checks if the destination is in valid CIDR notation.
func isRouteDestinationValid(dest string) bool {
	_, err := netip.ParsePrefix(dest)
	return err == nil
}

// isRouteGatewayValid checks if the gateway is a valid IP address.
func isRouteGatewayValid(gateway string) bool {
	addr, err := netip.ParseAddr(gateway)
	return err == nil && addr.Is4()
}

// interfaceDBMap maps the interface string to the db.NetworkInterface enum.
var interfaceDBMap = map[string]db.NetworkInterface{
	"n3": db.N3,
	"n6": db.N6,
}

func ListRoutes(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
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

		dbRoutes, total, err := dbInstance.ListRoutesPage(r.Context(), page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Routes not found", err, logger.APILog)
			return
		}

		var learned []bgp.LearnedRoute
		if bgpService != nil && bgpService.IsRunning() {
			learned = bgpService.GetLearnedRoutes()
		}

		items := make([]Route, 0, len(dbRoutes)+len(learned))

		for _, lr := range learned {
			items = append(items, Route{
				Destination: lr.Prefix,
				Gateway:     lr.NextHop,
				Interface:   "n6",
				Metric:      200,
				Source:      "bgp",
			})
		}

		for _, dbRoute := range dbRoutes {
			items = append(items, Route{
				ID:          dbRoute.ID,
				Destination: dbRoute.Destination,
				Gateway:     dbRoute.Gateway,
				Interface:   dbRoute.Interface.String(),
				Metric:      dbRoute.Metric,
				Source:      "static",
			})
		}

		resp := ListRoutesResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total + len(learned),
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetRoute(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")

		idNum, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid id format", err, logger.APILog)
			return
		}

		dbRoute, err := dbInstance.GetRoute(r.Context(), idNum)
		if err != nil {
			if err == db.ErrNotFound {
				writeError(r.Context(), w, http.StatusNotFound, "Route not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get route", err, logger.APILog)

			return
		}

		routeResponse := Route{
			ID:          dbRoute.ID,
			Destination: dbRoute.Destination,
			Gateway:     dbRoute.Gateway,
			Interface:   dbRoute.Interface.String(),
			Metric:      dbRoute.Metric,
			Source:      "static",
		}

		writeResponse(r.Context(), w, routeResponse, http.StatusOK, logger.APILog)
	})
}

func CreateRoute(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var createRouteParams CreateRouteParams
		if err := json.NewDecoder(r.Body).Decode(&createRouteParams); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if createRouteParams.Destination == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "destination is missing", nil, logger.APILog)
			return
		}

		if createRouteParams.Gateway == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "gateway is missing", nil, logger.APILog)
			return
		}

		if createRouteParams.Interface == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "interface is missing", nil, logger.APILog)
			return
		}

		if !isRouteDestinationValid(createRouteParams.Destination) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid destination format: expecting CIDR notation", nil, logger.APILog)
			return
		}

		if !isRouteGatewayValid(createRouteParams.Gateway) {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid gateway format: expecting an IPv4 address", nil, logger.APILog)
			return
		}

		if createRouteParams.Metric < 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid metric value", nil, logger.APILog)
			return
		}

		dbNetworkInterface, ok := interfaceDBMap[createRouteParams.Interface]
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid interface: only n3 and n6 are allowed", nil, logger.APILog)
			return
		}

		// Hard cap on total static routes; bounds the cost of the
		// reconciler's DB read on every tick.
		existing, _, err := dbInstance.ListRoutesPage(r.Context(), 1, MaxNumRoutes+1)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list routes", err, logger.APILog)
			return
		}

		if len(existing) >= MaxNumRoutes {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of routes reached ("+strconv.Itoa(MaxNumRoutes)+")", nil, logger.APILog)
			return
		}

		for _, existingRoute := range existing {
			if existingRoute.Destination == createRouteParams.Destination &&
				existingRoute.Gateway == createRouteParams.Gateway &&
				existingRoute.Metric == createRouteParams.Metric &&
				existingRoute.Interface == dbNetworkInterface {
				writeError(r.Context(), w, http.StatusBadRequest, "Route already exists", nil, logger.APILog)
				return
			}
		}

		dbRoute := &db.Route{
			Destination: createRouteParams.Destination,
			Gateway:     createRouteParams.Gateway,
			Interface:   dbNetworkInterface,
			Metric:      createRouteParams.Metric,
		}

		routeID, err := dbInstance.CreateRoute(r.Context(), dbRoute)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create route in DB", err, logger.APILog)
			return
		}

		response := CreateSuccessResponse{Message: "Route created successfully", ID: routeID}
		writeResponse(r.Context(), w, response, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(r.Context(), CreateRouteAction, email, getClientIP(r), "User created route: "+fmt.Sprint(routeID))
	})
}

func DeleteRoute(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value(contextKeyEmail)

		email, ok := emailAny.(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		routeIDStr := r.PathValue("id")

		routeID, err := strconv.ParseInt(routeIDStr, 10, 64)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid id format", err, logger.APILog)
			return
		}

		if _, err := dbInstance.GetRoute(r.Context(), routeID); err != nil {
			if err == db.ErrNotFound {
				writeError(r.Context(), w, http.StatusNotFound, "Route not found", nil, logger.APILog)
				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get route", err, logger.APILog)

			return
		}

		if err := dbInstance.DeleteRoute(r.Context(), routeID); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete route from DB", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Route deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			DeleteRouteAction,
			email,
			getClientIP(r),
			"User deleted route: "+routeIDStr,
		)
	})
}
