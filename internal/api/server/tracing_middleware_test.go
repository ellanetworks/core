// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingMiddlewareNamesSpansByRoute(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	original := otel.GetTracerProvider()

	otel.SetTracerProvider(provider)

	defer otel.SetTracerProvider(original)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT /api/v1/users/{email}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {})

	handler := server.TracingMiddleware("ella-core/api", mux)

	tests := []struct {
		name          string
		method        string
		path          string
		expectedSpan  string
		expectedRoute string
	}{
		{
			name:          "dynamic segment replaced by its placeholder",
			method:        http.MethodPut,
			path:          "/api/v1/users/someone@example.com",
			expectedSpan:  "PUT /api/v1/users/{email}",
			expectedRoute: "/api/v1/users/{email}",
		},
		{
			name:          "catch-all route",
			method:        http.MethodGet,
			path:          "/assets/index-abc123.js",
			expectedSpan:  "GET /",
			expectedRoute: "/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exporter.Reset()

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil))

			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("expected 1 span, got %d", len(spans))
			}

			if spans[0].Name != tc.expectedSpan {
				t.Errorf("expected span name %q, got %q", tc.expectedSpan, spans[0].Name)
			}

			route := ""

			for _, attr := range spans[0].Attributes {
				if attr.Key == "http.route" {
					route = attr.Value.AsString()
				}
			}

			if route != tc.expectedRoute {
				t.Errorf("expected http.route %q, got %q", tc.expectedRoute, route)
			}
		})
	}
}
