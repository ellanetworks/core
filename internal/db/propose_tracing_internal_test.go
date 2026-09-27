// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var (
	testSpanExporterOnce sync.Once
	testSpanExporter     *tracetest.InMemoryExporter
)

func recordTestSpans(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	testSpanExporterOnce.Do(func() {
		testSpanExporter = tracetest.NewInMemoryExporter()
		otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSyncer(testSpanExporter)))
	})

	t.Cleanup(testSpanExporter.Reset)

	return testSpanExporter
}

func spansInTrace(exporter *tracetest.InMemoryExporter, traceID trace.TraceID) tracetest.SpanStubs {
	var out tracetest.SpanStubs

	for _, s := range exporter.GetSpans() {
		if s.SpanContext.TraceID() == traceID {
			out = append(out, s)
		}
	}

	return out
}

func requireSpan(t *testing.T, spans tracetest.SpanStubs, name string) tracetest.SpanStub {
	t.Helper()

	for _, s := range spans {
		if s.Name == name {
			return s
		}
	}

	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}

	t.Fatalf("span %q not found; got %v", name, names)

	return tracetest.SpanStub{}
}

func requireChildOf(t *testing.T, child, parent tracetest.SpanStub) {
	t.Helper()

	if child.Parent.SpanID() != parent.SpanContext.SpanID() {
		t.Fatalf("span %q should be a child of %q", child.Name, parent.Name)
	}
}

func requireAttr(t *testing.T, s tracetest.SpanStub, key, want string) {
	t.Helper()

	for _, a := range s.Attributes {
		if string(a.Key) == key {
			if got := a.Value.String(); got != want {
				t.Fatalf("span %q attribute %s = %q, want %q", s.Name, key, got, want)
			}

			return
		}
	}

	t.Fatalf("span %q has no attribute %s", s.Name, key)
}

func TestProposeSpansNestUnderCaller(t *testing.T) {
	exporter := recordTestSpans(t)
	database := newStandaloneDB(t)

	t.Run("changeset op", func(t *testing.T) {
		ctx, caller := otel.Tracer("test").Start(t.Context(), "test/caller")

		if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 30}); err != nil {
			t.Fatalf("SetRetentionPolicy: %v", err)
		}

		caller.End()

		spans := spansInTrace(exporter, caller.SpanContext().TraceID())

		write := requireSpan(t, spans, "UPSERT retention_policies")
		propose := requireSpan(t, spans, "db/propose")

		requireChildOf(t, propose, write)
		requireAttr(t, propose, "ella.raft.operation", "SetRetentionPolicy")

		for _, name := range []string{"db/propose_lock_wait", "db/write_barrier", "db/capture_changeset", "db/raft_apply"} {
			requireChildOf(t, requireSpan(t, spans, name), propose)
		}
	})

	t.Run("forwarded intent op", func(t *testing.T) {
		ctx, caller := otel.Tracer("test").Start(t.Context(), "test/caller")

		payload, err := json.Marshal(int64Payload{Value: time.Now().Unix()})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}

		if _, err := database.ApplyForwardedOperation(ctx, "DeleteExpiredSessions", payload); err != nil {
			t.Fatalf("ApplyForwardedOperation: %v", err)
		}

		caller.End()

		spans := spansInTrace(exporter, caller.SpanContext().TraceID())

		propose := requireSpan(t, spans, "db/propose")

		requireChildOf(t, propose, requireSpan(t, spans, "test/caller"))
		requireAttr(t, propose, "ella.raft.operation", "DeleteExpiredSessions")
		requireChildOf(t, requireSpan(t, spans, "db/propose_lock_wait"), propose)
		requireChildOf(t, requireSpan(t, spans, "db/raft_apply"), propose)
	})
}
