package server

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ui"
	"go.uber.org/zap"
)

type Mode string

const (
	TestMode    Mode = "test"
	ReleaseMode Mode = "release"
)

func NewHandler(dbInstance *db.Database, kernel kernel.Kernel, jwtSecret []byte, mode Mode, tracingEnabled bool) http.Handler {
	mux := http.NewServeMux()

	// UI Service fallback (must be registered before "/" to be matched last)
	uiHandler := AddUIService()

	// Status (Unauthenticated)
	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance).ServeHTTP)

	// Metrics (Unauthenticated)
	mux.HandleFunc("GET /api/v1/metrics", GetMetrics().ServeHTTP)

	// Authentication
	mux.HandleFunc("POST /api/v1/auth/login", Login(dbInstance, jwtSecret).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/lookup-token", LookupToken(dbInstance, jwtSecret).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", Authenticate(jwtSecret, GetLoggedInUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users", Authenticate(jwtSecret, RequirePermission(PermListUsers, ListUsers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/users", RequirePermissionOrFirstUser(PermCreateUser, dbInstance, jwtSecret, CreateUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}", Authenticate(jwtSecret, RequirePermission(PermUpdateUser, UpdateUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/users/{email}/password", Authenticate(jwtSecret, RequirePermission(PermUpdateUserPassword, UpdateUserPassword(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users/{email}", Authenticate(jwtSecret, RequirePermission(PermReadUser, GetUser(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/users/{email}", Authenticate(jwtSecret, RequirePermission(PermDeleteUser, DeleteUser(dbInstance))).ServeHTTP)

	// Subscribers (Authenticated)
	mux.HandleFunc("GET /api/v1/subscribers", Authenticate(jwtSecret, RequirePermission(PermListSubscribers, ListSubscribers(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/subscribers", Authenticate(jwtSecret, RequirePermission(PermCreateSubscriber, CreateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/subscribers/", Authenticate(jwtSecret, RequirePermission(PermUpdateSubscriber, UpdateSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/subscribers/", Authenticate(jwtSecret, RequirePermission(PermReadSubscriber, GetSubscriber(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/subscribers/", Authenticate(jwtSecret, RequirePermission(PermDeleteSubscriber, DeleteSubscriber(dbInstance))).ServeHTTP)

	// Profiles (Authenticated)
	mux.HandleFunc("GET /api/v1/profiles", Authenticate(jwtSecret, RequirePermission(PermListProfiles, ListProfiles(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/profiles", Authenticate(jwtSecret, RequirePermission(PermCreateProfile, CreateProfile(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/profiles/", Authenticate(jwtSecret, RequirePermission(PermUpdateProfile, UpdateProfile(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/profiles/", Authenticate(jwtSecret, RequirePermission(PermReadProfile, GetProfile(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/profiles/", Authenticate(jwtSecret, RequirePermission(PermDeleteProfile, DeleteProfile(dbInstance))).ServeHTTP)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/routes", Authenticate(jwtSecret, RequirePermission(PermListRoutes, ListRoutes(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/routes", Authenticate(jwtSecret, RequirePermission(PermCreateRoute, CreateRoute(dbInstance, kernel))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/routes/{id}", Authenticate(jwtSecret, RequirePermission(PermReadRoute, GetRoute(dbInstance))).ServeHTTP)
	mux.HandleFunc("DELETE /api/v1/routes/{id}", Authenticate(jwtSecret, RequirePermission(PermDeleteRoute, DeleteRoute(dbInstance, kernel))).ServeHTTP)

	// Operator (Authenticated)
	mux.HandleFunc("GET /api/v1/operator", Authenticate(jwtSecret, RequirePermission(PermReadOperator, GetOperator(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/slice", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorSlice, UpdateOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/slice", Authenticate(jwtSecret, RequirePermission(PermGetOperatorSlice, GetOperatorSlice(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/tracking", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorTracking, UpdateOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/tracking", Authenticate(jwtSecret, RequirePermission(PermGetOperatorTracking, GetOperatorTracking(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/id", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorID, UpdateOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("GET /api/v1/operator/id", Authenticate(jwtSecret, RequirePermission(PermGetOperatorID, GetOperatorID(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/code", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorCode, UpdateOperatorCode(dbInstance))).ServeHTTP)
	mux.HandleFunc("PUT /api/v1/operator/home-network", Authenticate(jwtSecret, RequirePermission(PermUpdateOperatorHomeNetwork, UpdateOperatorHomeNetwork(dbInstance))).ServeHTTP)

	// Radios (Authenticated)
	mux.HandleFunc("GET /api/v1/radios", Authenticate(jwtSecret, RequirePermission(PermListRadios, ListRadios())).ServeHTTP)
	mux.HandleFunc("GET /api/v1/radios/", Authenticate(jwtSecret, RequirePermission(PermReadRadio, GetRadio())).ServeHTTP)

	// Backup and Restore (Authenticated)
	mux.HandleFunc("POST /api/v1/backup", Authenticate(jwtSecret, RequirePermission(PermBackup, Backup(dbInstance))).ServeHTTP)
	mux.HandleFunc("POST /api/v1/restore", Authenticate(jwtSecret, RequirePermission(PermRestore, Restore(dbInstance))).ServeHTTP)

	// Fallback to UI
	mux.Handle("/", uiHandler)

	// Wrap with optional tracing and rate limiting
	var handler http.Handler = mux
	if tracingEnabled {
		handler = TracingMiddlewareHTTP("ella-core/api", handler)
	}
	if mode != TestMode {
		handler = RateLimitMiddlewareHTTP(handler)
	}

	return handler
}

func AddUIService() http.Handler {
	staticFilesSystem, err := fs.Sub(ui.FrontendFS, "out")
	if err != nil {
		logger.APILog.Fatal("Failed to create static files system", zap.Error(err))
	}

	fileServer := http.FileServer(http.FS(staticFilesSystem))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip if API path
		if isAPIURLPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}

		// Check for direct file first
		f, err := staticFilesSystem.Open(strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Check if {path}.html exists
		htmlPath := strings.TrimPrefix(r.URL.Path, "/") + ".html"
		f, err = staticFilesSystem.Open(htmlPath)
		if err == nil {
			_ = f.Close()
			r.URL.Path = "/" + htmlPath
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallthrough: 404
		http.NotFound(w, r)
	})
}

func isAPIURLPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}
