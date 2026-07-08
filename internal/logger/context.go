// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"context"

	"go.uber.org/zap"
)

type loggerCtxKey struct{}

// Into carries a connection-scoped logger for From to read back. Inject it at
// message ingress keyed by the connection's temporary identity (AMF-UE-NGAP-ID /
// MME-UE-S1AP-ID) so logs correlate by temporary identity rather than SUPI/IMSI
// (TS 33.501 §6.12.3, TS 33.401 §7.1).
func Into(ctx context.Context, l *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// From returns the logger stored by Into (or base when none is set), enriched
// with the trace and span IDs from ctx. Handlers log through this instead of a
// per-UE logger field, which would go stale when the UE context outlives the
// connection whose identity it captured.
func From(ctx context.Context, base *zap.Logger) *zap.Logger {
	l := base
	if v, ok := ctx.Value(loggerCtxKey{}).(*zap.Logger); ok && v != nil {
		l = v
	}

	return WithTrace(ctx, l)
}
