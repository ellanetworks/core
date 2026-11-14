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
	s := NewTraceSessions(func() int { return 5 })

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
	s := NewTraceSessions(func() int { return 1 })

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
	s := NewTraceSessions(func() int { return 1 })

	for v := range uint32(traceIDValueMask + 1) {
		s.inUse[v] = struct{}{}
	}

	if _, err := s.Acquire("001010000000001", testPLMN); !errors.Is(err, ErrTraceIDsExhausted) {
		t.Fatalf("err %v, want ErrTraceIDsExhausted", err)
	}
}

func TestRecordingSessionReference(t *testing.T) {
	s := NewTraceSessions(func() int { return 1 })

	var amf, mme UETrace

	if _, err := amf.Activate(s, "001010000000001", testPLMN); err != nil {
		t.Fatal(err)
	}

	if _, err := mme.Activate(s, "001010000000001", testPLMN); err != nil {
		t.Fatal(err)
	}

	first, ok := amf.NewRecording()
	if !ok {
		t.Fatal("no recording on an active session")
	}

	second, _ := mme.NewRecording()
	third, _ := amf.NewRecording()

	if [6]byte(first[:6]) != [6]byte(second[:6]) || [6]byte(first[:6]) != [6]byte(third[:6]) {
		t.Fatalf("recordings %x %x %x do not share the Trace Reference", first, second, third)
	}

	if first == second || first == third || second == third {
		t.Fatalf("recordings %x %x %x reuse a TRSR", first, second, third)
	}

	amf.Release()
	mme.Release()

	if s.Active() != 0 {
		t.Fatalf("active %d after both NEs released", s.Active())
	}

	if _, ok := amf.NewRecording(); ok {
		t.Fatal("recording after release")
	}
}

func TestTakeOverKeepsTraceReference(t *testing.T) {
	s := NewTraceSessions(func() int { return 1 })

	var old, fresh UETrace

	if _, err := old.Activate(s, "001010000000001", testPLMN); err != nil {
		t.Fatal(err)
	}

	before, _ := old.NewRecording()

	fresh.TakeOver(&old)
	old.Release()

	after, ok := fresh.NewRecording()
	if !ok || [6]byte(after[:6]) != [6]byte(before[:6]) {
		t.Fatalf("Trace Reference changed from %x to %x", before[:6], after[:6])
	}

	if s.Active() != 1 {
		t.Fatalf("active %d after the take-over", s.Active())
	}

	var other UETrace

	if _, err := other.Activate(s, "001010000000001", testPLMN); err != nil {
		t.Fatal(err)
	}

	fresh.TakeOver(&other)

	fresh.Release()

	if s.Active() != 0 {
		t.Fatalf("active %d: a take-over onto a held session leaked a reference", s.Active())
	}
}

func TestAcquireRefusesWithoutNodePrefix(t *testing.T) {
	s := NewTraceSessions(func() int { return 0 })

	if _, err := s.Acquire("001010000000001", testPLMN); !errors.Is(err, ErrNoNodePrefix) {
		t.Fatalf("err %v, want ErrNoNodePrefix", err)
	}
}

func TestConnTrace(t *testing.T) {
	var source, target ConnTrace

	if _, ok := source.ID(); ok {
		t.Fatal("zero ConnTrace has an ID")
	}

	first := RANTraceID{1, 2, 3, 4, 5, 6, 0, 1}
	second := RANTraceID{1, 2, 3, 4, 5, 6, 0, 2}

	source.Assign(first)
	source.Assign(second)

	if id, pending := source.Pending(); id != first || !pending {
		t.Fatalf("pending %x %v, want %x true", id, pending, first)
	}

	source.MarkActivated(second)

	if source.Activated() {
		t.Fatal("activated by an ID the connection does not carry")
	}

	source.MarkActivated(first)

	if _, pending := source.Pending(); pending {
		t.Fatal("still pending after activation")
	}

	target.Inherit(&source)

	if id, _ := target.ID(); id != first || !target.Activated() {
		t.Fatalf("inherited %x activated=%v", id, target.Activated())
	}

	if target.Fail(second) {
		t.Fatal("failure for another ID changed the connection")
	}

	if !target.Fail(first) {
		t.Fatal("failure for the connection's ID ignored")
	}

	if id, pending := target.Pending(); id != first || !pending {
		t.Fatalf("after failure pending %x %v, want %x true", id, pending, first)
	}

	if got, ok := PinnedTraceID(target.Context(context.Background())); !ok || got != first.OTelTraceID() {
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
