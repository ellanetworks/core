package server

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Tracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(
		serviceName,
		otelgin.WithSpanNameFormatter(func(c *gin.Context) string {
			// Name spans like "GET /api/v1/subscribers"
			return fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
		}),
	)
}
