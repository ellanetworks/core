// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
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
		otel.SetTextMapPropagator(propagation.TraceContext{})
	})

	t.Cleanup(testSpanExporter.Reset)

	return testSpanExporter
}

func findSpan(t *testing.T, spans tracetest.SpanStubs, match func(tracetest.SpanStub) bool, what string) tracetest.SpanStub {
	t.Helper()

	for _, s := range spans {
		if match(s) {
			return s
		}
	}

	names := make([]string, 0, len(spans))
	for _, s := range spans {
		names = append(names, s.Name)
	}

	t.Fatalf("%s not found; got %v", what, names)

	return tracetest.SpanStub{}
}

func TestForwardPropose_PropagatesTraceContext(t *testing.T) {
	exporter := recordTestSpans(t)

	tc := SetupTestClusterWithAppliers(t, 3, func() Applier { return newTestApplier(t) })

	tc.WireHandlers([]string{listener.ALPNHTTP}, func(_ int, m *Manager, ln *listener.Listener) {
		sm := http.NewServeMux()
		sm.Handle("POST "+ProposeForwardPath, miniProposeHandler(m))

		startTestClusterHTTP(t, ln, otelhttp.NewHandler(sm, ""))
	})

	var follower *Manager

	for _, m := range tc.Nodes {
		if !m.IsLeader() {
			follower = m
			break
		}
	}

	if follower == nil {
		t.Fatal("no follower")
	}

	payload, err := json.Marshal(map[string]string{"via": "follower"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	ctx, caller := otel.Tracer("test").Start(context.Background(), "test/caller")

	if _, err := follower.ForwardOperation(ctx, "TestOp", payload, 5*time.Second); err != nil {
		t.Fatalf("follower.ForwardOperation: %v", err)
	}

	caller.End()

	var spans tracetest.SpanStubs

	for _, s := range exporter.GetSpans() {
		if s.SpanContext.TraceID() == caller.SpanContext().TraceID() {
			spans = append(spans, s)
		}
	}

	forward := findSpan(t, spans, func(s tracetest.SpanStub) bool { return s.Name == "raft/forward" }, "raft/forward span")
	client := findSpan(t, spans, func(s tracetest.SpanStub) bool { return s.SpanKind == trace.SpanKindClient }, "client span")
	server := findSpan(t, spans, func(s tracetest.SpanStub) bool { return s.SpanKind == trace.SpanKindServer }, "leader server span")
	wait := findSpan(t, spans, func(s tracetest.SpanStub) bool { return s.Name == "raft/wait_local_apply" }, "raft/wait_local_apply span")

	if forward.Parent.SpanID() != caller.SpanContext().SpanID() {
		t.Fatal("raft/forward should be a child of the caller")
	}

	if client.Name != "POST "+ProposeForwardPath {
		t.Fatalf("client span name = %q, want %q", client.Name, "POST "+ProposeForwardPath)
	}

	if client.Parent.SpanID() != forward.SpanContext.SpanID() {
		t.Fatal("client span should be a child of raft/forward")
	}

	if server.Parent.SpanID() != client.SpanContext.SpanID() {
		t.Fatal("leader server span should continue the follower's client span")
	}

	if wait.Parent.SpanID() != forward.SpanContext.SpanID() {
		t.Fatal("raft/wait_local_apply should be a child of raft/forward")
	}
}
