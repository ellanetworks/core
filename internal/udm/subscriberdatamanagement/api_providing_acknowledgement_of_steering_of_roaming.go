package subscriberdatamanagement

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Info - Nudm_Sdm Info service operation
func HTTPInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}
