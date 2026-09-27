// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// TracingMiddleware wraps the handler in OpenTelemetry HTTP middleware
func TracingMiddleware(serviceName string, handler http.Handler, opts ...otelhttp.Option) http.Handler {
	return otelhttp.NewHandler(
		routeAttributeMiddleware(handler),
		"",
		append([]otelhttp.Option{otelhttp.WithServerName(serviceName)}, opts...)...,
	)
}

func ClusterTracingMiddleware(handler http.Handler) http.Handler {
	return TracingMiddleware("ella-core/cluster", handler, otelhttp.WithFilter(func(r *http.Request) bool {
		return r.Method != http.MethodGet || r.URL.Path != "/cluster/status"
	}))
}

func PublicTracingMiddleware(serviceName string, handler http.Handler) http.Handler {
	return TracingMiddleware(serviceName, handler, otelhttp.WithPublicEndpointFn(func(*http.Request) bool { return true }))
}

func routeAttributeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		if i := strings.IndexByte(r.Pattern, '/'); i >= 0 {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(r.Pattern[i:]))
		}
	})
}
