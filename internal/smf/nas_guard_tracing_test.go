// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRetransmitGuardLinksTimerSpanToArmingSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	orig := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)

	defer otel.SetTracerProvider(orig)

	tracer = otel.Tracer("ella-core/smf/session")

	sc := &SMContext{Ref: "ref-1"}
	s := &SMF{t3591: 5 * time.Millisecond, pool: map[string]*SMContext{"ref-1": sc}}

	armCtx, armSpan := tracer.Start(t.Context(), "smf/handle_pdu_session_modify")
	armSC := armSpan.SpanContext()

	fired := make(chan context.Context, 4)

	s.armRetransmit(armCtx, sc, s.timerT3591(),
		func(ctx context.Context) error { fired <- ctx; return nil },
		func(ctx context.Context, _ *SMContext) { fired <- ctx })

	armSpan.End()

	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("guard never fired")
	}

	sc.procedureTimer.Stop()

	var retx *tracetest.SpanStub

	for _, sp := range exp.GetSpans() {
		if sp.Name == "smf/nas_guard_retransmit" {
			c := sp
			retx = &c
		}
	}

	if retx == nil {
		t.Fatalf("no retransmit span; got %d spans", len(exp.GetSpans()))
	}

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

	if timer != "T3591" {
		t.Errorf("nas.guard.timer = %q, want T3591", timer)
	}
}
