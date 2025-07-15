package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type CreateRouteParams struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type GetRouteResponse struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

const (
	ListRoutesAction  = "list_routes"
	GetRouteAction    = "get_route"
	CreateRouteAction = "create_route"
	DeleteRouteAction = "delete_route"
)

// isRouteDestinationValid checks if the destination is in valid CIDR notation.
func isRouteDestinationValid(dest string) bool {
	_, _, err := net.ParseCIDR(dest)
	return err == nil
}

// isRouteGatewayValid checks if the gateway is a valid IP address.
func isRouteGatewayValid(gateway string) bool {
	ip := net.ParseIP(gateway)
	return ip != nil
}

// interfaceDBMap maps the interface string to the db.NetworkInterface enum.
var interfaceDBMap = map[string]db.NetworkInterface{
	"n3": db.N3,
	"n6": db.N6,
}

// interfaceKernelMap maps the interface string to the kernel.NetworkInterface enum.
var interfaceKernelMap = map[string]kernel.NetworkInterface{
	"n3": kernel.N3,
	"n6": kernel.N6,
}

func ListRoutes(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		dbRoutes, err := dbInstance.ListRoutes(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Routes not found", err, logger.APILog)
			return
		}

		routeList := make([]GetRouteResponse, 0)
		for _, dbRoute := range dbRoutes {
			routeList = append(routeList, GetRouteResponse{
				ID:          dbRoute.ID,
				Destination: dbRoute.Destination,
				Gateway:     dbRoute.Gateway,
				Interface:   dbRoute.Interface.String(),
				Metric:      dbRoute.Metric,
			})
		}
		writeResponse(w, routeList, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(ListRoutesAction, email, getClientIP(r), "User listed routes")
	})
}

func GetRoute(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/routes/")
		idNum, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid id format", err, logger.APILog)
			return
		}

		dbRoute, err := dbInstance.GetRoute(r.Context(), idNum)
		if err != nil {
			writeError(w, http.StatusNotFound, "Route not found", err, logger.APILog)
			return
		}

		routeResponse := GetRouteResponse{
			ID:          dbRoute.ID,
			Destination: dbRoute.Destination,
			Gateway:     dbRoute.Gateway,
			Interface:   dbRoute.Interface.String(),
			Metric:      dbRoute.Metric,
		}
		writeResponse(w, routeResponse, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(GetRouteAction, email, getClientIP(r), "User retrieved route: "+idStr)
	})
}

