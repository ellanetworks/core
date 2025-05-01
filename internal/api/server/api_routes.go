package server

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/gin-gonic/gin"
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

func ListRoutes(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbRoutes, err := dbInstance.ListRoutes(c.Request.Context())
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Routes not found")
			return
		}
		routeList := make([]GetRouteResponse, 0)
		for _, dbRoute := range dbRoutes {
			routeResponse := GetRouteResponse{
				ID:          dbRoute.ID,
				Destination: dbRoute.Destination,
				Gateway:     dbRoute.Gateway,
				Interface:   dbRoute.Interface.String(),
				Metric:      dbRoute.Metric,
			}
			routeList = append(routeList, routeResponse)
		}
		writeResponse(c, routeList, http.StatusOK)
		logger.LogAuditEvent(
			ListRoutesAction,
			email,
			c.ClientIP(),
			"User listed routes",
		)
	}
}

func GetRoute(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		routeID, exists := c.Params.Get("id")
		if !exists {
			writeError(c, http.StatusBadRequest, "Missing id parameter")
			return
		}
		idNum, err := strconv.ParseInt(routeID, 10, 64)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid id format")
			return
		}
		dbRoute, err := dbInstance.GetRoute(idNum, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Route not found")
			return
		}

		routeResponse := GetRouteResponse{
			ID:          dbRoute.ID,
			Destination: dbRoute.Destination,
			Gateway:     dbRoute.Gateway,
			Interface:   dbRoute.Interface.String(),
			Metric:      dbRoute.Metric,
		}
		writeResponse(c, routeResponse, http.StatusOK)
		logger.LogAuditEvent(
			GetRouteAction,
			email,
			c.ClientIP(),
			"User retrieved route: "+routeID,
		)
	}
}

func CreateRoute(dbInstance *db.Database, kernelInt kernel.Kernel) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var createRouteParams CreateRouteParams
		if err := c.ShouldBindJSON(&createRouteParams); err != nil {
			writeError(c, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createRouteParams.Destination == "" {
			writeError(c, http.StatusBadRequest, "destination is missing")
			return
		}
		if createRouteParams.Gateway == "" {
			writeError(c, http.StatusBadRequest, "gateway is missing")
			return
		}
		if createRouteParams.Interface == "" {
			writeError(c, http.StatusBadRequest, "interface is missing")
			return
		}
		if !isRouteDestinationValid(createRouteParams.Destination) {
			writeError(c, http.StatusBadRequest, "invalid destination format: expecting CIDR notation")
			return
		}
		if !isRouteGatewayValid(createRouteParams.Gateway) {
			writeError(c, http.StatusBadRequest, "invalid gateway format: expecting an IPv4 address")
			return
		}
		if createRouteParams.Metric < 0 {
			writeError(c, http.StatusBadRequest, "Invalid metric value")
			return
		}
		kernelNetworkInterface, ok := interfaceKernelMap[createRouteParams.Interface]
		if !ok {
			writeError(c, http.StatusBadRequest, "invalid interface: abcdef: only n3 and n6 are allowed")
			return
		}

		_, ipNetwork, err := net.ParseCIDR(createRouteParams.Destination)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid destination format")
			return
		}
		ipGateway := net.ParseIP(createRouteParams.Gateway)
		if ipGateway == nil || ipGateway.To4() == nil {
			writeError(c, http.StatusBadRequest, "Invalid gateway format: expecting an IPv4 address")
			return
		}
		ipGateway = ipGateway.To4()

		routeExists, err := kernelInt.RouteExists(ipNetwork, ipGateway, createRouteParams.Metric, kernelNetworkInterface)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to check if route exists")
			return
		}
		if routeExists {
			writeError(c, http.StatusBadRequest, "Route already exists")
			return
		}

		dbNetworkInterface, ok := interfaceDBMap[createRouteParams.Interface]
		if !ok {
			writeError(c, http.StatusBadRequest, "invalid interface: abcdef: only n3 and n6 are allowed")
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
			writeError(c, http.StatusInternalServerError, "Internal error starting transaction")
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

		routeID, err := tx.CreateRoute(dbRoute, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to create route in DB")
			return
		}

		if err := kernelInt.CreateRoute(ipNetwork, ipGateway, createRouteParams.Metric, kernelNetworkInterface); err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to create kernel route: "+err.Error())
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to commit transaction")
			return
		}
		committed = true

		response := CreateSuccessResponse{Message: "Route created successfully", ID: routeID}
		writeResponse(c, response, http.StatusCreated)
		logger.LogAuditEvent(
			CreateRouteAction,
			email,
			c.ClientIP(),
			"User created route: "+fmt.Sprint(routeID),
		)
	}
}

func DeleteRoute(dbInstance *db.Database, kernelInt kernel.Kernel) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		routeID, exists := c.Params.Get("id")
		if !exists {
			writeError(c, http.StatusBadRequest, "Missing id parameter")
			return
		}
		routeIDNum, err := strconv.ParseInt(routeID, 10, 64)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid id format")
			return
		}
		route, err := dbInstance.GetRoute(routeIDNum, c.Request.Context())
		if err != nil {
			writeError(c, http.StatusNotFound, "Route not found")
			return
		}
		_, ipNetwork, err := net.ParseCIDR(route.Destination)
		if err != nil {
			writeError(c, http.StatusBadRequest, "Invalid destination format: expecting CIDR notation.")
			return
		}
		gateway := net.ParseIP(route.Gateway)
		if gateway == nil || gateway.To4() == nil {
			writeError(c, http.StatusBadRequest, "Invalid gateway format: expecting an IPv4 address")
			return
		}
		gateway = gateway.To4()

		// Begin a transaction to ensure DB deletion is tied to kernel route deletion.
		tx, err := dbInstance.BeginTransaction()
		if err != nil {
			writeError(c, http.StatusInternalServerError, "Internal error starting transaction")
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

		// Delete the DB record within the transaction.
		if err := tx.DeleteRoute(routeIDNum, c.Request.Context()); err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to delete route from DB")
			return
		}

		// Delete the kernel route.
		kernelNetwokrInterface, ok := interfaceKernelMap[route.Interface.String()]
		if !ok {
			writeError(c, http.StatusInternalServerError, "invalid interface: abcdef: only n3 and n6 are allowed")
			return
		}
		if err := kernelInt.DeleteRoute(ipNetwork, gateway, route.Metric, kernelNetwokrInterface); err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to delete kernel route")
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(c, http.StatusInternalServerError, "Failed to commit transaction")
			return
		}
		committed = true

		response := SuccessResponse{Message: "Route deleted successfully"}
		writeResponse(c, response, http.StatusOK)
		logger.LogAuditEvent(
			DeleteRouteAction,
			email,
			c.ClientIP(),
			"User deleted route: "+routeID,
		)
	}
}
