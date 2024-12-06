package subscriberdatamanagement

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udm/producer"
)

// GetSmData - retrieve a UE's Session Management Subscription Data
func HTTPGetSmData(c *gin.Context) {
	req := httpwrapper.NewRequest(c.Request, nil)
	req.Params["supi"] = c.Param("supi")
	req.Query.Set("plmn-id", c.Query("plmn-id"))
	req.Query.Set("dnn", c.Query("dnn"))
	req.Query.Set("single-nssai", c.Query("single-nssai"))
	req.Query.Set("supported-features", c.Query("supported-features"))

	rsp := producer.HandleGetSmDataRequest(req)

	responseBody, err := openapi.Serialize(rsp.Body, "application/json")
	if err != nil {
		logger.SdmLog.Errorln(err)
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
