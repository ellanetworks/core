package server

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// TracingMiddleware wraps the handler in OpenTelemetry HTTP middleware
func TracingMiddleware(serviceName string, handler http.Handler) http.Handler {
	return otelhttp.NewHandler(
		handler,
		"", // leave span name empty so formatter is used
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			path := r.URL.Path
			// strip dynamic segments for better grouping, e.g., "/users/{email}"
			path = strings.ReplaceAll(path, "/{", "/:")

			return r.Method + " " + path
		}),
		otelhttp.WithServerName(serviceName),
	)
}
