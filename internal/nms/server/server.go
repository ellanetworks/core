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

func NewHandler(dbInstance *db.Database, jwtSecret []byte) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginToZap(logger.NmsLog), ginRecover(logger.NmsLog))
	AddUiService(router)

	apiGroup := router.Group("/api/v1")

	// Metrics (Unauthenticated)
	apiGroup.GET("/metrics", Any(GetMetrics()))

	// Status (Unauthenticated)
	apiGroup.GET("/status", Any(GetStatus(dbInstance)))

	// Subscribers (Authenticated)
	apiGroup.GET("/subscribers", User(ListSubscribers(dbInstance), jwtSecret))
	apiGroup.POST("/subscribers", User(CreateSubscriber(dbInstance), jwtSecret))
	apiGroup.PUT("/subscribers/:imsi", User(UpdateSubscriber(dbInstance), jwtSecret))
	apiGroup.GET("/subscribers/:imsi", User(GetSubscriber(dbInstance), jwtSecret))
	apiGroup.DELETE("/subscribers/:imsi", User(DeleteSubscriber(dbInstance), jwtSecret))

	// Profiles (Authenticated)
	apiGroup.GET("/profiles", User(ListProfiles(dbInstance), jwtSecret))
	apiGroup.POST("/profiles", User(CreateProfile(dbInstance), jwtSecret))
	apiGroup.PUT("/profiles/:name", User(UpdateProfile(dbInstance), jwtSecret))
	apiGroup.GET("/profiles/:name", User(GetProfile(dbInstance), jwtSecret))
	apiGroup.DELETE("/profiles/:name", User(DeleteProfile(dbInstance), jwtSecret))

	// Operator (Authenticated)
	apiGroup.GET("/operator", User(GetOperator(dbInstance), jwtSecret))
	apiGroup.PUT("/operator/slice", User(UpdateOperatorSlice(dbInstance), jwtSecret))
	apiGroup.GET("/operator/slice", User(GetOperatorSlice(dbInstance), jwtSecret))
	apiGroup.PUT("/operator/tracking", User(UpdateOperatorTracking(dbInstance), jwtSecret))
	apiGroup.GET("/operator/tracking", User(GetOperatorTracking(dbInstance), jwtSecret))
	apiGroup.PUT("/operator/id", User(UpdateOperatorId(dbInstance), jwtSecret))
	apiGroup.GET("/operator/id", User(GetOperatorId(dbInstance), jwtSecret))
	apiGroup.PUT("/operator/code", User(UpdateOperatorCode(dbInstance), jwtSecret))
	apiGroup.PUT("/operator/home-network", User(UpdateOperatorHomeNetwork(dbInstance), jwtSecret))

	// Radios (Authenticated)
	apiGroup.GET("/radios", User(ListRadios(), jwtSecret))
	apiGroup.GET("/radios/:name", User(GetRadio(), jwtSecret))

	// Users (Authenticated except for first user creation)
	apiGroup.GET("/users", User(ListUsers(dbInstance), jwtSecret))
	apiGroup.POST("/users", UserOrFirstUser(CreateUser(dbInstance), dbInstance, jwtSecret))
	apiGroup.PUT("/users/:email", User(UpdateUser(dbInstance), jwtSecret))
	apiGroup.GET("/users/:email", User(GetUser(dbInstance), jwtSecret))
	apiGroup.DELETE("/users/:email", User(DeleteUser(dbInstance), jwtSecret))
	apiGroup.GET("/users/me", User(GetLoggedInUser(dbInstance), jwtSecret))

	// Backup and Restore (Authenticated)
	apiGroup.POST("/backup", User(Backup(dbInstance), jwtSecret))
	apiGroup.POST("/restore", User(Restore(dbInstance), jwtSecret))

	// Authentication
	apiGroup.POST("/auth/login", Any(Login(dbInstance, jwtSecret)))
	apiGroup.POST("/auth/lookup-token", Any(LookupToken(dbInstance, jwtSecret)))

	return router
}

func AddUiService(engine *gin.Engine) {
	staticFilesSystem, err := fs.Sub(ui.FrontendFS, "out")
	if err != nil {
		logger.NmsLog.Fatal(err)
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
