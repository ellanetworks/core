// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing_test

import (
	"context"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/tracing"
	"go.opentelemetry.io/otel"
)

func TestInitTracerPropagatesOnlyTraceContext(t *testing.T) {
	tp, err := tracing.InitTracer(t.Context(), tracing.TelemetryConfig{
		OTLPEndpoint: "127.0.0.1:4317",
		ServiceName:  "ella-core",
	})
	if err != nil {
		t.Fatalf("InitTracer: %v", err)
	}

	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	fields := otel.GetTextMapPropagator().Fields()
	slices.Sort(fields)

	if !slices.Equal(fields, []string{"traceparent", "tracestate"}) {
		t.Fatalf("propagated fields = %v, want [traceparent tracestate]", fields)
	}
}
