package server

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ui"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func ginToZap(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip logging for static files and other unwanted paths
		if strings.HasPrefix(path, "/_next/static") ||
			strings.HasPrefix(path, "/favicon.ico") ||
			strings.HasPrefix(path, "/assets/") ||
			strings.HasPrefix(path, "/static/") {
			c.Next()
			return
		}

		raw := c.Request.URL.RawQuery

		c.Next()

		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.Infof("| %3d | %15s | %-7s | %s | %s",
			statusCode, clientIP, method, path, errorMessage)
	}
}

func ginRecover(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if p := recover(); p != nil {
				var brokenPipe bool
				if ne, ok := p.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				if logger != nil {
					stack := string(debug.Stack())
					if httpRequest, err := httputil.DumpRequest(c.Request, false); err != nil {
						logger.Errorf("dump http request error: %v", err)
					} else {
						headers := strings.Split(string(httpRequest), "\r\n")
						for idx, header := range headers {
							current := strings.Split(header, ":")
							if current[0] == "Authorization" {
								headers[idx] = current[0] + ": *"
							}
						}

						if brokenPipe {
							logger.Errorf("%v\n%s", p, string(httpRequest))
						} else if gin.IsDebugging() {
							logger.Errorf("[Debugging] panic:\n%s\n%v\n%s", strings.Join(headers, "\r"), p, stack)
						} else {
							logger.Errorf("panic: %v\n%s", p, stack)
						}
					}
				}

				if brokenPipe {
					c.Error(p.(error)) // nolint: errcheck
					c.Abort()
				} else {
					c.AbortWithStatus(http.StatusInternalServerError)
				}
			}
		}()
		c.Next()
	}
}

func NewHandler(dbInstance *db.Database, kernel kernel.Kernel, jwtSecret []byte) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginToZap(logger.APILog), ginRecover(logger.APILog))
	AddUiService(router)

	apiGroup := router.Group("/api/v1")

	// Metrics (Unauthenticated)
	apiGroup.GET("/metrics", GetMetrics())

	// Status (Unauthenticated)
	apiGroup.GET("/status", GetStatus(dbInstance))

	// Subscribers (Authenticated)
	apiGroup.GET("/subscribers", Authenticate(jwtSecret), ListSubscribers(dbInstance))
	apiGroup.POST("/subscribers", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), CreateSubscriber(dbInstance))
	apiGroup.PUT("/subscribers/:imsi", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateSubscriber(dbInstance))
	apiGroup.GET("/subscribers/:imsi", Authenticate(jwtSecret), GetSubscriber(dbInstance))
	apiGroup.DELETE("/subscribers/:imsi", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), DeleteSubscriber(dbInstance))

	// Profiles (Authenticated)
	apiGroup.GET("/profiles", Authenticate(jwtSecret), ListProfiles(dbInstance))
	apiGroup.POST("/profiles", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), CreateProfile(dbInstance))
	apiGroup.PUT("/profiles/:name", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateProfile(dbInstance))
	apiGroup.GET("/profiles/:name", Authenticate(jwtSecret), GetProfile(dbInstance))
	apiGroup.DELETE("/profiles/:name", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), DeleteProfile(dbInstance))

	// Routes (Authenticated)
	apiGroup.GET("/routes", Authenticate(jwtSecret), ListRoutes(dbInstance))
	apiGroup.POST("/routes", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), CreateRoute(dbInstance, kernel))
	apiGroup.GET("/routes/:id", Authenticate(jwtSecret), GetRoute(dbInstance))
	apiGroup.DELETE("/routes/:id", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), DeleteRoute(dbInstance, kernel))

	// Operator (Authenticated)
	apiGroup.GET("/operator", Authenticate(jwtSecret), GetOperator(dbInstance))
	apiGroup.PUT("/operator/slice", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateOperatorSlice(dbInstance))
	apiGroup.GET("/operator/slice", Authenticate(jwtSecret), GetOperatorSlice(dbInstance))
	apiGroup.PUT("/operator/tracking", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateOperatorTracking(dbInstance))
	apiGroup.GET("/operator/tracking", Authenticate(jwtSecret), GetOperatorTracking(dbInstance))
	apiGroup.PUT("/operator/id", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateOperatorId(dbInstance))
	apiGroup.GET("/operator/id", Authenticate(jwtSecret), GetOperatorId(dbInstance))
	apiGroup.PUT("/operator/code", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateOperatorCode(dbInstance))
	apiGroup.PUT("/operator/home-network", Authenticate(jwtSecret), Require(AdminRole, NetworkManagerRole), UpdateOperatorHomeNetwork(dbInstance))

	// Radios (Authenticated)
	apiGroup.GET("/radios", Authenticate(jwtSecret), ListRadios())
	apiGroup.GET("/radios/:name", Authenticate(jwtSecret), GetRadio())

	// Users (Authenticated except for first user creation)
	apiGroup.GET("/users", Authenticate(jwtSecret), Require(AdminRole), ListUsers(dbInstance))
	apiGroup.POST("/users", RequireAdminOrFirstUser(dbInstance, jwtSecret), CreateUser(dbInstance))
	apiGroup.PUT("/users/:email", Authenticate(jwtSecret), Require(AdminRole), UpdateUser(dbInstance))
	apiGroup.PUT("/users/:email/password", Authenticate(jwtSecret), Require(AdminRole), UpdateUserPassword(dbInstance))
	apiGroup.GET("/users/:email", Authenticate(jwtSecret), Require(AdminRole), GetUser(dbInstance))
	apiGroup.DELETE("/users/:email", Authenticate(jwtSecret), Require(AdminRole), DeleteUser(dbInstance))
	apiGroup.GET("/users/me", Authenticate(jwtSecret), GetLoggedInUser(dbInstance))

	// Backup and Restore (Authenticated)
	apiGroup.POST("/backup", Authenticate(jwtSecret), Require(AdminRole), Backup(dbInstance))
	apiGroup.POST("/restore", Authenticate(jwtSecret), Require(AdminRole), Restore(dbInstance))

	// Authentication
	apiGroup.POST("/auth/login", Login(dbInstance, jwtSecret))
	apiGroup.POST("/auth/lookup-token", LookupToken(dbInstance, jwtSecret))

	return router
}

func AddUiService(engine *gin.Engine) {
	staticFilesSystem, err := fs.Sub(ui.FrontendFS, "out")
	if err != nil {
		logger.APILog.Fatal(err)
	}

	engine.Use(func(c *gin.Context) {
		if !isApiUrlPath(c.Request.URL.Path) {
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

func isApiUrlPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}
