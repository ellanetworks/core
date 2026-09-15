// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package attrs_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/tracing/attrs"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestIdentityAttributesAreOmittedWhenUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		attr attribute.KeyValue
	}{
		{"malformed IMSI", attrs.SUPIFromIMSI("not-an-imsi")},
		{"empty IMSI", attrs.SUPIFromIMSI("")},
		{"unset SUPI", attrs.SUPI("")},
		{"unset SUCI", attrs.SUCI("")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.attr.Valid() {
				t.Fatalf("expected no attribute, got %s=%q", tc.attr.Key, tc.attr.Value.AsString())
			}
		})
	}
}

func TestMalformedIMSILeavesNoSupiOnTheSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	_, span := tp.Tracer("test").Start(t.Context(), "db/get_session_policy",
		trace.WithAttributes(attrs.SUPIFromIMSI("not-an-imsi"), attrs.DNN("internet")))
	span.End()

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	for _, a := range spans[0].Attributes {
		if a.Key == "ue.supi" {
			t.Errorf("a malformed IMSI stamped ue.supi=%q on the span", a.Value.AsString())
		}
	}
}