func CreateRoute(dbInstance *db.Database, kernelInt kernel.Kernel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var createRouteParams CreateRouteParams
		if err := json.NewDecoder(r.Body).Decode(&createRouteParams); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if createRouteParams.Destination == "" {
			writeError(w, http.StatusBadRequest, "destination is missing", nil, logger.APILog)
			return
		}
		if createRouteParams.Gateway == "" {
			writeError(w, http.StatusBadRequest, "gateway is missing", nil, logger.APILog)
			return
		}
		if createRouteParams.Interface == "" {
			writeError(w, http.StatusBadRequest, "interface is missing", nil, logger.APILog)
			return
		}
		if !isRouteDestinationValid(createRouteParams.Destination) {
			writeError(w, http.StatusBadRequest, "invalid destination format: expecting CIDR notation", nil, logger.APILog)
			return
		}
		if !isRouteGatewayValid(createRouteParams.Gateway) {
			writeError(w, http.StatusBadRequest, "invalid gateway format: expecting an IPv4 address", nil, logger.APILog)
			return
		}
		if createRouteParams.Metric < 0 {
			writeError(w, http.StatusBadRequest, "Invalid metric value", nil, logger.APILog)
			return
		}

		kernelNetworkInterface, ok := interfaceKernelMap[createRouteParams.Interface]
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid interface: abcdef: only n3 and n6 are allowed", nil, logger.APILog)
			return
		}

		_, ipNetwork, err := net.ParseCIDR(createRouteParams.Destination)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid destination format", err, logger.APILog)
			return
		}

		ipGateway := net.ParseIP(createRouteParams.Gateway)
		if ipGateway == nil || ipGateway.To4() == nil {
			writeError(w, http.StatusBadRequest, "Invalid gateway format: expecting an IPv4 address", nil, logger.APILog)
			return
		}
		ipGateway = ipGateway.To4()

		routeExists, err := kernelInt.RouteExists(ipNetwork, ipGateway, createRouteParams.Metric, kernelNetworkInterface)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to check if route exists", err, logger.APILog)
			return
		}
		if routeExists {
			writeError(w, http.StatusBadRequest, "Route already exists", nil, logger.APILog)
			return
		}

		dbNetworkInterface, ok := interfaceDBMap[createRouteParams.Interface]
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid interface: abcdef: only n3 and n6 are allowed", nil, logger.APILog)
			return
		}

		dbRoute := &db.Route{
			Destination: createRouteParams.Destination,
			Gateway:     createRouteParams.Gateway,
			Interface:   dbNetworkInterface,
			Metric:      createRouteParams.Metric,
		}

		tx, err := dbInstance.BeginTransaction()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal error starting transaction", err, logger.APILog)
			return
		}
		committed := false
		defer func() {
			if !committed {
				if rbErr := tx.Rollback(); rbErr != nil {
					logger.APILog.Error("Failed to rollback transaction", zap.Error(rbErr))
				}
			}
		}()

		routeID, err := tx.CreateRoute(r.Context(), dbRoute)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create route in DB", err, logger.APILog)
			return
		}

		if err := kernelInt.CreateRoute(ipNetwork, ipGateway, createRouteParams.Metric, kernelNetworkInterface); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to create kernel route: "+err.Error(), nil, logger.APILog)
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to commit transaction", err, logger.APILog)
			return
		}
		committed = true

		response := CreateSuccessResponse{Message: "Route created successfully", ID: routeID}
		writeResponse(w, response, http.StatusCreated, logger.APILog)
		logger.LogAuditEvent(CreateRouteAction, email, getClientIP(r), "User created route: "+fmt.Sprint(routeID))
	})
}

func DeleteRoute(dbInstance *db.Database, kernelInt kernel.Kernel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailAny := r.Context().Value("email")
		email, ok := emailAny.(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		routeIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/routes/")
		routeID, err := strconv.ParseInt(routeIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid id format", err, logger.APILog)
			return
		}

		route, err := dbInstance.GetRoute(r.Context(), routeID)
		if err != nil {
			writeError(w, http.StatusNotFound, "Route not found", err, logger.APILog)
			return
		}

		_, ipNetwork, err := net.ParseCIDR(route.Destination)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid destination format: expecting CIDR notation.", err, logger.APILog)
			return
		}

		gateway := net.ParseIP(route.Gateway)
		if gateway == nil || gateway.To4() == nil {
			writeError(w, http.StatusBadRequest, "Invalid gateway format: expecting an IPv4 address", nil, logger.APILog)
			return
		}
		gateway = gateway.To4()

		tx, err := dbInstance.BeginTransaction()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Internal error starting transaction", err, logger.APILog)
			return
		}
		committed := false
		defer func() {
			if !committed {
				if rbErr := tx.Rollback(); rbErr != nil {
					logger.APILog.Error("Failed to rollback transaction", zap.Error(rbErr))
				}
			}
		}()

		if err := tx.DeleteRoute(r.Context(), routeID); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to delete route from DB", err, logger.APILog)
			return
		}

		kernelInterface, ok := interfaceKernelMap[route.Interface.String()]
		if !ok {
			writeError(w, http.StatusInternalServerError, "invalid interface: abcdef: only n3 and n6 are allowed", nil, logger.APILog)
			return
		}

		if err := kernelInt.DeleteRoute(ipNetwork, gateway, route.Metric, kernelInterface); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to delete kernel route", err, logger.APILog)
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to commit transaction", err, logger.APILog)
			return
		}
		committed = true

		writeResponse(w, SuccessResponse{Message: "Route deleted successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(
			DeleteRouteAction,
			email,
			getClientIP(r),
			"User deleted route: "+routeIDStr,
		)
	})
}
