package subscriberdatamanagement

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udm/logger"
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

func oneLayerPathHandlerFunc(c *gin.Context) {
	supi := c.Param("supi")
	for _, route := range oneLayerPathRouter {
		if strings.Contains(route.Pattern, supi) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
	}

	// special case for :supi
	if c.Request.Method == strings.ToUpper("Get") {
		HTTPGetSupi(c)
		return
	}

	c.String(http.StatusNotFound, "404 page not found")
}

func twoLayerPathHandlerFunc(c *gin.Context) {
	supi := c.Param("supi")
	op := c.Param("subscriptionId")

	// for "/shared-data-subscriptions/:subscriptionId"
	if supi == "shared-data-subscriptions" && strings.ToUpper("Delete") == c.Request.Method {
		HTTPUnsubscribeForSharedData(c)
		return
	}

	// for "/shared-data-subscriptions/:subscriptionId"
	if supi == "shared-data-subscriptions" && strings.ToUpper("Patch") == c.Request.Method {
		HTTPModifyForSharedData(c)
		return
	}

	for _, route := range twoLayerPathRouter {
		if strings.Contains(route.Pattern, op) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
	}

	c.String(http.StatusNotFound, "404 page not found")
}

func threeLayerPathHandlerFunc(c *gin.Context) {
	op := c.Param("subscriptionId")

	// for "/:supi/sdm-subscriptions/:subscriptionId"
	if op == "sdm-subscriptions" && strings.ToUpper("Delete") == c.Request.Method {
		var tmpParams gin.Params
		tmpParams = append(tmpParams, gin.Param{Key: "supi", Value: c.Param("supi")})
		tmpParams = append(tmpParams, gin.Param{Key: "subscriptionId", Value: c.Param("thirdLayer")})
		c.Params = tmpParams
		HTTPUnsubscribe(c)
		return
	}

	// for "/:supi/am-data/sor-ack"
	if op == "am-data" && strings.ToUpper("Put") == c.Request.Method {
		HTTPInfo(c)
		return
	}

	// for "/:supi/sdm-subscriptions/:subscriptionId"
	if op == "sdm-subscriptions" && strings.ToUpper("Patch") == c.Request.Method {
		var tmpParams gin.Params
		tmpParams = append(tmpParams, gin.Param{Key: "supi", Value: c.Param("supi")})
		tmpParams = append(tmpParams, gin.Param{Key: "subscriptionId", Value: c.Param("thirdLayer")})
		c.Params = tmpParams
		HTTPModify(c)
		return
	}

	c.String(http.StatusNotFound, "404 page not found")
}

func AddService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/nudm-sdm/v1")

	oneLayerPath := "/:supi"
	group.Any(oneLayerPath, oneLayerPathHandlerFunc)

	twoLayerPath := "/:supi/:subscriptionId"
	group.Any(twoLayerPath, twoLayerPathHandlerFunc)

	threeLayerPath := "/:supi/:subscriptionId/:thirdLayer"
	group.Any(threeLayerPath, threeLayerPathHandlerFunc)

	return group
}

var oneLayerPathRouter = Routes{
	{
		"GetSupi",
		strings.ToUpper("Get"),
		"/:supi",
		HTTPGetSupi,
	},

	{
		"GetSharedData",
		strings.ToUpper("Get"),
		"/shared-data",
		HTTPGetSharedData,
	},

	{
		"SubscribeToSharedData",
		strings.ToUpper("Post"),
		"/shared-data-subscriptions",
		HTTPSubscribeToSharedData,
	},
}

var twoLayerPathRouter = Routes{
	{
		"GetAmData",
		strings.ToUpper("Get"),
		"/:supi/am-data",
		HTTPGetAmData,
	},

	{
		"GetSmfSelectData",
		strings.ToUpper("Get"),
		"/:supi/smf-select-data",
		HTTPGetSmfSelectData,
	},

	{
		"GetSmsMngData",
		strings.ToUpper("Get"),
		"/:supi/sms-mng-data",
		HTTPGetSmsMngData,
	},

	{
		"GetSmsData",
		strings.ToUpper("Get"),
		"/:supi/sms-data",
		HTTPGetSmsData,
	},

	{
		"GetSmData",
		strings.ToUpper("Get"),
		"/:supi/sm-data",
		HTTPGetSmData,
	},

	{
		"GetNssai",
		strings.ToUpper("Get"),
		"/:supi/nssai",
		HTTPGetNssai,
	},

	{
		"Subscribe",
		strings.ToUpper("Post"),
		"/:supi/sdm-subscriptions",
		HTTPSubscribe,
	},

	{
		"GetTraceData",
		strings.ToUpper("Get"),
		"/:supi/trace-data",
		HTTPGetTraceData,
	},

	{
		"GetUeContextInSmfData",
		strings.ToUpper("Get"),
		"/:supi/ue-context-in-smf-data",
		HTTPGetUeContextInSmfData,
	},

	{
		"GetUeContextInSmsfData",
		strings.ToUpper("Get"),
		"/:supi/ue-context-in-smsf-data",
		HTTPGetUeContextInSmsfData,
	},
}
