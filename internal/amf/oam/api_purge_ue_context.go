package oam

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/amf/producer"

	"github.com/omec-project/openapi/models"
)

func HTTPPurgeUEContext(c *gin.Context) {
	setCorsHeader(c)

	amfSelf := context.AMF_Self()
	req := httpwrapper.NewRequest(c.Request, nil)
	if supi, exists := c.Params.Get("supi"); exists {
		req.Params["supi"] = supi
		reqUri := req.URL.RequestURI()
		if ue, ok := amfSelf.AmfUeFindBySupi(supi); ok {
			sbiMsg := context.SbiMsg{
				UeContextId: ue.Supi,
				ReqUri:      reqUri,
				Msg:         nil,
				Result:      make(chan context.SbiResponseMsg, 10),
			}
			ue.EventChannel.UpdateSbiHandler(producer.HandleOAMPurgeUEContextRequest)
			ue.EventChannel.SubmitMessage(sbiMsg)
			msg := <-sbiMsg.Result
			if msg.ProblemDetails != nil {
				c.JSON(int(msg.ProblemDetails.(models.ProblemDetails).Status), msg.ProblemDetails)
			} else {
				c.JSON(http.StatusOK, nil)
			}
		} else {
			logger.ProducerLog.Errorln("No Ue found by the provided supi")
			c.JSON(http.StatusNotFound, nil)
		}
	}
}
