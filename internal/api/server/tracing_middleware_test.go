// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
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

func useTestTracing(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()

	originalProvider := otel.GetTracerProvider()
	originalPropagator := otel.GetTextMapPropagator()

	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		otel.SetTracerProvider(originalProvider)
		otel.SetTextMapPropagator(originalPropagator)
	})

	return exporter
}

func requestWithParent(t *testing.T, method, path string) (*http.Request, trace.SpanContext) {
	t.Helper()

	ctx, parent := sdktrace.NewTracerProvider().Tracer("test").Start(t.Context(), "peer")
	defer parent.End()

	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(req.Header))

	return req, parent.SpanContext()
}

func TestClusterTracingMiddlewareContinuesPeerTrace(t *testing.T) {
	exporter := useTestTracing(t)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /cluster/internal/propose", func(w http.ResponseWriter, r *http.Request) {})

	req, parent := requestWithParent(t, http.MethodPost, "/cluster/internal/propose")

	server.ClusterTracingMiddleware(mux).ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].SpanContext.TraceID() != parent.TraceID() || spans[0].Parent.SpanID() != parent.SpanID() {
		t.Fatal("cluster server span should be a child of the calling peer's span")
	}

	if spans[0].Name != "POST /cluster/internal/propose" {
		t.Fatalf("expected span name %q, got %q", "POST /cluster/internal/propose", spans[0].Name)
	}
}

func TestClusterTracingMiddlewareSkipsStatusProbes(t *testing.T) {
	exporter := useTestTracing(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /cluster/status", func(w http.ResponseWriter, r *http.Request) {})

	server.ClusterTracingMiddleware(mux).ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/cluster/status", nil))

	if spans := exporter.GetSpans(); len(spans) != 0 {
		t.Fatalf("expected no spans for status probes, got %d", len(spans))
	}
}

func TestPublicTracingMiddlewareLinksInsteadOfParenting(t *testing.T) {
	exporter := useTestTracing(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", func(w http.ResponseWriter, r *http.Request) {})

	req, parent := requestWithParent(t, http.MethodGet, "/api/v1/status")

	server.PublicTracingMiddleware("ella-core/api", mux).ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Parent.IsValid() {
		t.Fatal("public API span should start a new trace")
	}

	if len(spans[0].Links) != 1 || spans[0].Links[0].SpanContext.SpanID() != parent.SpanID() {
		t.Fatal("public API span should link to the caller's span")
	}
}
