// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNASGuardLinksTimerSpanToArmingSpan(t *testing.T) {
	conn := &UeConn{}
	cfg := guard.TimerValue{Enable: true, ExpireTime: 5 * time.Millisecond, MaxRetryTimes: 1}

	before := len(testSpanRecorder.Ended())

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

	retx := awaitGuardSpan(t, before, "amf/nas_guard_retransmit", "T3560 (test)")

	conn.StopNASGuard(t.Context())

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
