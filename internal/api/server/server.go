package server

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/pprof"

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

	// Pprof (Authenticated)
	registerAuthenticatedPprof(mux, jwtSecret, dbInstance)

	// Authentication
	mux.HandleFunc("POST /api/v1/auth/login", Login(dbInstance, jwtSecret, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/refresh", Refresh(dbInstance, jwtSecret, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/logout", Logout(dbInstance, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/lookup-token", LookupToken(dbInstance, jwtSecret).ServeHTTP)

	// Initialization (Unauthenticated)
	mux.HandleFunc("POST /api/v1/init", Initialize(dbInstance, jwtSecret, secureCookie).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", Authenticate(jwtSecret, dbInstance, GetLoggedInUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/me/password", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateMyUserPassword, UpdateMyUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/me/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermListMyAPITokens, ListMyAPITokens(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users/me/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateMyAPIToken, CreateMyAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/me/api-tokens/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteMyAPIToken, DeleteMyAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users", Authenticate(jwtSecret, dbInstance, Authorize(PermListUsers, ListUsers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateUser, CreateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateUser, UpdateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}/password", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateUserPassword, UpdateUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadUser, GetUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteUser, DeleteUser(dbInstance))).ServeHTTP)

	// Subscribers (Authenticated)
	mux.HandleFunc("GET /api/v1/subscribers", Authenticate(jwtSecret, dbInstance, Authorize(PermListSubscribers, ListSubscribers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/subscribers", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateSubscriber, CreateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateSubscriber, UpdateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadSubscriber, GetSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteSubscriber, DeleteSubscriber(dbInstance))).ServeHTTP)

	// Subscriber Usage (Authenticated)
	mux.HandleFunc("GET /api/v1/subscriber-usage/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermGetSubscriberUsageRetentionPolicy, GetSubscriberUsageRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscriber-usage/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermSetSubscriberUsageRetentionPolicy, UpdateSubscriberUsageRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscriber-usage", Authenticate(jwtSecret, dbInstance, Authorize(PermClearSubscriberUsage, ClearSubscriberUsage(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscriber-usage", Authenticate(jwtSecret, dbInstance, Authorize(PermGetSubscriberUsage, GetSubscriberUsage(dbInstance))).ServeHTTP)

	// Policies (Authenticated)
	mux.HandleFunc("GET /api/v1/policies", Authenticate(jwtSecret, dbInstance, Authorize(PermListPolicies, ListPolicies(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/policies", Authenticate(jwtSecret, dbInstance, Authorize(PermCreatePolicy, CreatePolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/policies/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdatePolicy, UpdatePolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/policies/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadPolicy, GetPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/policies/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeletePolicy, DeletePolicy(dbInstance))).ServeHTTP)

	// Operator (Authenticated)
	mux.HandleFunc("GET /api/v1/operator", Authenticate(jwtSecret, dbInstance, Authorize(PermReadOperator, GetOperator(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/slice", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorSlice, UpdateOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/slice", Authenticate(jwtSecret, dbInstance, Authorize(PermGetOperatorSlice, GetOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorTracking, UpdateOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, Authorize(PermGetOperatorTracking, GetOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorID, UpdateOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, Authorize(PermGetOperatorID, GetOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/code", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorCode, UpdateOperatorCode(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/home-network", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorHomeNetwork, UpdateOperatorHomeNetwork(dbInstance))).ServeHTTP)

	// Data Networks (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, Authorize(PermListDataNetworks, ListDataNetworks(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateDataNetwork, CreateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateDataNetwork, UpdateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadDataNetwork, GetDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteDataNetwork, DeleteDataNetwork(dbInstance))).ServeHTTP)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, Authorize(PermListRoutes, ListRoutes(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateRoute, CreateRoute(dbInstance, kernel))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadRoute, GetRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteRoute, DeleteRoute(dbInstance, kernel))).ServeHTTP)

	// NAT (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, Authorize(PermGetNATInfo, GetNATInfo(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateNATInfo, UpdateNATInfo(dbInstance, upf))).ServeHTTP)

	// Interfaces (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/interfaces", Authenticate(jwtSecret, dbInstance, Authorize(PermListNetworkInterfaces, ListNetworkInterfaces(dbInstance, cfg))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/interfaces/n3", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateN3Interface, UpdateN3Interface(dbInstance, upf, cfg))).ServeHTTP)

	// Radios (Authenticated)
	mux.HandleFunc("GET /api/v1/ran/radios", Authenticate(jwtSecret, dbInstance, Authorize(PermListRadios, ListRadios())).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/radios/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadRadio, GetRadio())).ServeHTTP)

	// Radio Events (Authenticated)
	mux.HandleFunc("GET /api/v1/ran/events/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermGetRadioEventRetentionPolicy, GetRadioEventRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/ran/events/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermSetRadioEventRetentionPolicy, UpdateRadioEventRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/events", Authenticate(jwtSecret, dbInstance, Authorize(PermListRadioEvents, ListRadioEvents(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/ran/events", Authenticate(jwtSecret, dbInstance, Authorize(PermClearRadioEvents, ClearRadioEvents(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/events/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermGetRadioEvent, GetRadioEvent(dbInstance))).ServeHTTP)

	// Backup and Restore (Authenticated)
	mux.HandleFunc("POST /api/v1/backup", Authenticate(jwtSecret, dbInstance, Authorize(PermBackup, Backup(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/restore", Authenticate(jwtSecret, dbInstance, Authorize(PermRestore, Restore(dbInstance))).ServeHTTP)

	// Audit Logs (Authenticated)
	mux.HandleFunc("GET /api/v1/logs/audit/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermGetAuditLogRetentionPolicy, GetAuditLogRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/logs/audit/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermSetAuditLogRetentionPolicy, UpdateAuditLogRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/logs/audit", Authenticate(jwtSecret, dbInstance, Authorize(PermListAuditLogs, ListAuditLogs(dbInstance))).ServeHTTP)

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

	handler = MetricsMiddleware(handler)

	if cfg.Telemetry.Enabled {
		handler = TracingMiddleware("ella-core/api", handler)
	}

	return handler
}

func registerAuthenticatedPprof(root *http.ServeMux, jwtSecret []byte, dbInstance *db.Database) {
	pp := http.NewServeMux()

	pp.HandleFunc("/api/v1/pprof/", pprof.Index)
	pp.HandleFunc("/api/v1/pprof/cmdline", pprof.Cmdline)
	pp.HandleFunc("/api/v1/pprof/profile", pprof.Profile)
	pp.HandleFunc("/api/v1/pprof/symbol", pprof.Symbol)
	pp.HandleFunc("/api/v1/pprof/trace", pprof.Trace)

	pp.Handle("/api/v1/pprof/allocs", pprof.Handler("allocs"))
	pp.Handle("/api/v1/pprof/block", pprof.Handler("block"))
	pp.Handle("/api/v1/pprof/goroutine", pprof.Handler("goroutine"))
	pp.Handle("/api/v1/pprof/heap", pprof.Handler("heap"))
	pp.Handle("/api/v1/pprof/mutex", pprof.Handler("mutex"))
	pp.Handle("/api/v1/pprof/threadcreate", pprof.Handler("threadcreate"))

	root.Handle("/api/v1/pprof/", Authenticate(jwtSecret, dbInstance, Authorize(PermPprof, pp)))
}
