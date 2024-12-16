package server

import (
	"github.com/gin-gonic/gin"
)

func AddApiService(engine *gin.Engine) *gin.RouterGroup {
	group := engine.Group("/api/v1")
	return group
}
