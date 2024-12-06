package uepolicy

import (
	"strings"

	"github.com/gin-gonic/gin"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/pcf/logger"
)

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

type Routes []Route

func NewRouter() *gin.Engine {
	router := logger_util.NewGinWithZap(logger.GinLog)
	AddService(router)
	return router
}

func AddService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/npcf-ue-policy-control/v1/")

	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		case "PATCH":
			group.PATCH(route.Pattern, route.HandlerFunc)
		}
	}
	return group
}

var routes = Routes{
	{
		"PoliciesPolAssoIdDelete",
		strings.ToUpper("Delete"),
		"/policies/{polAssoId}",
		PoliciesPolAssoIdDelete,
	},

	{
		"PoliciesPolAssoIdGet",
		strings.ToUpper("Get"),
		"/policies/{polAssoId}",
		PoliciesPolAssoIdGet,
	},

	{
		"PoliciesPolAssoIdUpdatePost",
		strings.ToUpper("Post"),
		"/policies/{polAssoId}/update",
		PoliciesPolAssoIdUpdatePost,
	},

	{
		"PoliciesPost",
		strings.ToUpper("Post"),
		"/policies",
		PoliciesPost,
	},
}
