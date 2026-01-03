package server

import (
	"io/fs"
	"net"
	"net/http"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type UPFUpdater interface {
	Reload(natEnabled bool) error
	UpdateAdvertisedN3Address(net.IP)
}

func NewHandler(dbInstance *db.Database, cfg config.Config, upf UPFUpdater, kernel kernel.Kernel, jwtSecret []byte, secureCookie bool, embedFS fs.FS, registerExtraRoutes func(mux *http.ServeMux)) http.Handler {
	mux := http.NewServeMux()

	// Status (Unauthenticated)
	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance).ServeHTTP)

	// Metrics (Unauthenticated)
	mux.HandleFunc("GET /api/v1/metrics", GetMetrics().ServeHTTP)

	// Authentication
	mux.HandleFunc("POST /api/v1/auth/login", Login(dbInstance, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/refresh", Refresh(dbInstance, jwtSecret).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/logout", Logout(dbInstance, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/lookup-token", LookupToken(dbInstance, jwtSecret).ServeHTTP)

	// Initialization (Unauthenticated)
	mux.HandleFunc("POST /api/v1/init", Initialize(dbInstance, secureCookie).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", Authenticate(jwtSecret, dbInstance, GetLoggedInUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/me/password", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateMyUserPassword, jwtSecret, UpdateMyUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/me/api-tokens", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListMyAPITokens, jwtSecret, ListMyAPITokens(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users/me/api-tokens", Authenticate(jwtSecret, dbInstance, RequirePermission(PermCreateMyAPIToken, jwtSecret, CreateMyAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/me/api-tokens/{id}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermDeleteMyAPIToken, jwtSecret, DeleteMyAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListUsers, jwtSecret, ListUsers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users", Authenticate(jwtSecret, dbInstance, RequirePermission(PermCreateUser, jwtSecret, CreateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateUser, jwtSecret, UpdateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}/password", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateUserPassword, jwtSecret, UpdateUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadUser, jwtSecret, GetUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermDeleteUser, jwtSecret, DeleteUser(dbInstance))).ServeHTTP)

	// Subscribers (Authenticated)
	mux.HandleFunc("GET /api/v1/subscribers", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListSubscribers, jwtSecret, ListSubscribers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/subscribers", Authenticate(jwtSecret, dbInstance, RequirePermission(PermCreateSubscriber, jwtSecret, CreateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateSubscriber, jwtSecret, UpdateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadSubscriber, jwtSecret, GetSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermDeleteSubscriber, jwtSecret, DeleteSubscriber(dbInstance))).ServeHTTP)

	// Subscriber Usage (Authenticated)
	mux.HandleFunc("GET /api/v1/subscriber-usage/retention", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetSubscriberUsageRetentionPolicy, jwtSecret, GetSubscriberUsageRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscriber-usage/retention", Authenticate(jwtSecret, dbInstance, RequirePermission(PermSetSubscriberUsageRetentionPolicy, jwtSecret, UpdateSubscriberUsageRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscriber-usage", Authenticate(jwtSecret, dbInstance, RequirePermission(PermClearSubscriberUsage, jwtSecret, ClearSubscriberUsage(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscriber-usage", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetSubscriberUsage, jwtSecret, GetSubscriberUsage(dbInstance))).ServeHTTP)

	// Policies (Authenticated)
	mux.HandleFunc("GET /api/v1/policies", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListPolicies, jwtSecret, ListPolicies(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/policies", Authenticate(jwtSecret, dbInstance, RequirePermission(PermCreatePolicy, jwtSecret, CreatePolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/policies/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdatePolicy, jwtSecret, UpdatePolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/policies/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadPolicy, jwtSecret, GetPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/policies/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermDeletePolicy, jwtSecret, DeletePolicy(dbInstance))).ServeHTTP)

	// Operator (Authenticated)
	mux.HandleFunc("GET /api/v1/operator", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadOperator, jwtSecret, GetOperator(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/slice", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateOperatorSlice, jwtSecret, UpdateOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/slice", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetOperatorSlice, jwtSecret, GetOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateOperatorTracking, jwtSecret, UpdateOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetOperatorTracking, jwtSecret, GetOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateOperatorID, jwtSecret, UpdateOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetOperatorID, jwtSecret, GetOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/code", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateOperatorCode, jwtSecret, UpdateOperatorCode(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/home-network", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateOperatorHomeNetwork, jwtSecret, UpdateOperatorHomeNetwork(dbInstance))).ServeHTTP)

	// Data Networks (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListDataNetworks, jwtSecret, ListDataNetworks(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, RequirePermission(PermCreateDataNetwork, jwtSecret, CreateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateDataNetwork, jwtSecret, UpdateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadDataNetwork, jwtSecret, GetDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermDeleteDataNetwork, jwtSecret, DeleteDataNetwork(dbInstance))).ServeHTTP)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListRoutes, jwtSecret, ListRoutes(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, RequirePermission(PermCreateRoute, jwtSecret, CreateRoute(dbInstance, kernel))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadRoute, jwtSecret, GetRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermDeleteRoute, jwtSecret, DeleteRoute(dbInstance, kernel))).ServeHTTP)

	// NAT (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetNATInfo, jwtSecret, GetNATInfo(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateNATInfo, jwtSecret, UpdateNATInfo(dbInstance, upf))).ServeHTTP)

	// Interfaces (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/interfaces", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListNetworkInterfaces, jwtSecret, ListNetworkInterfaces(dbInstance, cfg))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/interfaces/n3", Authenticate(jwtSecret, dbInstance, RequirePermission(PermUpdateN3Interface, jwtSecret, UpdateN3Interface(dbInstance, upf, cfg))).ServeHTTP)

	// Radios (Authenticated)
	mux.HandleFunc("GET /api/v1/ran/radios", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListRadios, jwtSecret, ListRadios())).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/radios/{name}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermReadRadio, jwtSecret, GetRadio())).ServeHTTP)

	// Radio Events (Authenticated)
	mux.HandleFunc("GET /api/v1/ran/events/retention", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetRadioEventRetentionPolicy, jwtSecret, GetRadioEventRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/ran/events/retention", Authenticate(jwtSecret, dbInstance, RequirePermission(PermSetRadioEventRetentionPolicy, jwtSecret, UpdateRadioEventRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/events", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListRadioEvents, jwtSecret, ListRadioEvents(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/ran/events", Authenticate(jwtSecret, dbInstance, RequirePermission(PermClearRadioEvents, jwtSecret, ClearRadioEvents(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/events/{id}", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetRadioEvent, jwtSecret, GetRadioEvent(dbInstance))).ServeHTTP)

	// Backup and Restore (Authenticated)
	mux.HandleFunc("POST /api/v1/backup", Authenticate(jwtSecret, dbInstance, RequirePermission(PermBackup, jwtSecret, Backup(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/restore", Authenticate(jwtSecret, dbInstance, RequirePermission(PermRestore, jwtSecret, Restore(dbInstance))).ServeHTTP)

	// Audit Logs (Authenticated)
	mux.HandleFunc("GET /api/v1/logs/audit/retention", Authenticate(jwtSecret, dbInstance, RequirePermission(PermGetAuditLogRetentionPolicy, jwtSecret, GetAuditLogRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/logs/audit/retention", Authenticate(jwtSecret, dbInstance, RequirePermission(PermSetAuditLogRetentionPolicy, jwtSecret, UpdateAuditLogRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/logs/audit", Authenticate(jwtSecret, dbInstance, RequirePermission(PermListAuditLogs, jwtSecret, ListAuditLogs(dbInstance))).ServeHTTP)

	// Fallback to UI
	frontendHandler, err := newFrontendFileServer(embedFS)
	if err != nil {
		logger.APILog.Fatal("Failed to create frontend file server", zap.Error(err))
		return nil
	}

	mux.Handle("/", frontendHandler)

	if registerExtraRoutes != nil {
		registerExtraRoutes(mux)
	}

	var handler http.Handler = mux
	if cfg.Telemetry.Enabled {
		handler = TracingMiddleware("ella-core/api", handler)
	}

	return handler
}
