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

// GetSharedData - retrieve shared data
func HTTPGetSharedData(c *gin.Context) {
	req := httpwrapper.NewRequest(c.Request, nil)
	req.Query["sharedDataIds"] = c.QueryArray("shared-data-ids")
	req.Query["supported-features"] = c.QueryArray("supported-features")

	rsp := producer.HandleGetSharedDataRequest(req)

	// step 5: response
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
