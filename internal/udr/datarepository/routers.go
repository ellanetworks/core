package datarepository

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udr/logger"
)

// Route is the information for every URI.
type Route struct {
	// Name is the name of this Route.
	Name string
	// Method is the string for the HTTP method. e.g., GET, POST etc.
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

func subMsgShortDispatchHandlerFunc(c *gin.Context) {
	op := c.Param("ueId")
	for _, route := range subShortRoutes {
		if strings.Contains(route.Pattern, op) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
	}
	c.String(http.StatusMethodNotAllowed, "Method Not Allowed")
}

func subMsgDispatchHandlerFunc(c *gin.Context) {
	op := c.Param("servingPlmnId")
	subsToNotify := c.Param("ueId")
	for _, route := range subRoutes {
		if strings.Contains(route.Pattern, op) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
		// Sepcial case
		if subsToNotify == "subs-to-notify" && strings.Contains(route.Pattern, "subs-to-notify") && route.Method == c.Request.Method {
			c.Params = append(c.Params, gin.Param{Key: "subsId", Value: c.Param("servingPlmnId")})
			route.HandlerFunc(c)
			return
		}
	}
	c.String(http.StatusMethodNotAllowed, "Method Not Allowed")
}

func eeMsgShortDispatchHandlerFunc(c *gin.Context) {
	groupData := c.Param("ueId")
	contextData := c.Param("servingPlmnId")
	for _, route := range eeShortRoutes {
		if strings.Contains(route.Pattern, groupData) && route.Method == c.Request.Method {
			c.Params = append(c.Params, gin.Param{Key: "ueGroupId", Value: c.Param("servingPlmnId")})
			route.HandlerFunc(c)
			return
		}
		if strings.Contains(route.Pattern, contextData) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
	}
	c.String(http.StatusMethodNotAllowed, "Method Not Allowed")
}

func eeMsgDispatchHandlerFunc(c *gin.Context) {
	groupData := c.Param("ueId")
	contextData := c.Param("servingPlmnId")
	for _, route := range eeRoutes {
		if strings.Contains(route.Pattern, groupData) && route.Method == c.Request.Method {
			c.Params = append(c.Params, gin.Param{Key: "ueGroupId", Value: c.Param("servingPlmnId")})
			route.HandlerFunc(c)
			return
		}
		if strings.Contains(route.Pattern, contextData) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
	}
	c.String(http.StatusMethodNotAllowed, "Method Not Allowed")
}

func expoMsgDispatchHandlerFunc(c *gin.Context) {
	subsToNotify := c.Param("ueId")
	op := c.Param("subId")
	for _, route := range expoRoutes {
		if strings.Contains(route.Pattern, op) && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
		if subsToNotify == "subs-to-notify" && strings.Contains(route.Pattern, "subs-to-notify") && route.Method == c.Request.Method {
			route.HandlerFunc(c)
			return
		}
	}
	c.String(http.StatusMethodNotAllowed, "Method Not Allowed")
}

func AddService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/nudr-dr/v1")

	for _, route := range routes {
		switch route.Method {
		case "GET":
			group.GET(route.Pattern, route.HandlerFunc)
		case "PATCH":
			group.PATCH(route.Pattern, route.HandlerFunc)
		case "POST":
			group.POST(route.Pattern, route.HandlerFunc)
		case "PUT":
			group.PUT(route.Pattern, route.HandlerFunc)
		case "DELETE":
			group.DELETE(route.Pattern, route.HandlerFunc)
		}
	}

	subPatternShort := "/subscription-data/:ueId"
	group.Any(subPatternShort, subMsgShortDispatchHandlerFunc)

	subPattern := "/subscription-data/:ueId/:servingPlmnId"
	group.Any(subPattern, subMsgDispatchHandlerFunc)

	eePatternShort := "/subscription-data/:ueId/:servingPlmnId/ee-subscriptions"
	group.Any(eePatternShort, eeMsgShortDispatchHandlerFunc)

	eePattern := "/subscription-data/:ueId/:servingPlmnId/ee-subscriptions/:subsId"
	group.Any(eePattern, eeMsgDispatchHandlerFunc)

	expoPatternShort := "/exposure-data/:ueId/:subId"
	group.Any(expoPatternShort, expoMsgDispatchHandlerFunc)

	expoPattern := "/exposure-data/:ueId/:subId/:pduSessionId"
	group.Any(expoPattern, expoMsgDispatchHandlerFunc)

	return group
}

