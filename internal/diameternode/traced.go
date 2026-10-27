// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameternode

import (
	"context"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func Traced(tracer trace.Tracer, name string, h func(context.Context, diameter.Identity, *diameter.Message) *diameter.Message) diameter.Handler {
	return TracedFrom(tracer, name, func(ctx context.Context, id diameter.Identity, _ string, req *diameter.Message) *diameter.Message {
		return h(ctx, id, req)
	})
}

func TracedFrom(tracer trace.Tracer, name string, h func(context.Context, diameter.Identity, string, *diameter.Message) *diameter.Message) diameter.Handler {
	return diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		ctx, span := tracer.Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("network.protocol.name", "diameter"),
				attribute.Int64("diameter.application_id", int64(req.ApplicationID)),
				attribute.Int64("diameter.command_code", int64(req.CommandCode)),
			),
		)
		defer span.End()

		ans := h(ctx, c.LocalIdentity(), c.PeerHost(), req)
		if ans == nil {
			return nil
		}

		if r, err := tgpp.ParseResult(ans); err == nil {
			span.SetAttributes(attribute.Int64("diameter.result_code", int64(r.Code)))
		}

		return ans
	})
}
