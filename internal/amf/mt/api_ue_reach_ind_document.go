/*
 * Namf_MT
 *
 * AMF Mobile Termination Service
 *
 * API version: 1.0.0
 * Generated by: OpenAPI Generator (https://openapi-generator.tech)
 */

package mt

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yeastengine/canard/internal/amf/logger"
)

// EnableUeReachability - Namf_MT EnableUEReachability service Operation
func HTTPEnableUeReachability(c *gin.Context) {
	logger.MtLog.Warnf("Handle Enable Ue Reachability is not implemented.")
	c.JSON(http.StatusOK, gin.H{})
}
