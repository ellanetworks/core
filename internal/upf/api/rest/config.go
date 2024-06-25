package rest

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/ella/internal/upf/config"
	"github.com/yeastengine/ella/internal/upf/core"
)

// DisplayConfig godoc
//
//	@Summary		Display configuration
//	@Description	Display configuration
//	@Tags			Configuration
//	@Produce		json
//	@Success		200	{object}	config.UpfConfig
//	@Router			/config [get]
func (h *ApiHandler) displayConfig(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, *h.Cfg)
}

func (h *ApiHandler) editConfig(c *gin.Context) {
	var conf config.UpfConfig
	if err := c.BindJSON(&conf); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{
			"message":       "Request body json has incorrect format. Use one or more fields from the following structure",
			"correctFormat": config.UpfConfig{},
		})
		return
	}

	*h.Cfg = conf

	if err := core.SetConfig(conf); err != nil {
		c.IndentedJSON(http.StatusBadRequest, gin.H{
			"message": fmt.Sprintf("Error during editing config: %s", err.Error()),
		})
	} else {
		c.Status(http.StatusOK)
	}
}
