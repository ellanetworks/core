package server

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/pprof"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

type UPFUpdater interface {
	ReloadNAT(natEnabled bool) error
	ReloadFlowAccounting(flowAccountingEnabled bool) error
	UpdateAdvertisedN3Address(net.IP)
}

func NewHandler(dbInstance *db.Database, cfg config.Config, upf UPFUpdater, kernel kernel.Kernel, jwtSecret *JWTSecret, secureCookie bool, embedFS fs.FS, sessions smf.SessionQuerier, amfInstance *amf.AMF, bgpService *bgp.BGPService, registerExtraRoutes func(mux *http.ServeMux)) http.Handler {
	mux := http.NewServeMux()

	// Status (Unauthenticated)
	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance).ServeHTTP)

	// OpenAPI Specification (Unauthenticated)
	mux.HandleFunc("GET /api/v1/openapi.yaml", OpenAPISpec().ServeHTTP)

	// Metrics (Unauthenticated)
	mux.HandleFunc("GET /api/v1/metrics", GetMetrics().ServeHTTP)

	// Pprof (Authenticated)
	registerAuthenticatedPprof(mux, jwtSecret, dbInstance)

	// Authentication
	loginLimiter := newIPRateLimiter(LoginRateLimit, LoginRateWindow)
	mux.HandleFunc("POST /api/v1/auth/login", Login(dbInstance, jwtSecret, secureCookie, loginLimiter).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/refresh", Refresh(dbInstance, jwtSecret, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/logout", Logout(dbInstance, secureCookie).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/lookup-token", LookupToken(dbInstance, jwtSecret).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/rotate-secret", Authenticate(jwtSecret, dbInstance, Authorize(PermRotateSecret, RotateSecret(dbInstance, jwtSecret))).ServeHTTP)

	// Initialization (Unauthenticated)
	mux.HandleFunc("POST /api/v1/init", Initialize(dbInstance, jwtSecret, secureCookie).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", Authenticate(jwtSecret, dbInstance, Authorize(PermReadMyUser, GetLoggedInUser(dbInstance))).ServeHTTP)
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
	mux.HandleFunc("GET /api/v1/users/{email}/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermListUserAPITokens, ListUserAPITokens(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users/{email}/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateUserAPIToken, CreateUserAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/{email}/api-tokens/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteUserAPIToken, DeleteUserAPIToken(dbInstance))).ServeHTTP)

	// Subscribers (Authenticated)
	mux.HandleFunc("GET /api/v1/subscribers", Authenticate(jwtSecret, dbInstance, Authorize(PermListSubscribers, ListSubscribers(dbInstance, amfInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/subscribers", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateSubscriber, CreateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateSubscriber, UpdateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadSubscriber, GetSubscriber(dbInstance, amfInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscribers/{imsi}/credentials", Authenticate(jwtSecret, dbInstance, Authorize(PermReadSubscriberCredentials, GetSubscriberCredentials(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscribers/{imsi}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteSubscriber, DeleteSubscriber(dbInstance, amfInstance))).ServeHTTP)

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

	// Network Rules (Authenticated)
	mux.HandleFunc("POST /api/v1/policies/{name}/rules", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateNetworkRule, CreateNetworkRuleForPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/policies/{name}/rules", Authenticate(jwtSecret, dbInstance, Authorize(PermListNetworkRules, ListNetworkRulesForPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/policies/{name}/rules/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadNetworkRule, GetNetworkRule(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/policies/{name}/rules/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateNetworkRule, UpdateNetworkRule(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/policies/{name}/rules/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteNetworkRule, DeleteNetworkRule(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/policies/{name}/rules/{id}/reorder", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateNetworkRule, ReorderNetworkRule(dbInstance))).ServeHTTP)

	// Operator (Authenticated)
	mux.HandleFunc("GET /api/v1/operator", Authenticate(jwtSecret, dbInstance, Authorize(PermReadOperator, GetOperator(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/slice", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorSlice, UpdateOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorTracking, UpdateOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorID, UpdateOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/home-network-keys/{id}/private-key", Authenticate(jwtSecret, dbInstance, Authorize(PermReadHomeNetworkPrivateKey, GetHomeNetworkKeyPrivateKey(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/operator/home-network-keys", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorHomeNetwork, CreateHomeNetworkKey(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/operator/home-network-keys/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorHomeNetwork, DeleteHomeNetworkKey(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/code", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorCode, UpdateOperatorCode(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/nas-security", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorNASSecurity, UpdateOperatorNASSecurity(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/spn", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorSPN, UpdateOperatorSPN(dbInstance))).ServeHTTP)

	// Deprecated: sub-resource GETs — use GET /api/v1/operator instead.
	// These endpoints will be removed in a future release.
	mux.HandleFunc("GET /api/v1/operator/slice", Authenticate(jwtSecret, dbInstance, Authorize(PermGetOperatorSlice, GetOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, Authorize(PermGetOperatorTracking, GetOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, Authorize(PermGetOperatorID, GetOperatorID(dbInstance))).ServeHTTP)

	// Data Networks (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, Authorize(PermListDataNetworks, ListDataNetworks(dbInstance, sessions))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateDataNetwork, CreateDataNetwork(dbInstance, cfg, bgpService))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateDataNetwork, UpdateDataNetwork(dbInstance, cfg, bgpService))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadDataNetwork, GetDataNetwork(dbInstance, sessions))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteDataNetwork, DeleteDataNetwork(dbInstance, cfg, bgpService))).ServeHTTP)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, Authorize(PermListRoutes, ListRoutes(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateRoute, CreateRoute(dbInstance, kernel))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadRoute, GetRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteRoute, DeleteRoute(dbInstance, kernel))).ServeHTTP)

	// NAT (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, Authorize(PermGetNATInfo, GetNATInfo(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateNATInfo, UpdateNATInfo(dbInstance, upf, bgpService))).ServeHTTP)

	// BGP (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/bgp", Authenticate(jwtSecret, dbInstance, Authorize(PermReadBGP, GetBGPSettings(dbInstance, bgpService, cfg))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/bgp", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateBGP, UpdateBGPSettings(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/bgp/peers", Authenticate(jwtSecret, dbInstance, Authorize(PermReadBGP, ListBGPPeers(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/bgp/peers", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateBGP, CreateBGPPeer(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/bgp/peers/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadBGP, GetBGPPeer(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/bgp/peers/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateBGP, UpdateBGPPeer(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/bgp/peers/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateBGP, DeleteBGPPeer(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/bgp/advertised-routes", Authenticate(jwtSecret, dbInstance, Authorize(PermReadBGP, GetBGPAdvertisedRoutes(bgpService))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/bgp/learned-routes", Authenticate(jwtSecret, dbInstance, Authorize(PermReadBGP, GetBGPLearnedRoutes(bgpService))).ServeHTTP)

	// Flow Accounting (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/flow-accounting", Authenticate(jwtSecret, dbInstance, Authorize(PermGetFlowAccountingInfo, GetFlowAccountingInfo(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/flow-accounting", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateFlowAccountingInfo, UpdateFlowAccountingInfo(dbInstance, upf))).ServeHTTP)

	// Interfaces (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/interfaces", Authenticate(jwtSecret, dbInstance, Authorize(PermListNetworkInterfaces, ListNetworkInterfaces(dbInstance, cfg))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/interfaces/n3", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateN3Interface, UpdateN3Interface(dbInstance, upf, cfg))).ServeHTTP)

	// Radios (Authenticated)
	mux.HandleFunc("GET /api/v1/ran/radios", Authenticate(jwtSecret, dbInstance, Authorize(PermListRadios, ListRadios(amfInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/radios/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadRadio, GetRadio(amfInstance))).ServeHTTP)

	// Radio Events (Authenticated)
	mux.HandleFunc("GET /api/v1/ran/events/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermGetRadioEventRetentionPolicy, GetRadioEventRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/ran/events/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermSetRadioEventRetentionPolicy, UpdateRadioEventRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/events", Authenticate(jwtSecret, dbInstance, Authorize(PermListRadioEvents, ListRadioEvents(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/ran/events", Authenticate(jwtSecret, dbInstance, Authorize(PermClearRadioEvents, ClearRadioEvents(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/ran/events/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermGetRadioEvent, GetRadioEvent(dbInstance))).ServeHTTP)

	// Flow Reports (Authenticated)
	mux.HandleFunc("GET /api/v1/flow-reports/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermGetFlowReportsRetentionPolicy, GetFlowReportsRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/flow-reports/retention", Authenticate(jwtSecret, dbInstance, Authorize(PermSetFlowReportsRetentionPolicy, UpdateFlowReportsRetentionPolicy(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/flow-reports/stats", Authenticate(jwtSecret, dbInstance, Authorize(PermListFlowReports, GetFlowReportStats(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/flow-reports", Authenticate(jwtSecret, dbInstance, Authorize(PermListFlowReports, ListFlowReports(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/flow-reports", Authenticate(jwtSecret, dbInstance, Authorize(PermClearFlowReports, ClearFlowReports(dbInstance))).ServeHTTP)

	// Backup and Restore (Authenticated)
	mux.HandleFunc("POST /api/v1/backup", Authenticate(jwtSecret, dbInstance, Authorize(PermBackup, Backup(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/restore", Authenticate(jwtSecret, dbInstance, Authorize(PermRestore, Restore(dbInstance))).ServeHTTP)

	// Support bundle generation (Authenticated)
	mux.HandleFunc("POST /api/v1/support-bundle", Authenticate(jwtSecret, dbInstance, Authorize(PermSupportBundle, SupportBundle(dbInstance))).ServeHTTP)

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

	handler = MaxBodySizeMiddleware(handler)
	handler = SecurityHeadersMiddleware(secureCookie, handler)
	handler = MetricsMiddleware(handler)

	if cfg.Telemetry.Enabled {
		handler = TracingMiddleware("ella-core/api", handler)
	}

	return handler
}

func registerAuthenticatedPprof(root *http.ServeMux, jwtSecret *JWTSecret, dbInstance *db.Database) {
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
