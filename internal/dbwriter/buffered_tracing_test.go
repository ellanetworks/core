// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package dbwriter_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/dbwriter"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
)

func TestBufferedDBWriter_LinksWriteSpanToProducer(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	orig := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)

	defer otel.SetTracerProvider(orig)

	fake := &fakeDBWriter{}
	buf := dbwriter.NewBufferedDBWriter(fake, 10, zap.NewNop())

	pctx, producer := otel.Tracer("test").Start(t.Context(), "s1ap/receive")
	producerSC := producer.SpanContext()

	_ = buf.InsertRadioEvent(pctx, &dbwriter.RadioEvent{MessageType: "InitialUEMessage"})

	producer.End()
	buf.Stop(context.Background())

	var writeSpan *tracetest.SpanStub

	spans := exp.GetSpans()
	for i := range spans {
		if spans[i].Name == "dbwriter/insert_radio_event" {
			writeSpan = &spans[i]
		}
	}

	if writeSpan == nil {
		t.Fatalf("no write span, got %d spans", len(spans))
	}

	if writeSpan.Parent.IsValid() {
		t.Error("write span should be a root, not a child of the producer")
	}

	if writeSpan.SpanContext.TraceID() == producerSC.TraceID() {
		t.Error("write span should be in its own trace")
	}

	if len(writeSpan.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(writeSpan.Links))
	}

	if writeSpan.Links[0].SpanContext.SpanID() != producerSC.SpanID() {
		t.Error("link does not point at the producer span")
	}
}

func TestBufferedDBWriter_NoSpanWhenProducerUntraced(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	orig := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)

	defer otel.SetTracerProvider(orig)

	fake := &fakeDBWriter{}
	buf := dbwriter.NewBufferedDBWriter(fake, 10, zap.NewNop())

	_ = buf.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{MessageType: "NGSetupRequest"})
	buf.Stop(context.Background())

	if n := len(exp.GetSpans()); n != 0 {
		t.Fatalf("expected no span for an untraced producer, got %d", n)
	}

	if fake.radioEventCount() != 1 {
		t.Fatal("event was not written")
	}
}
