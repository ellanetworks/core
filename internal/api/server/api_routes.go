package server

import (
	"net"
	"net/http"
	"regexp"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/routes"
	"github.com/gin-gonic/gin"
)

type CreateRouteParams struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type GetRouteResponse struct {
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

// isRouteInterfaceValid uses a simple regex to validate interface names.
// This regex allows alphanumeric characters, dashes, and underscores.
var interfaceRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func isRouteInterfaceValid(iface string) bool {
	return interfaceRegex.MatchString(iface)
}

func ListRoutes(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		dbRoutes, err := dbInstance.ListRoutes()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Routes not found")
			return
		}
		routeList := make([]GetRouteResponse, 0)
		for _, dbRoute := range dbRoutes {
			routeResponse := GetRouteResponse{
				Destination: dbRoute.Destination,
				Gateway:     dbRoute.Gateway,
				Interface:   dbRoute.Interface,
				Metric:      dbRoute.Metric,
			}
			routeList = append(routeList, routeResponse)
		}
		if err := writeResponse(c.Writer, routeList, http.StatusOK); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal error")
			return
		}
		logger.LogAuditEvent(ListRoutesAction, email, "User listed routes")
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
		routeDestination, exists := c.Params.Get("destination")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "Missing destination parameter")
			return
		}
		dbRoute, err := dbInstance.GetRoute(routeDestination)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Route not found")
			return
		}

		routeResponse := GetRouteResponse{
			Destination: dbRoute.Destination,
			Gateway:     dbRoute.Gateway,
			Interface:   dbRoute.Interface,
			Metric:      dbRoute.Metric,
		}
		if err := writeResponse(c.Writer, routeResponse, http.StatusOK); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal error")
			return
		}
		logger.LogAuditEvent(GetRouteAction, email, "User retrieved route: "+routeDestination)
	}
}

func CreateRoute(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		var createRouteParams CreateRouteParams
		if err := c.ShouldBindJSON(&createRouteParams); err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid request data")
			return
		}
		if createRouteParams.Destination == "" {
			writeError(c.Writer, http.StatusBadRequest, "destination is missing")
			return
		}
		if createRouteParams.Gateway == "" {
			writeError(c.Writer, http.StatusBadRequest, "gateway is missing")
			return
		}
		if createRouteParams.Interface == "" {
			writeError(c.Writer, http.StatusBadRequest, "interface is missing")
			return
		}
		if !isRouteDestinationValid(createRouteParams.Destination) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid destination format")
			return
		}
		if !isRouteGatewayValid(createRouteParams.Gateway) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid gateway format")
			return
		}
		if !isRouteInterfaceValid(createRouteParams.Interface) {
			writeError(c.Writer, http.StatusBadRequest, "Invalid interface format")
			return
		}
		if createRouteParams.Metric < 0 {
			writeError(c.Writer, http.StatusBadRequest, "Invalid metric value")
			return
		}

		// Ensure the route doesn't already exist.
		if _, err := dbInstance.GetRoute(createRouteParams.Destination); err == nil {
			writeError(c.Writer, http.StatusBadRequest, "Route already exists")
			return
		}
		_, ipNetwork, err := net.ParseCIDR(createRouteParams.Destination)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid destination format")
			return
		}
		ipGateway := net.ParseIP(createRouteParams.Gateway)
		if ipGateway == nil || ipGateway.To4() == nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid gateway format; expecting an IPv4 address")
			return
		}
		ipGateway = ipGateway.To4()

		dbRoute := &db.Route{
			Destination: createRouteParams.Destination,
			Gateway:     createRouteParams.Gateway,
			Interface:   createRouteParams.Interface,
			Metric:      createRouteParams.Metric,
		}
		tx, err := dbInstance.BeginTransaction()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal error starting transaction")
			return
		}
		committed := false
		defer func() {
			if !committed {
				if rbErr := tx.Rollback(); rbErr != nil {
					logger.APILog.Errorf("Failed to rollback transaction: %v", rbErr)
				}
			}
		}()

		if err := tx.CreateRoute(dbRoute); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create route in DB")
			return
		}

		if err := routes.CreateKernelRoute(ipNetwork, ipGateway, createRouteParams.Metric, createRouteParams.Interface); err != nil {
			logger.APILog.Errorf("Kernel route creation failed: %v", err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to create kernel route")
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to commit transaction")
			return
		}
		committed = true

		response := SuccessResponse{Message: "Route created successfully"}
		if err := writeResponse(c.Writer, response, http.StatusCreated); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal error")
			return
		}
		logger.LogAuditEvent(CreateRouteAction, email, "User created route: "+createRouteParams.Destination)
	}
}

func DeleteRoute(dbInstance *db.Database) gin.HandlerFunc {
	return func(c *gin.Context) {
		emailAny, _ := c.Get("email")
		email, ok := emailAny.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get email"})
			return
		}
		routeDestination, exists := c.Params.Get("destination")
		if !exists {
			writeError(c.Writer, http.StatusBadRequest, "Missing destination parameter")
			return
		}
		route, err := dbInstance.GetRoute(routeDestination)
		if err != nil {
			writeError(c.Writer, http.StatusNotFound, "Route not found")
			return
		}
		_, ipNetwork, err := net.ParseCIDR(route.Destination)
		if err != nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid destination format")
			return
		}
		gateway := net.ParseIP(route.Gateway)
		if gateway == nil || gateway.To4() == nil {
			writeError(c.Writer, http.StatusBadRequest, "Invalid gateway format; expecting an IPv4 address")
			return
		}
		gateway = gateway.To4()

		// Begin a transaction to ensure DB deletion is tied to kernel route deletion.
		tx, err := dbInstance.BeginTransaction()
		if err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal error starting transaction")
			return
		}
		committed := false
		defer func() {
			if !committed {
				if rbErr := tx.Rollback(); rbErr != nil {
					logger.APILog.Errorf("Failed to rollback transaction: %v", rbErr)
				}
			}
		}()

		// Delete the DB record within the transaction.
		if err := tx.DeleteRoute(routeDestination); err != nil {
			logger.APILog.Errorf("DB route deletion failed: %v", err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to delete route from DB")
			return
		}

		// Delete the kernel route.
		if err := routes.DeleteKernelRoute(ipNetwork, gateway, route.Metric, route.Interface); err != nil {
			logger.APILog.Errorf("Kernel route deletion failed: %v", err)
			writeError(c.Writer, http.StatusInternalServerError, "Failed to delete kernel route")
			return
		}

		if err := tx.Commit(); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Failed to commit transaction")
			return
		}
		committed = true

		response := SuccessResponse{Message: "Route deleted successfully"}
		if err := writeResponse(c.Writer, response, http.StatusOK); err != nil {
			writeError(c.Writer, http.StatusInternalServerError, "Internal error")
			return
		}
		logger.LogAuditEvent(DeleteRouteAction, email, "User deleted route: "+routeDestination)
	}
}
