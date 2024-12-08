package oam

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/amf/logger"
)

func HTTPAmfInstanceDown(c *gin.Context) {
	setCorsHeader(c)

	nfId, _ := c.Params.Get("nfid")
	logger.ProducerLog.Infof("AMF Instance Down Notification from NRF: %v", nfId)
	req := httpwrapper.NewRequest(c.Request, nil)
	if nfInstanceId, exists := c.Params.Get("nfid"); exists {
		req.Params["nfid"] = nfInstanceId
		c.JSON(http.StatusOK, nil)
	}
}