var routes = Routes{
	{
		"HTTPAmfContext3gpp",
		strings.ToUpper("Patch"),
		"/subscription-data/:ueId/:servingPlmnId/amf-3gpp-access",
		HTTPAmfContext3gpp,
	},

	{
		"HTTPCreateAmfContext3gpp",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/amf-3gpp-access",
		HTTPCreateAmfContext3gpp,
	},

	{
		"HTTPQueryAmfContext3gpp",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/amf-3gpp-access",
		HTTPQueryAmfContext3gpp,
	},

	{
		"HTTPQueryAmData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/am-data",
		HTTPQueryAmData,
	},

	{
		"HTTPQueryAuthenticationStatus",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/authentication-status",
		HTTPQueryAuthenticationStatus,
	},

	{
		"HTTPModifyAuthentication",
		strings.ToUpper("Patch"),
		"/subscription-data/:ueId/:servingPlmnId/authentication-subscription",
		HTTPModifyAuthentication,
	},

	{
		"HTTPQueryAuthSubsData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/authentication-subscription",
		HTTPQueryAuthSubsData,
	},

	{
		"HTTPCreateAuthenticationStatus",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/authentication-status",
		HTTPCreateAuthenticationStatus,
	},

	{
		"HTTPPolicyDataSubsToNotifyPost",
		strings.ToUpper("Post"),
		"/policy-data/subs-to-notify",
		HTTPPolicyDataSubsToNotifyPost,
	},

	{
		"HTTPPolicyDataSubsToNotifySubsIdDelete",
		strings.ToUpper("Delete"),
		"/policy-data/subs-to-notify/:subsId",
		HTTPPolicyDataSubsToNotifySubsIdDelete,
	},

	{
		"HTTPPolicyDataSubsToNotifySubsIdPut",
		strings.ToUpper("Put"),
		"/policy-data/subs-to-notify/:subsId",
		HTTPPolicyDataSubsToNotifySubsIdPut,
	},

	{
		"HTTPPolicyDataUesUeIdAmDataGet",
		strings.ToUpper("Get"),
		"/policy-data/ues/:ueId/am-data",
		HTTPPolicyDataUesUeIdAmDataGet,
	},

	{
		"HTTPPolicyDataUesUeIdSmDataGet",
		strings.ToUpper("Get"),
		"/policy-data/ues/:ueId/sm-data",
		HTTPPolicyDataUesUeIdSmDataGet,
	},

	{
		"HTTPQueryProvisionedData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data",
		HTTPQueryProvisionedData,
	},

	{
		"HTTPRemovesdmSubscriptions",
		strings.ToUpper("Delete"),
		"/subscription-data/:ueId/:servingPlmnId/sdm-subscriptions/:subsId",
		HTTPRemovesdmSubscriptions,
	},

	{
		"HTTPUpdatesdmsubscriptions",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/sdm-subscriptions/:subsId",
		HTTPUpdatesdmsubscriptions,
	},

	{
		"HTTPCreateSdmSubscriptions",
		strings.ToUpper("Post"),
		"/subscription-data/:ueId/:servingPlmnId/sdm-subscriptions",
		HTTPCreateSdmSubscriptions,
	},

	{
		"HTTPQuerysdmsubscriptions",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/sdm-subscriptions",
		HTTPQuerysdmsubscriptions,
	},

	{
		"HTTPQuerySmfSelectData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/smf-selection-subscription-data",
		HTTPQuerySmfSelectData,
	},

	{
		"HTTPQuerySmData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/sm-data",
		HTTPQuerySmData,
	},

	{
		"HTTPCreateAMFSubscriptions",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/ee-subscriptions/:subsId/amf-subscriptions",
		HTTPCreateAMFSubscriptions,
	},

	{
		"HTTPModifyAmfSubscriptionInfo",
		strings.ToUpper("Patch"),
		"/subscription-data/:ueId/:servingPlmnId/ee-subscriptions/:subsId/amf-subscriptions",
		HTTPModifyAmfSubscriptionInfo,
	},

	{
		"HTTPRemoveAmfSubscriptionsInfo",
		strings.ToUpper("Delete"),
		"/subscription-data/:ueId/:servingPlmnId/ee-subscriptions/:subsId/amf-subscriptions",
		HTTPRemoveAmfSubscriptionsInfo,
	},

	{
		"HTTPGetAmfSubscriptionInfo",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/ee-subscriptions/:subsId/amf-subscriptions",
		HTTPGetAmfSubscriptionInfo,
	},
}

