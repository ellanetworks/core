package server

import (
	"io/fs"
	"net/http"
	"net/http/pprof"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

type HandlerConfig struct {
	DB                  *db.Database
	Config              config.Config
	JWTSecret           *JWTSecret
	SecureCookie        bool
	FrontendFS          fs.FS
	Sessions            smf.SessionQuerier
	AMF                 *amf.AMF
	BGP                 *bgp.BGPService
	BcryptCost          int
	Ready               *atomic.Bool
	RegisterExtraRoutes func(*http.ServeMux)
	ClusterListener     *listener.Listener
}

func NewHandler(cfg HandlerConfig) http.Handler {
	dbInstance := cfg.DB
	appCfg := cfg.Config
	jwtSecret := cfg.JWTSecret
	secureCookie := cfg.SecureCookie
	embedFS := cfg.FrontendFS
	sessions := cfg.Sessions
	amfInstance := cfg.AMF
	bgpService := cfg.BGP
	bcryptCost := cfg.BcryptCost
	registerExtraRoutes := cfg.RegisterExtraRoutes

	mux := http.NewServeMux()

	// Status (Unauthenticated)
	ready := cfg.Ready
	if ready == nil {
		ready = &atomic.Bool{}
		ready.Store(true)
	}

	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance, ready).ServeHTTP)

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
	mux.HandleFunc("POST /api/v1/init", Initialize(dbInstance, jwtSecret, secureCookie, bcryptCost).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", Authenticate(jwtSecret, dbInstance, Authorize(PermReadMyUser, GetLoggedInUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/me/password", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateMyUserPassword, UpdateMyUserPassword(dbInstance, bcryptCost))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/me/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermListMyAPITokens, ListMyAPITokens(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users/me/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateMyAPIToken, CreateMyAPIToken(dbInstance, bcryptCost))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/me/api-tokens/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteMyAPIToken, DeleteMyAPIToken(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users", Authenticate(jwtSecret, dbInstance, Authorize(PermListUsers, ListUsers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateUser, CreateUser(dbInstance, bcryptCost))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateUser, UpdateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}/password", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateUserPassword, UpdateUserPassword(dbInstance, bcryptCost))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadUser, GetUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/{email}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteUser, DeleteUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/{email}/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermListUserAPITokens, ListUserAPITokens(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users/{email}/api-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateUserAPIToken, CreateUserAPIToken(dbInstance, bcryptCost))).ServeHTTP)
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

	// Profiles (Authenticated)
	mux.HandleFunc("GET /api/v1/profiles", Authenticate(jwtSecret, dbInstance, Authorize(PermListProfiles, ListProfiles(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/profiles", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateProfile, CreateProfile(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/profiles/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateProfile, UpdateProfile(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/profiles/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadProfile, GetProfile(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/profiles/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteProfile, DeleteProfile(dbInstance))).ServeHTTP)

	// Slices (Authenticated)
	mux.HandleFunc("GET /api/v1/slices", Authenticate(jwtSecret, dbInstance, Authorize(PermListSlices, ListSlices(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/slices", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateSlice, CreateSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/slices/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateSlice, UpdateSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/slices/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadSlice, GetSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/slices/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteSlice, DeleteSlice(dbInstance))).ServeHTTP)

	// Operator (Authenticated)
	mux.HandleFunc("GET /api/v1/operator", Authenticate(jwtSecret, dbInstance, Authorize(PermReadOperator, GetOperator(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/tracking", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorTracking, UpdateOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/id", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorID, UpdateOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/home-network-keys/{id}/private-key", Authenticate(jwtSecret, dbInstance, Authorize(PermReadHomeNetworkPrivateKey, GetHomeNetworkKeyPrivateKey(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/operator/home-network-keys", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorHomeNetwork, CreateHomeNetworkKey(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/operator/home-network-keys/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorHomeNetwork, DeleteHomeNetworkKey(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/code", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorCode, UpdateOperatorCode(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/nas-security", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorNASSecurity, UpdateOperatorNASSecurity(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/spn", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateOperatorSPN, UpdateOperatorSPN(dbInstance))).ServeHTTP)

	// Data Networks (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, Authorize(PermListDataNetworks, ListDataNetworks(dbInstance, sessions))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/data-networks", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateDataNetwork, CreateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateDataNetwork, UpdateDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadDataNetwork, GetDataNetwork(dbInstance, sessions))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/data-networks/{name}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteDataNetwork, DeleteDataNetwork(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/data-networks/{name}/ip-allocations", Authenticate(jwtSecret, dbInstance, Authorize(PermReadDataNetwork, ListIPAllocations(dbInstance))).ServeHTTP)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, Authorize(PermListRoutes, ListRoutes(dbInstance, bgpService))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/networking/routes", Authenticate(jwtSecret, dbInstance, Authorize(PermCreateRoute, CreateRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermReadRoute, GetRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/networking/routes/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermDeleteRoute, DeleteRoute(dbInstance))).ServeHTTP)

	// NAT (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, Authorize(PermGetNATInfo, GetNATInfo(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/nat", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateNATInfo, UpdateNATInfo(dbInstance))).ServeHTTP)

	// BGP (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/bgp", Authenticate(jwtSecret, dbInstance, Authorize(PermReadBGP, GetBGPSettings(dbInstance, bgpService, appCfg))).ServeHTTP)
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
	mux.HandleFunc("PUT /api/v1/networking/flow-accounting", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateFlowAccountingInfo, UpdateFlowAccountingInfo(dbInstance))).ServeHTTP)

	// Interfaces (Authenticated)
	mux.HandleFunc("GET /api/v1/networking/interfaces", Authenticate(jwtSecret, dbInstance, Authorize(PermListNetworkInterfaces, ListNetworkInterfaces(dbInstance, appCfg))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/networking/interfaces/n3", Authenticate(jwtSecret, dbInstance, Authorize(PermUpdateN3Interface, UpdateN3Interface(dbInstance))).ServeHTTP)

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

	// Cluster (Authenticated, admin only)
	mux.HandleFunc("GET /api/v1/cluster/members", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, ListClusterMembers(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/cluster/members/{id}", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, RemoveClusterMember(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/cluster/members/{id}/promote", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, PromoteClusterMember(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/cluster/members/{id}/drain", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, DrainClusterMember(dbInstance, amfInstance, bgpService, cfg.ClusterListener))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/cluster/members/{id}/resume", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, ResumeClusterMember(dbInstance, bgpService, cfg.ClusterListener))).ServeHTTP)

	mux.HandleFunc("GET /api/v1/cluster/autopilot", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, GetAutopilotState(dbInstance))).ServeHTTP)

	// PKI admin endpoints. Handlers resolve the issuer service at
	// request time (set by runtime after first-leader bootstrap), so
	// these routes can be registered before the issuer is ready.
	mux.HandleFunc("POST /api/v1/cluster/pki/join-tokens", Authenticate(jwtSecret, dbInstance, Authorize(PermManageCluster, pkiAdminEndpoint(func(svc *pkiissuer.Service) http.Handler {
		return PKIMintJoinToken(dbInstance, svc)
	}))).ServeHTTP)

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

	if appCfg.Telemetry.Enabled {
		handler = TracingMiddleware("ella-core/api", handler)
	}

	return handler
}

// DiscoveryHandlerConfig holds the dependencies for the discovery-phase
// HTTP handler that runs before cluster formation.
type DiscoveryHandlerConfig struct {
	DB     *db.Database
	Config config.Config
	Ready  *atomic.Bool
}

// NewDiscoveryHandler returns an HTTP handler serving only the routes
// needed for cluster discovery: status, cluster membership, metrics,
// and the OpenAPI spec. It requires no JWT secret or NF instances.
func NewDiscoveryHandler(cfg DiscoveryHandlerConfig) http.Handler {
	dbInstance := cfg.DB
	appCfg := cfg.Config
	secureCookie := appCfg.Interfaces.API.TLS.Cert != "" && appCfg.Interfaces.API.TLS.Key != ""

	mux := http.NewServeMux()

	ready := cfg.Ready
	if ready == nil {
		ready = &atomic.Bool{}
		ready.Store(true)
	}

	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance, ready).ServeHTTP)
	mux.HandleFunc("GET /api/v1/metrics", GetMetrics().ServeHTTP)
	mux.HandleFunc("GET /api/v1/openapi.yaml", OpenAPISpec().ServeHTTP)

	// POST /api/v1/cluster/members is not served during discovery; in mTLS
	// mode, join requests arrive on the cluster port (wired in a later step).

	var handler http.Handler = mux

	handler = MaxBodySizeMiddleware(handler)
	handler = SecurityHeadersMiddleware(secureCookie, handler)
	handler = MetricsMiddleware(handler)

	if appCfg.Telemetry.Enabled {
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
