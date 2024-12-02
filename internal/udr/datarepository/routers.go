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

// Handler to distinguish "subs-to-notify" from ":influenceId".
func appInfluDataMsgDispatchHandlerFunc(c *gin.Context) {
	influID := c.Param("influenceId")
	for _, route := range appInfluDataRoutes {
		if route.Method == c.Request.Method {
			if influID == "subs-to-notify" {
				if strings.Contains(route.Pattern, "subs-to-notify") {
					route.HandlerFunc(c)
					return
				}
			} else {
				if !strings.Contains(route.Pattern, "subs-to-notify") {
					route.HandlerFunc(c)
					return
				}
			}
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

	/*
	 * GIN wildcard issue:
	 * '/application-data/influenceData/:influenceId' and
	 * '/application-data/influenceData/subs-to-notify' patterns will be conflicted.
	 * Only can use '/application-data/influenceData/:influenceId' pattern and
	 * use a dispatch handler to distinguish "subs-to-notify" from ":influenceId".
	 */
	appInfluDataPattern := "/application-data/influenceData/:influenceId"
	group.Any(appInfluDataPattern, appInfluDataMsgDispatchHandlerFunc)

	expoPatternShort := "/exposure-data/:ueId/:subId"
	group.Any(expoPatternShort, expoMsgDispatchHandlerFunc)

	expoPattern := "/exposure-data/:ueId/:subId/:pduSessionId"
	group.Any(expoPattern, expoMsgDispatchHandlerFunc)

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
		"HTTPAmfContextNon3gpp",
		strings.ToUpper("Patch"),
		"/subscription-data/:ueId/:servingPlmnId/amf-non-3gpp-access",
		HTTPAmfContextNon3gpp,
	},

	{
		"HTTPCreateAmfContextNon3gpp",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/amf-non-3gpp-access",
		HTTPCreateAmfContextNon3gpp,
	},

	{
		"HTTPQueryAmfContextNon3gpp",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/amf-non-3gpp-access",
		HTTPQueryAmfContextNon3gpp,
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
		"HTTPCreateAuthenticationSoR",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/sor-data",
		HTTPCreateAuthenticationSoR,
	},

	{
		"HTTPQueryAuthSoR",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/sor-data",
		HTTPQueryAuthSoR,
	},

	{
		"HTTPCreateAuthenticationStatus",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/authentication-status",
		HTTPCreateAuthenticationStatus,
	},

	{
		"HTTPApplicationDataInfluenceDataGet",
		strings.ToUpper("Get"),
		"/application-data/influenceData",
		HTTPApplicationDataInfluenceDataGet,
	},

	/*
	 * GIN wildcard issue:
	 * '/application-data/influenceData/:influenceId' and
	 * '/application-data/influenceData/subs-to-notify' patterns will be conflicted.
	 * Only can use '/application-data/influenceData/:influenceId' pattern.
	 * Here ":influenceId" value should be "subs-to-notify".
	 */
	{
		"HTTPApplicationDataInfluenceDataSubsToNotifySubscriptionIdDelete",
		strings.ToUpper("Delete"),
		"/application-data/influenceData/:influenceId/:subscriptionId",
		HTTPApplicationDataInfluenceDataSubsToNotifySubscriptionIdDelete,
	},

	{
		"HTTPApplicationDataInfluenceDataSubsToNotifySubscriptionIdGet",
		strings.ToUpper("Get"),
		"/application-data/influenceData/:influenceId/:subscriptionId",
		HTTPApplicationDataInfluenceDataSubsToNotifySubscriptionIdGet,
	},

	{
		"HTTPApplicationDataInfluenceDataSubsToNotifySubscriptionIdPut",
		strings.ToUpper("Put"),
		"/application-data/influenceData/:influenceId/:subscriptionId",
		HTTPApplicationDataInfluenceDataSubsToNotifySubscriptionIdPut,
	},

	{
		"HTTPApplicationDataPfdsAppIdDelete",
		strings.ToUpper("Delete"),
		"/application-data/pfds/:appId",
		HTTPApplicationDataPfdsAppIdDelete,
	},

	{
		"HTTPApplicationDataPfdsAppIdGet",
		strings.ToUpper("Get"),
		"/application-data/pfds/:appId",
		HTTPApplicationDataPfdsAppIdGet,
	},

	{
		"HTTPApplicationDataPfdsAppIdPut",
		strings.ToUpper("Put"),
		"/application-data/pfds/:appId",
		HTTPApplicationDataPfdsAppIdPut,
	},

	{
		"HTTPApplicationDataPfdsGet",
		strings.ToUpper("Get"),
		"/application-data/pfds",
		HTTPApplicationDataPfdsGet,
	},

	{
		"HTTPPolicyDataBdtDataBdtReferenceIdDelete",
		strings.ToUpper("Delete"),
		"/policy-data/bdt-data/:bdtReferenceId",
		HTTPPolicyDataBdtDataBdtReferenceIdDelete,
	},

	{
		"HTTPPolicyDataBdtDataBdtReferenceIdGet",
		strings.ToUpper("Get"),
		"/policy-data/bdt-data/:bdtReferenceId",
		HTTPPolicyDataBdtDataBdtReferenceIdGet,
	},

	{
		"HTTPPolicyDataBdtDataBdtReferenceIdPut",
		strings.ToUpper("Put"),
		"/policy-data/bdt-data/:bdtReferenceId",
		HTTPPolicyDataBdtDataBdtReferenceIdPut,
	},

	{
		"HTTPPolicyDataBdtDataGet",
		strings.ToUpper("Get"),
		"/policy-data/bdt-data",
		HTTPPolicyDataBdtDataGet,
	},

	{
		"HTTPPolicyDataPlmnsPlmnIdUePolicySetGet",
		strings.ToUpper("Get"),
		"/policy-data/plmns/:plmnId/ue-policy-set",
		HTTPPolicyDataPlmnsPlmnIdUePolicySetGet,
	},

	{
		"HTTPPolicyDataSponsorConnectivityDataSponsorIdGet",
		strings.ToUpper("Get"),
		"/policy-data/sponsor-connectivity-data/:sponsorId",
		HTTPPolicyDataSponsorConnectivityDataSponsorIdGet,
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
		"HTTPPolicyDataUesUeIdOperatorSpecificDataGet",
		strings.ToUpper("Get"),
		"/policy-data/ues/:ueId/operator-specific-data",
		HTTPPolicyDataUesUeIdOperatorSpecificDataGet,
	},

	{
		"HTTPPolicyDataUesUeIdOperatorSpecificDataPatch",
		strings.ToUpper("Patch"),
		"/policy-data/ues/:ueId/operator-specific-data",
		HTTPPolicyDataUesUeIdOperatorSpecificDataPatch,
	},

	{
		"HTTPPolicyDataUesUeIdOperatorSpecificDataPut",
		strings.ToUpper("Put"),
		"/policy-data/ues/:ueId/operator-specific-data",
		HTTPPolicyDataUesUeIdOperatorSpecificDataPut,
	},

	{
		"HTTPPolicyDataUesUeIdSmDataGet",
		strings.ToUpper("Get"),
		"/policy-data/ues/:ueId/sm-data",
		HTTPPolicyDataUesUeIdSmDataGet,
	},

	{
		"HTTPPolicyDataUesUeIdSmDataPatch",
		strings.ToUpper("Patch"),
		"/policy-data/ues/:ueId/sm-data",
		HTTPPolicyDataUesUeIdSmDataPatch,
	},

	{
		"HTTPPolicyDataUesUeIdSmDataUsageMonIdDelete",
		strings.ToUpper("Delete"),
		"/policy-data/ues/:ueId/sm-data/:usageMonId",
		HTTPPolicyDataUesUeIdSmDataUsageMonIdDelete,
	},

	{
		"HTTPPolicyDataUesUeIdSmDataUsageMonIdGet",
		strings.ToUpper("Get"),
		"/policy-data/ues/:ueId/sm-data/:usageMonId",
		HTTPPolicyDataUesUeIdSmDataUsageMonIdGet,
	},

	{
		"HTTPPolicyDataUesUeIdSmDataUsageMonIdPut",
		strings.ToUpper("Put"),
		"/policy-data/ues/:ueId/sm-data/:usageMonId",
		HTTPPolicyDataUesUeIdSmDataUsageMonIdPut,
	},

	{
		"HTTPPolicyDataUesUeIdUePolicySetGet",
		strings.ToUpper("Get"),
		"/policy-data/ues/:ueId/ue-policy-set",
		HTTPPolicyDataUesUeIdUePolicySetGet,
	},

	{
		"HTTPPolicyDataUesUeIdUePolicySetPatch",
		strings.ToUpper("Patch"),
		"/policy-data/ues/:ueId/ue-policy-set",
		HTTPPolicyDataUesUeIdUePolicySetPatch,
	},

	{
		"HTTPPolicyDataUesUeIdUePolicySetPut",
		strings.ToUpper("Put"),
		"/policy-data/ues/:ueId/ue-policy-set",
		HTTPPolicyDataUesUeIdUePolicySetPut,
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
		"HTTPCreateSmfContextNon3gpp",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/smf-registrations/:pduSessionId",
		HTTPCreateSmfContextNon3gpp,
	},

	{
		"HTTPDeleteSmfContext",
		strings.ToUpper("Delete"),
		"/subscription-data/:ueId/:servingPlmnId/smf-registrations/:pduSessionId",
		HTTPDeleteSmfContext,
	},

	{
		"HTTPQuerySmfRegistration",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/smf-registrations/:pduSessionId",
		HTTPQuerySmfRegistration,
	},

	{
		"HTTPQuerySmfRegList",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/smf-registrations",
		HTTPQuerySmfRegList,
	},

	{
		"HTTPQuerySmfSelectData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/smf-selection-subscription-data",
		HTTPQuerySmfSelectData,
	},

	{
		"HTTPCreateSmsfContext3gpp",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/smsf-3gpp-access",
		HTTPCreateSmsfContext3gpp,
	},

	{
		"HTTPDeleteSmsfContext3gpp",
		strings.ToUpper("Delete"),
		"/subscription-data/:ueId/:servingPlmnId/smsf-3gpp-access",
		HTTPDeleteSmsfContext3gpp,
	},

	{
		"HTTPQuerySmsfContext3gpp",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/smsf-3gpp-access",
		HTTPQuerySmsfContext3gpp,
	},

	{
		"HTTPCreateSmsfContextNon3gpp",
		strings.ToUpper("Put"),
		"/subscription-data/:ueId/:servingPlmnId/smsf-non-3gpp-access",
		HTTPCreateSmsfContextNon3gpp,
	},

	{
		"HTTPDeleteSmsfContextNon3gpp",
		strings.ToUpper("Delete"),
		"/subscription-data/:ueId/:servingPlmnId/smsf-non-3gpp-access",
		HTTPDeleteSmsfContextNon3gpp,
	},

	{
		"HTTPQuerySmsfContextNon3gpp",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/smsf-non-3gpp-access",
		HTTPQuerySmsfContextNon3gpp,
	},

	{
		"HTTPQuerySmsMngData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/sms-mng-data",
		HTTPQuerySmsMngData,
	},

	{
		"HTTPQuerySmsData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/sms-data",
		HTTPQuerySmsData,
	},

	{
		"HTTPQuerySmData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/sm-data",
		HTTPQuerySmData,
	},

	{
		"HTTPQueryTraceData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/:servingPlmnId/provisioned-data/trace-data",
		HTTPQueryTraceData,
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
		"HTTPQueryEEData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/ee-profile-data",
		HTTPQueryEEData,
	},

	{
		"HTTPPatchOperSpecData",
		strings.ToUpper("Patch"),
		"/subscription-data/:ueId/operator-specific-data",
		HTTPPatchOperSpecData,
	},

	{
		"HTTPQueryOperSpecData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/operator-specific-data",
		HTTPQueryOperSpecData,
	},

	{
		"HTTPGetppData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/pp-data",
		HTTPGetppData,
	},

	{
		"HTTPModifyPpData",
		strings.ToUpper("Patch"),
		"/subscription-data/:ueId/pp-data",
		HTTPModifyPpData,
	},

	{
		"HTTPGetIdentityData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/identity-data",
		HTTPGetIdentityData,
	},

	{
		"HTTPGetOdbData",
		strings.ToUpper("Get"),
		"/subscription-data/:ueId/operator-determined-barring-data",
		HTTPGetOdbData,
	},

	// Sepcial case
	{
		"HTTPRemovesubscriptionDataSubscriptions",
		strings.ToUpper("Delete"),
		"/subscription-data/subs-to-notify/:subsId",
		HTTPRemovesubscriptionDataSubscriptions,
	},
}

var subShortRoutes = Routes{
	{
		"HTTPGetSharedData",
		strings.ToUpper("Get"),
		"/subscription-data/shared-data",
		HTTPGetSharedData,
	},

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

var appInfluDataRoutes = Routes{
	{
		"HTTPApplicationDataInfluenceDataSubsToNotifyGet",
		strings.ToUpper("Get"),
		"/application-data/influenceData/subs-to-notify",
		HTTPApplicationDataInfluenceDataSubsToNotifyGet,
	},

	{
		"HTTPApplicationDataInfluenceDataSubsToNotifyPost",
		strings.ToUpper("Post"),
		"/application-data/influenceData/subs-to-notify",
		HTTPApplicationDataInfluenceDataSubsToNotifyPost,
	},

	{
		"HTTPApplicationDataInfluenceDataInfluenceIdDelete",
		strings.ToUpper("Delete"),
		"/application-data/influenceData/:influenceId",
		HTTPApplicationDataInfluenceDataInfluenceIdDelete,
	},

	{
		"HTTPApplicationDataInfluenceDataInfluenceIdPatch",
		strings.ToUpper("Patch"),
		"/application-data/influenceData/:influenceId",
		HTTPApplicationDataInfluenceDataInfluenceIdPatch,
	},

	{
		"HTTPApplicationDataInfluenceDataInfluenceIdPut",
		strings.ToUpper("Put"),
		"/application-data/influenceData/:influenceId",
		HTTPApplicationDataInfluenceDataInfluenceIdPut,
	},

	{
		"HTTPApplicationDataInfluenceDataSubsToNotifyGet",
		strings.ToUpper("Get"),
		"/application-data/influenceData/:influenceId",
		HTTPApplicationDataInfluenceDataSubsToNotifyGet,
	},

	{
		"HTTPApplicationDataInfluenceDataSubsToNotifyPost",
		strings.ToUpper("Post"),
		"/application-data/influenceData/:influenceId",
		HTTPApplicationDataInfluenceDataSubsToNotifyPost,
	},
}
