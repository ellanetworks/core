// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package configapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func AddServiceSub(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/api")

	for _, route := range routesL {
		switch route.Method {
		case http.MethodGet:
			group.GET(route.Pattern, route.HandlerFunc)
		case http.MethodPost:
			group.POST(route.Pattern, route.HandlerFunc)
		case http.MethodPut:
			group.PUT(route.Pattern, route.HandlerFunc)
		case http.MethodDelete:
			group.DELETE(route.Pattern, route.HandlerFunc)
		case http.MethodPatch:
			group.PATCH(route.Pattern, route.HandlerFunc)
		}
	}

	return group
}

var routesL = Routes{
	{
		"GetExample",
		http.MethodGet,
		"/sample",
		GetSampleJSON,
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
		"Registered UE Context",
		http.MethodGet,
		"/registered-ue-context",
		GetRegisteredUEContext,
	},

	{
		"Individual Registered UE Context",
		http.MethodGet,
		"/registered-ue-context/:supi",
		GetRegisteredUEContext,
	},

	{
		"UE PDU Session Info",
		http.MethodGet,
		"/ue-pdu-session-info/:smContextRef",
		GetUEPDUSessionInfo,
	},
}
