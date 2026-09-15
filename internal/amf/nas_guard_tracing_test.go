// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNASGuardLinksTimerSpanToArmingSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	orig := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)

	defer otel.SetTracerProvider(orig)

	tracer = otel.Tracer("ella-core/amf")

	conn := &UeConn{}
	cfg := guard.TimerValue{Enable: true, ExpireTime: 5 * time.Millisecond, MaxRetryTimes: 1}

	armCtx, armSpan := tracer.Start(t.Context(), "nas/handle_registration_request")
	armSC := armSpan.SpanContext()

	fired := make(chan context.Context, 4)

	conn.armNASGuardWith(armCtx, cfg, "T3560 (test)",
		func(ctx context.Context, _ int32) { fired <- ctx },
		func(ctx context.Context) { fired <- ctx },
	)

	armSpan.End()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("guard never fired")
	}

	retx := awaitSpan(t, exp, "amf/nas_guard_retransmit")

	conn.StopNASGuard(t.Context())

	if retx.Parent.IsValid() {
		t.Error("guard span should be a root")
	}

	if retx.SpanContext.TraceID() == armSC.TraceID() {
		t.Error("guard span should be in its own trace")
	}

	if len(retx.Links) != 1 || retx.Links[0].SpanContext.SpanID() != armSC.SpanID() {
		t.Errorf("guard span should link to the arming span, links=%d", len(retx.Links))
	}

	var timer string

	for _, a := range retx.Attributes {
		if a.Key == "nas.guard.timer" {
			timer = a.Value.AsString()
		}
	}

	if timer != "T3560 (test)" {
		t.Errorf("nas.guard.timer = %q", timer)
	}
}

func awaitSpan(t *testing.T, exp *tracetest.InMemoryExporter, name string) *tracetest.SpanStub {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		var found *tracetest.SpanStub

		for _, s := range exp.GetSpans() {
			if s.Name == name {
				c := s
				found = &c
			}
		}

		if found != nil {
			return found
		}

		if time.Now().After(deadline) {
			t.Fatalf("no %s span; got %d spans", name, len(exp.GetSpans()))
		}

		time.Sleep(time.Millisecond)
	}
}
