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

	// Metrics (Unauthenticated)
	apiGroup.GET("/metrics", GetMetrics())

	// Status (Unauthenticated)
	apiGroup.GET("/status", GetStatus(dbInstance))

	// Subscribers (Authenticated)
	apiGroup.GET("/subscribers", Authenticate(jwtSecret), RequirePermission(PermListSubscribers), ListSubscribers(dbInstance))
	apiGroup.POST("/subscribers", Authenticate(jwtSecret), RequirePermission(PermCreateSubscriber), CreateSubscriber(dbInstance))
	apiGroup.PUT("/subscribers/:imsi", Authenticate(jwtSecret), RequirePermission(PermUpdateSubscriber), UpdateSubscriber(dbInstance))
	apiGroup.GET("/subscribers/:imsi", Authenticate(jwtSecret), RequirePermission(PermReadSubscriber), GetSubscriber(dbInstance))
	apiGroup.DELETE("/subscribers/:imsi", Authenticate(jwtSecret), RequirePermission(PermDeleteSubscriber), DeleteSubscriber(dbInstance))

	// Profiles (Authenticated)
	apiGroup.GET("/profiles", Authenticate(jwtSecret), RequirePermission(PermListProfiles), ListProfiles(dbInstance))
	apiGroup.POST("/profiles", Authenticate(jwtSecret), RequirePermission(PermCreateProfile), CreateProfile(dbInstance))
	apiGroup.PUT("/profiles/:name", Authenticate(jwtSecret), RequirePermission(PermUpdateProfile), UpdateProfile(dbInstance))
	apiGroup.GET("/profiles/:name", Authenticate(jwtSecret), RequirePermission(PermReadProfile), GetProfile(dbInstance))
	apiGroup.DELETE("/profiles/:name", Authenticate(jwtSecret), RequirePermission(PermDeleteProfile), DeleteProfile(dbInstance))

	// Routes (Authenticated)
	apiGroup.GET("/routes", Authenticate(jwtSecret), RequirePermission(PermListRoutes), ListRoutes(dbInstance))
	apiGroup.POST("/routes", Authenticate(jwtSecret), RequirePermission(PermCreateRoute), CreateRoute(dbInstance, kernel))
	apiGroup.GET("/routes/:id", Authenticate(jwtSecret), RequirePermission(PermReadRoute), GetRoute(dbInstance))
	apiGroup.DELETE("/routes/:id", Authenticate(jwtSecret), RequirePermission(PermDeleteRoute), DeleteRoute(dbInstance, kernel))

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

	// Users (Authenticated except for first user creation)
	apiGroup.GET("/users", Authenticate(jwtSecret), RequirePermission(PermListUsers), ListUsers(dbInstance))
	apiGroup.POST("/users", RequirePermissionOrFirstUser(PermCreateUser, dbInstance, jwtSecret), CreateUser(dbInstance))
	apiGroup.PUT("/users/:email", Authenticate(jwtSecret), RequirePermission(PermUpdateUser), UpdateUser(dbInstance))
	apiGroup.PUT("/users/:email/password", Authenticate(jwtSecret), RequirePermission(PermUpdateUserPassword), UpdateUserPassword(dbInstance))
	apiGroup.GET("/users/:email", Authenticate(jwtSecret), RequirePermission(PermReadUser), GetUser(dbInstance))
	apiGroup.DELETE("/users/:email", Authenticate(jwtSecret), RequirePermission(PermDeleteUser), DeleteUser(dbInstance))
	apiGroup.GET("/users/me", Authenticate(jwtSecret), RequirePermission(PermReadMyUser), GetLoggedInUser(dbInstance))

	// Backup and Restore (Authenticated)
	apiGroup.POST("/backup", Authenticate(jwtSecret), RequirePermission(PermBackup), Backup(dbInstance))
	apiGroup.POST("/restore", Authenticate(jwtSecret), RequirePermission(PermRestore), Restore(dbInstance))

	// Authentication
	apiGroup.POST("/auth/login", Login(dbInstance, jwtSecret))
	apiGroup.POST("/auth/lookup-token", LookupToken(dbInstance, jwtSecret))

	return router
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
