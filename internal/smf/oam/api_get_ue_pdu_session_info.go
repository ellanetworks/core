package oam

import (
	"github.com/gin-gonic/gin"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/smf/producer"
)

func HTTPGetUEPDUSessionInfo(c *gin.Context) {
	req := httpwrapper.NewRequest(c.Request, nil)
	req.Params["smContextRef"] = c.Params.ByName("smContextRef")

	smContextRef := req.Params["smContextRef"]
	HTTPResponse := producer.HandleOAMGetUEPDUSessionInfo(smContextRef)

	c.JSON(HTTPResponse.Status, HTTPResponse.Body)
}
