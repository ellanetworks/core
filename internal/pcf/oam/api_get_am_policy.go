// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package oam

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/canard/internal/pcf/logger"
	"github.com/yeastengine/canard/internal/pcf/producer"
)

func setCorsHeader(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
}

func HTTPOAMGetAmPolicy(c *gin.Context) {
	setCorsHeader(c)

	req := httpwrapper.NewRequest(c.Request, nil)
	req.Params["supi"] = c.Params.ByName("supi")

	rsp := producer.HandleOAMGetAmPolicyRequest(req)

	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
	if err != nil {
		logger.OamLog.Errorln(err)
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
