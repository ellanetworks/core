// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/tracing"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestEndRecordsOnlyFailures(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)).Tracer("test")

	_, ok := tracer.Start(context.Background(), "ok")
	tracing.End(ok, nil)

	_, failed := tracer.Start(context.Background(), "failed")
	tracing.End(failed, errors.New("no S-CSCF"))

	spans := rec.Ended()
	if len(spans) != 2 {
		t.Fatalf("ended %d spans, want 2", len(spans))
	}

	if s := spans[0]; s.Status().Code != codes.Unset || len(s.Attributes()) != 0 {
		t.Errorf("successful span status %v, attributes %v", s.Status(), s.Attributes())
	}

	if s := spans[1]; s.Status().Code != codes.Error || s.Status().Description != "no S-CSCF" || len(s.Attributes()) != 1 || s.Attributes()[0].Key != "error.type" {
		t.Errorf("failed span status %v, attributes %v", s.Status(), s.Attributes())
	}
}
