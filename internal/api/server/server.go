package server

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ui"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ginToZap(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Record start time
		startTime := time.Now()

		// Skip logging for static files and other unwanted paths
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/_next/static") ||
			strings.HasPrefix(path, "/favicon.ico") ||
			strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}

		raw := c.Request.URL.RawQuery
		c.Next()

		latency := time.Since(startTime)
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.String()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Info("handled API request", zap.Int("statusCode", statusCode), zap.Any("latency", latency), zap.String("method", method), zap.String("path", path), zap.String("error", errorMessage))
	}
}

func NewHandler(dbInstance *db.Database, kernel kernel.Kernel, jwtSecret []byte, mode string, tracingEnabled bool) http.Handler {
	gin.SetMode(mode)
	router := gin.New()
	router.Use(ginToZap(logger.APILog))

	AddUIService(router)

	apiGroup := router.Group("/api/v1")
	if tracingEnabled {
		apiGroup.Use(Tracing("ella-core/api"))
	}
	if gin.Mode() != gin.TestMode {
		apiGroup.Use(RateLimitMiddleware())
	}

	// Operator (Authenticated)
	apiGroup.GET("/operator", Authenticate(jwtSecret), RequirePermission(PermReadOperator), GetOperator(dbInstance))
	apiGroup.PUT("/operator/slice", Authenticate(jwtSecret), RequirePermission(PermUpdateOperatorSlice), UpdateOperatorSlice(dbInstance))
	apiGroup.GET("/operator/slice", Authenticate(jwtSecret), RequirePermission(PermGetOperatorSlice), GetOperatorSlice(dbInstance))
	apiGroup.PUT("/operator/tracking", Authenticate(jwtSecret), RequirePermission(PermUpdateOperatorTracking), UpdateOperatorTracking(dbInstance))
	apiGroup.GET("/operator/tracking", Authenticate(jwtSecret), RequirePermission(PermGetOperatorTracking), GetOperatorTracking(dbInstance))
	apiGroup.PUT("/operator/id", Authenticate(jwtSecret), RequirePermission(PermUpdateOperatorID), UpdateOperatorID(dbInstance))
	apiGroup.GET("/operator/id", Authenticate(jwtSecret), RequirePermission(PermGetOperatorID), GetOperatorID(dbInstance))
	apiGroup.PUT("/operator/code", Authenticate(jwtSecret), RequirePermission(PermUpdateOperatorCode), UpdateOperatorCode(dbInstance))
	apiGroup.PUT("/operator/home-network", Authenticate(jwtSecret), RequirePermission(PermUpdateOperatorHomeNetwork), UpdateOperatorHomeNetwork(dbInstance))

	// Radios (Authenticated)
	apiGroup.GET("/radios", Authenticate(jwtSecret), RequirePermission(PermListRadios), ListRadios())
	apiGroup.GET("/radios/:name", Authenticate(jwtSecret), RequirePermission(PermReadRadio), GetRadio())

	// Backup and Restore (Authenticated)
	apiGroup.POST("/backup", Authenticate(jwtSecret), RequirePermission(PermBackup), Backup(dbInstance))
	apiGroup.POST("/restore", Authenticate(jwtSecret), RequirePermission(PermRestore), Restore(dbInstance))

	// Status (Unauthenticated)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", GetStatus(dbInstance).ServeHTTP)

	// Metrics (Unauthenticated)
	mux.HandleFunc("GET /api/v1/metrics", GetMetrics().ServeHTTP)

	// Authentication
	mux.HandleFunc("POST /api/v1/auth/login", Login(dbInstance, jwtSecret).ServeHTTP)
	mux.HandleFunc("POST /api/v1/auth/lookup-token", LookupToken(dbInstance, jwtSecret).ServeHTTP)

	// Users (Authenticated except for first user creation)
	mux.HandleFunc("GET /api/v1/users/me", AuthenticateHTTP(jwtSecret, GetLoggedInUser(dbInstance)).ServeHTTP)
	mux.HandleFunc("GET /api/v1/users",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermListUsers,
				ListUsers(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("POST /api/v1/users",
		RequirePermissionOrFirstUserHTTP(
			PermCreateUser,
			dbInstance,
			jwtSecret,
			CreateUser(dbInstance),
		).ServeHTTP,
	)
	mux.HandleFunc("PUT /api/v1/users/{email}",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermUpdateUser,
				UpdateUser(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("PUT /api/v1/users/{email}/password",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermUpdateUserPassword,
				UpdateUserPassword(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("GET /api/v1/users/{email}",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermReadUser,
				GetUser(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("DELETE /api/v1/users/{email}",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermDeleteUser,
				DeleteUser(dbInstance),
			),
		).ServeHTTP,
	)

	// Subscribers (Authenticated)
	mux.HandleFunc("GET /api/v1/subscribers",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermListSubscribers,
				ListSubscribers(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("POST /api/v1/subscribers",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermCreateSubscriber,
				CreateSubscriber(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("PUT /api/v1/subscribers/",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermUpdateSubscriber,
				UpdateSubscriber(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("GET /api/v1/subscribers/",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermReadSubscriber,
				GetSubscriber(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("DELETE /api/v1/subscribers/",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermDeleteSubscriber,
				DeleteSubscriber(dbInstance),
			),
		).ServeHTTP,
	)

	// Profiles (Authenticated)
	mux.HandleFunc("GET /api/v1/profiles",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermListProfiles,
				ListProfiles(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("POST /api/v1/profiles",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermCreateProfile,
				CreateProfile(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("PUT /api/v1/profiles/",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermUpdateProfile,
				UpdateProfile(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("GET /api/v1/profiles/",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermReadProfile,
				GetProfile(dbInstance),
			),
		).ServeHTTP,
	)
	mux.HandleFunc("DELETE /api/v1/profiles/",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermDeleteProfile,
				DeleteProfile(dbInstance),
			),
		).ServeHTTP,
	)

	// Routes (Authenticated)
	mux.HandleFunc("GET /api/v1/routes",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermListRoutes,
				ListRoutes(dbInstance),
			),
		).ServeHTTP,
	)

	mux.HandleFunc("POST /api/v1/routes",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermCreateRoute,
				CreateRoute(dbInstance, kernel),
			),
		).ServeHTTP,
	)

	mux.HandleFunc("GET /api/v1/routes/{id}",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermReadRoute,
				GetRoute(dbInstance),
			),
		).ServeHTTP,
	)

	mux.HandleFunc("DELETE /api/v1/routes/{id}",
		AuthenticateHTTP(jwtSecret,
			RequirePermissionHTTP(PermDeleteRoute,
				DeleteRoute(dbInstance, kernel),
			),
		).ServeHTTP,
	)

	// Mount Gin under root fallback
	mux.Handle("/", router)

	return mux
}

func AddUIService(engine *gin.Engine) {
	staticFilesSystem, err := fs.Sub(ui.FrontendFS, "out")
	if err != nil {
		logger.APILog.Fatal("Failed to create static files system", zap.Error(err))
	}

	engine.Use(func(c *gin.Context) {
		if !isAPIURLPath(c.Request.URL.Path) {
			htmlPath := strings.TrimPrefix(c.Request.URL.Path, "/") + ".html"
			if _, err := staticFilesSystem.Open(htmlPath); err == nil {
				c.Request.URL.Path = htmlPath
			}
			fileServer := http.FileServer(http.FS(staticFilesSystem))
			fileServer.ServeHTTP(c.Writer, c.Request)
			c.Abort()
		}
	})
}

func isAPIURLPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}
