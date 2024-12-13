package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc gin.HandlerFunc
}

type Routes []Route

func AddApiService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/api/v1")

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
		}
	}
	return group
}

var routes = Routes{
	{
		"GetMetrics",
		http.MethodGet,
		"/metrics",
		GetMetrics,
	},
	{
		"GetStatus",
		http.MethodGet,
		"/status",
		GetStatus,
	},
	{
		"GetSubscribers",
		http.MethodGet,
		"/subscriber",
		GetSubscribers,
	},

	{
		"GetSubscriberByID",
		http.MethodGet,
		"/subscriber/:ueId",
		GetSubscriberByID,
	},

	{
		"PostSubscriberByID",
		http.MethodPost,
		"/subscriber/:ueId",
		PostSubscriberByID,
	},

	{
		"PutSubscriberByID",
		http.MethodPut,
		"/subscriber/:ueId/:servingPlmnId",
		PutSubscriberByID,
	},

	{
		"DeleteSubscriberByID",
		http.MethodDelete,
		"/subscriber/:ueId",
		DeleteSubscriberByID,
	},

	{
		"PatchSubscriberByID",
		http.MethodPatch,
		"/subscriber/:ueId/:servingPlmnId",
		PatchSubscriberByID,
	},

	{
		"GetDeviceGroups",
		http.MethodGet,
		"/device-group",
		GetDeviceGroups,
	},

	{
		"GetDeviceGroupByName",
		http.MethodGet,
		"/device-group/:group-name",
		GetDeviceGroupByName,
	},

	{
		"DeviceGroupGroupNameDelete",
		http.MethodDelete,
		"/device-group/:group-name",
		DeviceGroupGroupNameDelete,
	},

	{
		"DeviceGroupGroupNamePatch",
		http.MethodPatch,
		"/device-group/:group-name",
		DeviceGroupGroupNamePatch,
	},

	{
		"DeviceGroupGroupNamePut",
		http.MethodPut,
		"/device-group/:group-name",
		DeviceGroupGroupNamePut,
	},

	{
		"DeviceGroupGroupNamePost",
		http.MethodPost,
		"/device-group/:group-name",
		DeviceGroupGroupNamePost,
	},

	{
		"GetNetworkSlices",
		http.MethodGet,
		"/network-slice",
		GetNetworkSlices,
	},

	{
		"GetNetworkSliceByName",
		http.MethodGet,
		"/network-slice/:slice-name",
		GetNetworkSliceByName,
	},

	{
		"NetworkSliceSliceNameDelete",
		http.MethodDelete,
		"/network-slice/:slice-name",
		NetworkSliceSliceNameDelete,
	},

	{
		"NetworkSliceSliceNamePost",
		http.MethodPost,
		"/network-slice/:slice-name",
		NetworkSliceSliceNamePost,
	},

	{
		"NetworkSliceSliceNamePut",
		http.MethodPut,
		"/network-slice/:slice-name",
		NetworkSliceSliceNamePut,
	},
	{
		"ListRadios",
		http.MethodGet,
		"/radios",
		ListRadios,
	},
	{
		"CreateRadio",
		http.MethodPost,
		"/radios/:radio-name",
		CreateRadio,
	},
	{
		"DeleteRadio",
		http.MethodDelete,
		"/radios/:radio-name",
		DeleteRadio,
	},
}
