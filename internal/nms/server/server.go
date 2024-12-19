package server

import (
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/ui"
	"go.uber.org/zap"
)

func ginToZap(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
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

func NewHandler(dbInstance *db.Database) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginToZap(logger.NmsLog), ginRecover(logger.NmsLog))
	AddUiService(router)

	apiGroup := router.Group("/api/v1")

	// Metrics
	apiGroup.GET("/metrics", GetMetrics())

	// Status
	apiGroup.GET("/status", GetStatus())

	// Subscribers
	apiGroup.GET("/subscribers", ListSubscribers(dbInstance))
	apiGroup.POST("/subscribers", CreateSubscriber(dbInstance))
	apiGroup.GET("/subscribers/:ueId", GetSubscriber(dbInstance))
	apiGroup.DELETE("/subscribers/:ueId", DeleteSubscriber(dbInstance))

	// Device Groups
	apiGroup.GET("/profiles", ListProfiles(dbInstance))
	apiGroup.POST("/profiles", CreateProfile(dbInstance))
	apiGroup.PUT("/profiles/:name", UpdateProfile(dbInstance))
	apiGroup.GET("/profiles/:name", GetProfile(dbInstance))
	apiGroup.DELETE("/profiles/:name", DeleteProfile(dbInstance))

	// Network Slices
	apiGroup.GET("/network-slices", ListNetworkSlices(dbInstance))
	apiGroup.POST("/network-slices", CreateNetworkSlice(dbInstance))
	apiGroup.GET("/network-slices/:name", GetNetworkSlice(dbInstance))
	apiGroup.DELETE("/network-slices/:name", DeleteNetworkSlice(dbInstance))

	// Radios
	apiGroup.GET("/radios", ListRadios(dbInstance))
	apiGroup.POST("/radios/", CreateRadio(dbInstance))
	apiGroup.GET("/radios/:name", GetRadio(dbInstance))
	apiGroup.DELETE("/radios/:name", DeleteRadio(dbInstance))

	router.Use(cors.New(cors.Config{
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders: []string{
			"Origin", "Content-Length", "Content-Type", "User-Agent",
			"Referrer", "Host", "Token", "X-Requested-With",
		},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
		MaxAge:           86400,
	}))

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
