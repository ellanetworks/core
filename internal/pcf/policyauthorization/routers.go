package policyauthorization

import (
	"strings"

	"github.com/gin-gonic/gin"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/pcf/logger"
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
	router := logger_util.NewGinWithZap(logger.GinLog)
	AddService(router)
	return router
}

func AddService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/npcf-policyauthorization/v1")

	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		case "PUT":
			group.PUT(route.Pattern, route.HandlerFunc)
		case "DELETE":
			group.DELETE(route.Pattern, route.HandlerFunc)
		case "PATCH":
			group.PATCH(route.Pattern, route.HandlerFunc)
		}
	}
	return group
}

var routes = Routes{
	{
		"HTTPPostAppSessions",
		strings.ToUpper("Post"),
		"/app-sessions",
		HTTPPostAppSessions,
	},

	{
		"HTTPDeleteEventsSubsc",
		strings.ToUpper("Delete"),
		"/app-sessions/:appSessionId/events-subscription",
		HTTPDeleteEventsSubsc,
	},

	{
		"HTTPUpdateEventsSubsc",
		strings.ToUpper("Put"),
		"/app-sessions/:appSessionId/events-subscription",
		HTTPUpdateEventsSubsc,
	},

	{
		"HTTPDeleteAppSession",
		strings.ToUpper("Post"),
		"/app-sessions/:appSessionId/delete",
		HTTPDeleteAppSession,
	},

	{
		"HTTPGetAppSession",
		strings.ToUpper("Get"),
		"/app-sessions/:appSessionId",
		HTTPGetAppSession,
	},

	{
		"HTTPModAppSession",
		strings.ToUpper("Patch"),
		"/app-sessions/:appSessionId",
		HTTPModAppSession,
	},
}
