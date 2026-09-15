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

// From returns base enriched with the request-scoped fields carried by ctx, any
// extra fields the call site adds, and the trace and span IDs of the active
// span. base itself — its sink and its component name — is never substituted,
// so a helper that names its own destination keeps it whatever the caller put
// in the context. extra comes after the context's own fields, so a call site
// naming a specific connection wins over an ambient one.
func From(ctx context.Context, base *zap.Logger, extra ...zap.Field) *zap.Logger {
	fields := Fields(ctx)

	sc := trace.SpanFromContext(ctx).SpanContext()

	n := len(fields) + len(extra)
	if sc.IsValid() {
		n += 2
	}

	if n == 0 {
		return base
	}

	out := make([]zap.Field, 0, n)
	out = append(out, fields...)
	out = append(out, extra...)

	if sc.IsValid() {
		out = append(out,
			zap.String("trace_id", sc.TraceID().String()),
			zap.String("span_id", sc.SpanID().String()),
		)
	}

	return base.With(out...)
}

// Enabled reports whether a record at lvl would be emitted. Hot paths guard a
// Debug statement with it so the logger and its fields are never built for a
// record that is dropped.
func Enabled(lvl zapcore.Level) bool {
	return atomicLevel.Enabled(lvl)
}
