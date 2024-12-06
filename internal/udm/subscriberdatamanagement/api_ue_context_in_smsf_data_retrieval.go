package subscriberdatamanagement

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetUeContextInSmsfData - retrieve a UE's UE Context In SMSF Data
func HTTPGetUeContextInSmsfData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}
