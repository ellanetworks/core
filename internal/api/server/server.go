package server

import (
	"io/fs"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func NewHandler(dbInstance *db.Database, kernel kernel.Kernel, jwtSecret []byte, tracingEnabled bool, embedFS fs.FS, registerExtraRoutes func(mux *http.ServeMux)) http.Handler {
	mux := http.NewServeMux()

	// Status (Unauthenticated)
	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance).ServeHTTP)

	// Metrics (Unauthenticated)
	mux.HandleFunc("GET /api/v1/metrics", GetMetrics().ServeHTTP)

	// Authentication
	mux.HandleFunc("POST /api/v1/auth/login", Login(dbInstance, jwtSecret).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/lookup-token", LookupToken(dbInstance, jwtSecret).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", Authenticate(jwtSecret, GetLoggedInUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/me/password", Authenticate(jwtSecret, RequirePermission(PermUpdateMyUserPassword, jwtSecret, UpdateMyUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/me/api-tokens", Authenticate(jwtSecret, RequirePermission(PermListMyAPITokens, jwtSecret, ListMyAPITokens(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users/me/api-tokens", Authenticate(jwtSecret, RequirePermission(PermCreateMyAPIToken, jwtSecret, CreateMyAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users", Authenticate(jwtSecret, RequirePermission(PermListUsers, jwtSecret, ListUsers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users", RequirePermissionOrFirstUser(PermCreateUser, dbInstance, jwtSecret, CreateUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}", Authenticate(jwtSecret, RequirePermission(PermUpdateUser, jwtSecret, UpdateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}/password", Authenticate(jwtSecret, RequirePermission(PermUpdateUserPassword, jwtSecret, UpdateUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/{email}", Authenticate(jwtSecret, RequirePermission(PermReadUser, jwtSecret, GetUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/{email}", Authenticate(jwtSecret, RequirePermission(PermDeleteUser, jwtSecret, DeleteUser(dbInstance))).ServeHTTP)

	// Subscribers (Authenticated)
	mux.HandleFunc("GET /api/v1/subscribers", Authenticate(jwtSecret, RequirePermission(PermListSubscribers, jwtSecret, ListSubscribers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/subscribers", Authenticate(jwtSecret, RequirePermission(PermCreateSubscriber, jwtSecret, CreateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscribers/", Authenticate(jwtSecret, RequirePermission(PermUpdateSubscriber, jwtSecret, UpdateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscribers/", Authenticate(jwtSecret, RequirePermission(PermReadSubscriber, jwtSecret, GetSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscribers/", Authenticate(jwtSecret, RequirePermission(PermDeleteSubscriber, jwtSecret, DeleteSubscriber(dbInstance))).ServeHTTP)

	// Policies (Authenticated)
	mux.HandleFunc("GET /api/v1/policies", Authenticate(jwtSecret, RequirePermission(PermListPolicies, jwtSecret, ListPolicies(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/policies", Authenticate(jwtSecret, RequirePermission(PermCreatePolicy, jwtSecret, CreatePolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/policies/", Authenticate(jwtSecret, RequirePermission(PermUpdatePolicy, jwtSecret, UpdatePolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/policies/", Authenticate(jwtSecret, RequirePermission(PermReadPolicy, jwtSecret, GetPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/policies/", Authenticate(jwtSecret, RequirePermission(PermDeletePolicy, jwtSecret, DeletePolicy(dbInstance))).ServeHTTP)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/routes", Authenticate(jwtSecret, RequirePermission(PermListRoutes, jwtSecret, ListRoutes(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/routes", Authenticate(jwtSecret, RequirePermission(PermCreateRoute, jwtSecret, CreateRoute(dbInstance, kernel))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/routes/{id}", Authenticate(jwtSecret, RequirePermission(PermReadRoute, jwtSecret, GetRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/routes/{id}", Authenticate(jwtSecret, RequirePermission(PermDeleteRoute, jwtSecret, DeleteRoute(dbInstance, kernel))).ServeHTTP)

	// Operator (Authenticated)
	mux.HandleFunc("GET /api/v1/operator", Authenticate(jwtSecret, RequirePermission(PermReadOperator, jwtSecret, GetOperator(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/slice", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorSlice, jwtSecret, UpdateOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/slice", Authenticate(jwtSecret, RequirePermission(PermGetOperatorSlice, jwtSecret, GetOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/tracking", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorTracking, jwtSecret, UpdateOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/tracking", Authenticate(jwtSecret, RequirePermission(PermGetOperatorTracking, jwtSecret, GetOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/id", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorID, jwtSecret, UpdateOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/id", Authenticate(jwtSecret, RequirePermission(PermGetOperatorID, jwtSecret, GetOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/code", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorCode, jwtSecret, UpdateOperatorCode(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/home-network", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorHomeNetwork, jwtSecret, UpdateOperatorHomeNetwork(dbInstance))).ServeHTTP)

	// Data Networks (Authenticated)
	mux.HandleFunc("GET /api/v1/data-networks", Authenticate(jwtSecret, RequirePermission(PermListDataNetworks, jwtSecret, ListDataNetworks(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/data-networks", Authenticate(jwtSecret, RequirePermission(PermCreateDataNetwork, jwtSecret, CreateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/data-networks/", Authenticate(jwtSecret, RequirePermission(PermUpdateDataNetwork, jwtSecret, UpdateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/data-networks/", Authenticate(jwtSecret, RequirePermission(PermReadDataNetwork, jwtSecret, GetDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/data-networks/", Authenticate(jwtSecret, RequirePermission(PermDeleteDataNetwork, jwtSecret, DeleteDataNetwork(dbInstance))).ServeHTTP)

	// Radios (Authenticated)
	mux.HandleFunc("GET /api/v1/radios", Authenticate(jwtSecret, RequirePermission(PermListRadios, jwtSecret, ListRadios())).ServeHTTP)
	mux.HandleFunc("GET /api/v1/radios/", Authenticate(jwtSecret, RequirePermission(PermReadRadio, jwtSecret, GetRadio())).ServeHTTP)

	// Backup and Restore (Authenticated)
	mux.HandleFunc("POST /api/v1/backup", Authenticate(jwtSecret, RequirePermission(PermBackup, jwtSecret, Backup(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/restore", Authenticate(jwtSecret, RequirePermission(PermRestore, jwtSecret, Restore(dbInstance))).ServeHTTP)

	// Audit Logs (Authenticated)
	mux.HandleFunc("GET /api/v1/logs/audit/retention", Authenticate(jwtSecret, RequirePermission(PermGetAuditLogRetentionPolicy, jwtSecret, GetAuditLogRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/logs/audit/retention", Authenticate(jwtSecret, RequirePermission(PermSetAuditLogRetentionPolicy, jwtSecret, UpdateAuditLogRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/logs/audit", Authenticate(jwtSecret, RequirePermission(PermListAuditLogs, jwtSecret, ListAuditLogs(dbInstance))).ServeHTTP)

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
	if tracingEnabled {
		handler = TracingMiddleware("ella-core/api", handler)
	}

	return handler
}
