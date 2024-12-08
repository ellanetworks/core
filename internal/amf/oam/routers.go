package oam

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/yeastengine/ella/internal/amf/logger"

	utilLogger "github.com/omec-project/util/logger"
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
	router := utilLogger.NewGinWithZap(logger.GinLog)
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

var routes = Routes{
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
		"Amf Instance Down Notification",
		strings.ToUpper("post"),
		"/amfInstanceDown/:nfid",
		HTTPAmfInstanceDown,
	},
}
