package nssaiavailability

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/yeastengine/ella/internal/nssf/logger"

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
	return router
}

func AddService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/nnssf-nssaiavailability/v1")

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
		"NSSAIAvailabilityDelete",
		strings.ToUpper("Delete"),
		"/nssai-availability/:nfId",
		HTTPNSSAIAvailabilityDelete,
	},

	{
		"NSSAIAvailabilityPatch",
		strings.ToUpper("Patch"),
		"/nssai-availability/:nfId",
		HTTPNSSAIAvailabilityPatch,
	},

	{
		"NSSAIAvailabilityPut",
		strings.ToUpper("Put"),
		"/nssai-availability/:nfId",
		HTTPNSSAIAvailabilityPut,
	},

	// Regular expressions for route matching should be unique in Gin package
	// 'subscriptions' would conflict with existing wildcard ':nfId'
	// Simply replace 'subscriptions' with ':nfId' and check if ':nfId' is 'subscriptions' in handler function
	{
		"NSSAIAvailabilityUnsubscribe",
		strings.ToUpper("Delete"),
		// "/nssai-availability/subscriptions/:subscriptionId",
		"/nssai-availability/:nfId/:subscriptionId",
		HTTPNSSAIAvailabilityUnsubscribe,
	},

	{
		"NSSAIAvailabilityPost",
		strings.ToUpper("Post"),
		"/nssai-availability/subscriptions",
		HTTPNSSAIAvailabilityPost,
	},
}
