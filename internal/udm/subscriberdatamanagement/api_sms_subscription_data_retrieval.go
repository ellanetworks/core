package subscriberdatamanagement

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetSmsData - retrieve a UE's SMS Subscription Data
func HTTPGetSmsData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}
