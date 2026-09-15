// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/nas/eps"
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

	Tracer = otel.Tracer("ella-core/mme")

	m := newTestMME(t)
	m.nasGuardCfg.ExpireTime = 5 * time.Millisecond
	m.nasGuardCfg.MaxRetryTimes = 2

	ue, cc := securedUE(t, m)

	armCtx, armSpan := Tracer.Start(t.Context(), "nas/handle_attach_request")
	armSC := armSpan.SpanContext()

	ue.Conn().ArmNASGuard(armCtx, "Authentication Request", []byte{0x07, 0x52}, eps.SHTIntegrityProtectedCiphered)
	armSpan.End()

	eventually(t, time.Second, func() bool { return cc.count() >= 3 })

	var retx *tracetest.SpanStub

	for _, s := range exp.GetSpans() {
		if s.Name == "mme/nas_guard_retransmit" {
			c := s
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
}
