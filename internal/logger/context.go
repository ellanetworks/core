// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type fieldsCtxKey struct{}

// Into carries request-scoped fields for From to read back. Inject it at
// message ingress. Fields accumulate in call order, and a record keeps only the
// last field named for a key, so a nested Into refines what an outer one set.
func Into(ctx context.Context, fields ...zap.Field) context.Context {
	kept := fields[:0:0]

	for _, f := range fields {
		if f.Type == zapcore.SkipType || f.Key == "" {
			continue
		}

		kept = append(kept, f)
	}

	if len(kept) == 0 {
		return ctx
	}

	prev := Fields(ctx)

	merged := make([]zap.Field, 0, len(prev)+len(kept))
	merged = append(merged, prev...)
	merged = append(merged, kept...)

	return context.WithValue(ctx, fieldsCtxKey{}, merged)
}

// Fields returns the request-scoped fields carried by ctx, or nil.
func Fields(ctx context.Context) []zap.Field {
	f, _ := ctx.Value(fieldsCtxKey{}).([]zap.Field)

	return f
}

// From returns base enriched with the request-scoped fields carried by ctx and
// with the trace and span IDs of the active span. base itself — its sink and
// its component name — is never substituted, so a helper that names its own
// destination keeps it whatever the caller put in the context.
func From(ctx context.Context, base *zap.Logger) *zap.Logger {
	fields := Fields(ctx)

	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		if len(fields) == 0 {
			return base
		}

		return base.With(fields...)
	}

	out := make([]zap.Field, 0, len(fields)+2)
	out = append(out, fields...)
	out = append(out,
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	)

	return base.With(out...)
}
