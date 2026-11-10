// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"context"
	"errors"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var testPLMN = [3]byte{0x02, 0xf8, 0x39}

func TestAcquireSharesSessionPerSubscriber(t *testing.T) {
	s := NewTraceSessions(TraceConfig{}, func() int { return 5 })

	a, err := s.Acquire("001010000000001", testPLMN)
	if err != nil {
		t.Fatal(err)
	}

	b, err := s.Acquire("001010000000001", testPLMN)
	if err != nil {
		t.Fatal(err)
	}

	if a != b {
		t.Fatal("two sessions for one subscriber")
	}

	c, err := s.Acquire("001010000000002", testPLMN)
	if err != nil {
		t.Fatal(err)
	}

	if c.TraceReference() == a.TraceReference() {
		t.Fatal("two subscribers share a Trace Reference")
	}

	tr := a.TraceReference()
	if [3]byte(tr[:3]) != testPLMN {
		t.Fatalf("Trace Reference %x does not start with the PLMN", tr)
	}

	if tr[3]>>2 != 5 {
		t.Fatalf("Trace ID %x does not carry the node prefix", tr[3:])
	}
}

func TestReleaseFreesOnLastReference(t *testing.T) {
	s := NewTraceSessions(TraceConfig{}, func() int { return 1 })

	a, _ := s.Acquire("001010000000001", testPLMN)
	_, _ = s.Acquire("001010000000001", testPLMN)

	s.Release(a)

	if s.Active() != 1 {
		t.Fatalf("active %d after the first release, want 1", s.Active())
	}

	s.Release(a)

	if s.Active() != 0 {
		t.Fatalf("active %d after the last release, want 0", s.Active())
	}

	s.Release(a)

	b, _ := s.Acquire("001010000000001", testPLMN)
	if b == a {
		t.Fatal("released session reused")
	}
}

func TestAllocationExhaustion(t *testing.T) {
	s := NewTraceSessions(TraceConfig{}, func() int { return 1 })

	for v := range uint32(traceIDValueMask + 1) {
		s.inUse[v] = struct{}{}
	}

	if _, err := s.Acquire("001010000000001", testPLMN); !errors.Is(err, ErrTraceIDsExhausted) {
		t.Fatalf("err %v, want ErrTraceIDsExhausted", err)
	}
}

func TestRecordingSessionReference(t *testing.T) {
	s := NewTraceSessions(TraceConfig{}, func() int { return 1 })

	var amf, mme UETrace

	if _, err := amf.Activate(s, "001010000000001", testPLMN); err != nil {
		t.Fatal(err)
	}

	if _, err := mme.Activate(s, "001010000000001", testPLMN); err != nil {
		t.Fatal(err)
	}

	first, started, ok := amf.StartRecording()
	if !started || !ok {
		t.Fatal("first start did not start a recording")
	}

	if again, started, _ := amf.StartRecording(); again != first || started {
		t.Fatalf("start during an active recording changed the TRSR: %x then %x", first, again)
	}

	other, started, _ := mme.StartRecording()
	if !started || other == first || [6]byte(other[:6]) != [6]byte(first[:6]) {
		t.Fatalf("second NE recording %x, first %x: want the same Trace Reference and a distinct TRSR", other, first)
	}

	amf.StopRecording()

	if _, recording := amf.Recording(); recording {
		t.Fatal("recording after stop")
	}

	if _, recording := mme.Recording(); !recording {
		t.Fatal("stopping one NE stopped the other")
	}

	second, _, _ := amf.StartRecording()
	if second == first || second == other {
		t.Fatalf("new recording %x reused a TRSR", second)
	}

	amf.Release()
	mme.Release()

	if s.Active() != 0 {
		t.Fatalf("active %d after both NEs released", s.Active())
	}

	if _, _, ok := amf.StartRecording(); ok {
		t.Fatal("recording after release")
	}
}

func TestConnTrace(t *testing.T) {
	var source, target ConnTrace

	if _, ok := source.ID(); ok {
		t.Fatal("zero ConnTrace has an ID")
	}

	first := RANTraceID{1, 2, 3, 4, 5, 6, 0, 1}
	second := RANTraceID{1, 2, 3, 4, 5, 6, 0, 2}

	source.Activate(first)
	source.Activate(second)

	if id, _ := source.ID(); id != first {
		t.Fatalf("activated connection took a later ID: %x", id)
	}

	target.Inherit(&source)

	if id, _ := target.ID(); id != first || !target.Activated() {
		t.Fatalf("inherited %x activated=%v", id, target.Activated())
	}

	target.Fail()
	target.Activate(second)

	if id, _ := target.ID(); id != second {
		t.Fatalf("failed connection kept %x, want %x", id, second)
	}

	if got, ok := PinnedTraceID(target.Context(context.Background())); !ok || got != second.OTelTraceID() {
		t.Fatalf("pinned %s", got)
	}
}

func TestContinueRANTrace(t *testing.T) {
	id := RANTraceID{1, 2, 3, 4, 5, 6, 0, 1}.OTelTraceID()
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: id, SpanID: trace.SpanID{1}})

	if got, ok := PinnedTraceID(ContinueRANTrace(context.Background(), sc)); !ok || got != id {
		t.Fatalf("RAN trace not continued: %s", got)
	}

	random := trace.NewSpanContext(trace.SpanContextConfig{TraceID: trace.TraceID{9, 1}, SpanID: trace.SpanID{1}})

	if _, ok := PinnedTraceID(ContinueRANTrace(context.Background(), random)); ok {
		t.Fatal("random trace pinned")
	}
}

func TestOTelTraceIDMapping(t *testing.T) {
	id := RANTraceID{0x02, 0xf8, 0x39, 0x0a, 0x0b, 0x0c, 0x12, 0x34}

	got := id.OTelTraceID().String()
	if got != "000000000000000002f8390a0b0c1234" {
		t.Fatalf("trace ID %s", got)
	}
}

func TestPinnedTraceIDBecomesRootTraceID(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec), sdktrace.WithIDGenerator(pinnedIDGenerator{}))
	tracer := tp.Tracer("test")

	id := RANTraceID{0x02, 0xf8, 0x39, 0x0a, 0x0b, 0x0c, 0x12, 0x34}

	ctx, root := tracer.Start(WithPinnedTraceID(context.Background(), id), "root")
	_, child := tracer.Start(ctx, "child")
	child.End()
	root.End()

	_, other := tracer.Start(context.Background(), "other")
	other.End()

	spans := rec.Ended()
	if len(spans) != 3 {
		t.Fatalf("%d spans", len(spans))
	}

	want := id.OTelTraceID()

	if spans[0].SpanContext().TraceID() != want || spans[1].SpanContext().TraceID() != want {
		t.Fatalf("pinned spans in traces %s and %s, want %s", spans[0].SpanContext().TraceID(), spans[1].SpanContext().TraceID(), want)
	}

	if spans[2].SpanContext().TraceID() == want {
		t.Fatal("unpinned span reused the pinned trace ID")
	}
}

func TestZeroIDIsNotPinned(t *testing.T) {
	if _, ok := PinnedTraceID(WithPinnedTraceID(context.Background(), RANTraceID{})); ok {
		t.Fatal("zero RAN trace ID pinned")
	}
}
