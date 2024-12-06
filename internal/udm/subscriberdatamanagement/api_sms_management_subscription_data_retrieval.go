package subscriberdatamanagement

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetSmsMngData - retrieve a UE's SMS Management Subscription Data
func HTTPGetSmsMngData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{})
}
