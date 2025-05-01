package server

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Tracing returns a Gin middleware that instruments every request.
// serviceName should match what you configured in main.go.
func Tracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
