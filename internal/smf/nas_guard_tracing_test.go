// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestRetransmitGuardLinksTimerSpanToArmingSpan(t *testing.T) {
	sc := &SMContext{Ref: "ref-1"}
	s := &SMF{t3591: 5 * time.Millisecond, pool: map[string]*SMContext{"ref-1": sc}}

	before := len(testSpanRecorder.Ended())

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

	retx := awaitGuardSpan(t, before, "smf/nas_guard_retransmit", "T3591")

	sc.procedureTimer.Stop()

	if retx.Parent().IsValid() {
		t.Error("guard span should be a root")
	}

	if retx.SpanContext().TraceID() == armSC.TraceID() {
		t.Error("guard span should be in its own trace")
	}

	if links := retx.Links(); len(links) != 1 || links[0].SpanContext.SpanID() != armSC.SpanID() {
		t.Errorf("guard span should link to the arming span, links=%d", len(links))
	}
}

func awaitGuardSpan(t *testing.T, before int, name string, timer string) sdktrace.ReadOnlySpan {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for {
		ended := testSpanRecorder.Ended()

		for _, s := range ended[before:] {
			if s.Name() != name {
				continue
			}

			for _, a := range s.Attributes() {
				if a.Key == "nas.guard.timer" && a.Value.AsString() == timer {
					return s
				}
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("no %s span for %s; got %d spans", name, timer, len(ended)-before)
		}

		time.Sleep(time.Millisecond)
	}
}