var subRoutes = Routes{
	{
		"HTTPRemovesubscriptionDataSubscriptions",
		strings.ToUpper("Delete"),
		"/subscription-data/subs-to-notify/:subsId",
		HTTPRemovesubscriptionDataSubscriptions,
	},
}

var subShortRoutes = Routes{
	{
		"HTTPPostSubscriptionDataSubscriptions",
		strings.ToUpper("Post"),
		"/subscription-data/subs-to-notify",
		HTTPPostSubscriptionDataSubscriptions,
	},
}

var eeShortRoutes = Routes{
	{
		"HTTPCreateEeGroupSubscriptions",
		strings.ToUpper("Post"),
		"/subscription-data/group-data/:ueGroupId/ee-subscriptions",
		HTTPCreateEeGroupSubscriptions,
	},

	{
		"HTTPQueryEeGroupSubscriptions",
		strings.ToUpper("Get"),
		"/subscription-data/group-data/:ueGroupId/ee-subscriptions",
		HTTPQueryEeGroupSubscriptions,
	},

	{
		"HTTPCreateEeSubscriptions",
		strings.ToUpper("Post"),
		"/subscription-data/:ueId/context-data/ee-subscriptions",
		HTTPCreateEeSubscriptions,
	},

	{
		"HTTPQueryeesubscriptions",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/context-data/ee-subscriptions",
		HTTPQueryeesubscriptions,
	},
}

var eeRoutes = Routes{
	{
		"HTTPRemoveeeSubscriptions",
		strings.ToUpper("Delete"),
		"/subscription-data/:ueId/context-data/ee-subscriptions/:subsId",
		HTTPRemoveeeSubscriptions,
	},

	{
		"HTTPUpdateEesubscriptions",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/context-data/ee-subscriptions/:subsId",
		HTTPUpdateEesubscriptions,
	},

	{
		"HTTPUpdateEeGroupSubscriptions",
		strings.ToUpper("Put"),
		"/subscription-data/group-data/:ueGroupId/ee-subscriptions/:subsId",
		HTTPUpdateEeGroupSubscriptions,
	},

	{
		"HTTPRemoveEeGroupSubscriptions",
		strings.ToUpper("Delete"),
		"/subscription-data/group-data/:ueGroupId/ee-subscriptions/:subsId",
		HTTPRemoveEeGroupSubscriptions,
	},
}

var expoRoutes = Routes{
	{
		"HTTPCreateSessionManagementData",
		strings.ToUpper("Put"),
		"/exposure-data/:ueId/session-management-data/:pduSessionId",
		HTTPCreateSessionManagementData,
	},

	{
		"HTTPDeleteSessionManagementData",
		strings.ToUpper("Delete"),
		"/exposure-data/:ueId/session-management-data/:pduSessionId",
		HTTPDeleteSessionManagementData,
	},

	{
		"HTTPQuerySessionManagementData",
		strings.ToUpper("Get"),
		"/exposure-data/:ueId/session-management-data/:pduSessionId",
		HTTPQuerySessionManagementData,
	},

	{
		"CreateAccessAndMobilityData",
		strings.ToUpper("Put"),
		"/exposure-data/:ueId/access-and-mobility-data",
		CreateAccessAndMobilityData,
	},

	{
		"DeleteAccessAndMobilityData",
		strings.ToUpper("Delete"),
		"/exposure-data/:ueId/access-and-mobility-data",
		DeleteAccessAndMobilityData,
	},

	{
		"QueryAccessAndMobilityData",
		strings.ToUpper("Get"),
		"/exposure-data/:ueId/access-and-mobility-data",
		QueryAccessAndMobilityData,
	},

	{
		"HTTPExposureDataSubsToNotifyPost",
		strings.ToUpper("Post"),
		"/exposure-data/subs-to-notify",
		HTTPExposureDataSubsToNotifyPost,
	},

	{
		"HTTPExposureDataSubsToNotifySubIdDelete",
		strings.ToUpper("Delete"),
		"/exposure-data/subs-to-notify/:subId",
		HTTPExposureDataSubsToNotifySubIdDelete,
	},

	{
		"HTTPExposureDataSubsToNotifySubIdPut",
		strings.ToUpper("Put"),
		"/exposure-data/subs-to-notify/:subId",
		HTTPExposureDataSubsToNotifySubIdPut,
	},
}
