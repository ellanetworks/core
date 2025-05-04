package server

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// Tracing returns a Gin middleware that instruments every request.
// serviceName should match what you configured in main.go.
func Tracing(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(
		serviceName,
		otelgin.WithSpanNameFormatter(func(r *http.Request) string {
			// Name spans like "GET /api/v1/subscribers"
			// (falls back to the actual request URL path)
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
	)
}
