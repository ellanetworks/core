// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

/*
 * Npcf_PolicyAuthorization Service API
 *
 * This is the Policy Authorization Service
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package policyauthorization

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/canard/internal/pcf/logger"
	"github.com/yeastengine/canard/internal/pcf/producer"
)

// HTTPDeleteAppSession - Deletes an existing Individual Application Session Context
func HTTPDeleteAppSession(c *gin.Context) {
	var eventsSubscReqData *models.EventsSubscReqData

	requestBody, err := c.GetRawData()
	if err != nil {
		problemDetail := models.ProblemDetails{
			Title:  "System failure",
			Status: http.StatusInternalServerError,
			Detail: err.Error(),
			Cause:  "SYSTEM_FAILURE",
		}
		logger.PolicyAuthorizationlog.Errorf("Get Request Body error: %+v", err)
		c.JSON(http.StatusInternalServerError, problemDetail)
		return
	}

	// EventsSubscReqData is Optional
	if len(requestBody) > 0 {
		err = openapi.Deserialize(&eventsSubscReqData, requestBody, "application/json")
		if err != nil {
			problemDetail := "[Request Body] " + err.Error()
			rsp := models.ProblemDetails{
				Title:  "Malformed request syntax",
				Status: http.StatusBadRequest,
				Detail: problemDetail,
			}
			logger.PolicyAuthorizationlog.Errorln(problemDetail)
			c.JSON(http.StatusBadRequest, rsp)
			return
		}
	}

	req := httpwrapper.NewRequest(c.Request, eventsSubscReqData)
	req.Params["appSessionId"], _ = c.Params.Get("appSessionId")

	rsp := producer.HandleDeleteAppSessionContext(req)

	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
	if err != nil {
		logger.PolicyAuthorizationlog.Errorln(err)
		problemDetails := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, problemDetails)
	} else {
		c.Data(rsp.Status, "application/json", responseBody)
	}
}

// HTTPGetAppSession - Reads an existing Individual Application Session Context
func HTTPGetAppSession(c *gin.Context) {
	req := httpwrapper.NewRequest(c.Request, nil)
	req.Params["appSessionId"], _ = c.Params.Get("appSessionId")

	rsp := producer.HandleGetAppSessionContext(req)

	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
	if err != nil {
		logger.PolicyAuthorizationlog.Errorln(err)
		problemDetails := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, problemDetails)
	} else {
		c.Data(rsp.Status, "application/json", responseBody)
	}
}

// HTTPModAppSession - Modifies an existing Individual Application Session Context
func HTTPModAppSession(c *gin.Context) {
	var appSessionContextUpdateData models.AppSessionContextUpdateData

	requestBody, err := c.GetRawData()
	if err != nil {
		problemDetail := models.ProblemDetails{
			Title:  "System failure",
			Status: http.StatusInternalServerError,
			Detail: err.Error(),
			Cause:  "SYSTEM_FAILURE",
		}
		logger.PolicyAuthorizationlog.Errorf("Get Request Body error: %+v", err)
		c.JSON(http.StatusInternalServerError, problemDetail)
		return
	}

	err = openapi.Deserialize(&appSessionContextUpdateData, requestBody, "application/json")
	if err != nil {
		problemDetail := "[Request Body] " + err.Error()
		rsp := models.ProblemDetails{
			Title:  "Malformed request syntax",
			Status: http.StatusBadRequest,
			Detail: problemDetail,
		}
		logger.PolicyAuthorizationlog.Errorln(problemDetail)
		c.JSON(http.StatusBadRequest, rsp)
		return
	}

	req := httpwrapper.NewRequest(c.Request, appSessionContextUpdateData)
	req.Params["appSessionId"], _ = c.Params.Get("appSessionId")

	rsp := producer.HandleModAppSessionContext(req)

	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
	if err != nil {
		logger.PolicyAuthorizationlog.Errorln(err)
		problemDetails := models.ProblemDetails{
			Status: http.StatusInternalServerError,
			Cause:  "SYSTEM_FAILURE",
			Detail: err.Error(),
		}
		c.JSON(http.StatusInternalServerError, problemDetails)
	} else {
		c.Data(rsp.Status, "application/json", responseBody)
	}
}
