package nms

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms/server"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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

func Start(dbInstance *db.Database, port int, cert_file string, key_file string) error {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginToZap(logger.NmsLog), ginRecover(logger.NmsLog))
	server.AddUiService(router)

	apiGroup := router.Group("/api/v1")

	// Metrics
	apiGroup.GET("/metrics", server.GetMetrics())

	// Status
	apiGroup.GET("/status", server.GetStatus())

	// Subscribers
	apiGroup.GET("/subscribers", server.GetSubscribers(dbInstance))
	apiGroup.GET("/subscribers/:ueId", server.GetSubscriber(dbInstance))
	apiGroup.POST("/subscribers/:ueId", server.PostSubscriber(dbInstance))
	apiGroup.DELETE("/subscribers/:ueId", server.DeleteSubscriber(dbInstance))

	// Device Groups
	apiGroup.GET("/device-groups", server.GetDeviceGroups(dbInstance))
	apiGroup.GET("/device-groups/:group-name", server.GetDeviceGroup(dbInstance))
	apiGroup.POST("/device-groups/:group-name", server.PostDeviceGroup(dbInstance))
	apiGroup.DELETE("/device-groups/:group-name", server.DeleteDeviceGroup(dbInstance))

	// Network Slices
	apiGroup.GET("/network-slices", server.GetNetworkSlices(dbInstance))
	apiGroup.GET("/network-slices/:slice-name", server.GetNetworkSlice(dbInstance))
	apiGroup.POST("/network-slices/:slice-name", server.PostNetworkSlice(dbInstance))
	apiGroup.DELETE("/network-slices/:slice-name", server.DeleteNetworkSlice(dbInstance))

	// Radios
	apiGroup.GET("/radios", server.GetRadios(dbInstance))
	apiGroup.GET("/radios/:radio-name", server.GetRadio(dbInstance))
	apiGroup.POST("/radios/:radio-name", server.PostRadio(dbInstance))
	apiGroup.DELETE("/radios/:radio-name", server.DeleteRadio(dbInstance))

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

	go func() {
		httpAddr := ":" + strconv.Itoa(port)
		h2Server := &http2.Server{
			IdleTimeout: 1 * time.Millisecond,
		}
		server := &http.Server{
			Addr:    httpAddr,
			Handler: h2c.NewHandler(router, h2Server),
		}
		err := server.ListenAndServeTLS(cert_file, key_file)
		if err != nil {
			logger.NmsLog.Errorln("couldn't start NMS server:", err)
		}
	}()
	return nil
}
