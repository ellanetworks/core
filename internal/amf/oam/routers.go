package oam

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/canard/internal/amf/logger"
)

// Route is the information for every URI.
type Route struct {
	// Name is the name of this Route.
	Name string
	// Method is the string for the HTTP method. ex) GET, POST etc..
	Method string
	// Pattern is the pattern of the URI.
	Pattern string
	// HandlerFunc is the handler function of this route.
	HandlerFunc gin.HandlerFunc
}

// Routes is the list of the generated Route.
type Routes []Route

// NewRouter returns a new router.
func NewRouter() *gin.Engine {
	router := logger_util.NewGinWithLogrus(logger.GinLog)
	AddService(router)

	router.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "OPTIONS", "PUT", "PATCH", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "User-Agent", "Referrer", "Host", "Token", "X-Requested-With"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowAllOrigins:  true,
		MaxAge:           86400,
	}))

	return router
}

func AddService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/namf-oam/v1")

	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "DELETE":
			group.DELETE(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		}
	}
	return group
}

// Index is the index handler.
func Index(c *gin.Context) {
	c.String(http.StatusOK, "Hello World!")
}

var routes = Routes{
	{
		"Index",
		"GET",
		"/",
		Index,
	},
	{
		"Registered UE Context",
		"GET",
		"/registered-ue-context",
		HTTPRegisteredUEContext,
	},

	{
		"Individual Registered UE Context",
		"GET",
		"/registered-ue-context/:supi",
		HTTPRegisteredUEContext,
	},

	{
		"Purge UE Context",
		strings.ToUpper("Delete"),
		"/purge-ue-context/:supi",
		HTTPPurgeUEContext,
	},
	{
		"Active UE List",
		strings.ToUpper("get"),
		"/active-ues",
		HTTPGetActiveUes,
	},
	{
		"Amf Instance Down Notification",
		strings.ToUpper("post"),
		"/amfInstanceDown/:nfid",
		HTTPAmfInstanceDown,
	},
}
