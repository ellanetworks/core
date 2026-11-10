// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"context"
	"crypto/rand"

	"go.opentelemetry.io/otel/trace"
)

type pinnedTraceIDKey struct{}

func WithPinnedTraceID(ctx context.Context, id RANTraceID) context.Context {
	if id.IsZero() {
		return ctx
	}

	return context.WithValue(ctx, pinnedTraceIDKey{}, id.OTelTraceID())
}

func PinnedTraceID(ctx context.Context) (trace.TraceID, bool) {
	id, ok := ctx.Value(pinnedTraceIDKey{}).(trace.TraceID)

	return id, ok && id.IsValid()
}

type pinnedIDGenerator struct{}

func (pinnedIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	if id, ok := PinnedTraceID(ctx); ok {
		return id, newSpanID()
	}

	var tid trace.TraceID

	for !tid.IsValid() {
		_, _ = rand.Read(tid[:])
	}

	return tid, newSpanID()
}

func (pinnedIDGenerator) NewSpanID(context.Context, trace.TraceID) trace.SpanID {
	return newSpanID()
}

func newSpanID() trace.SpanID {
	var sid trace.SpanID

	for !sid.IsValid() {
		_, _ = rand.Read(sid[:])
	}

	return sid
}
