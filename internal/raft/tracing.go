// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"

	hraft "github.com/hashicorp/raft"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/raft")

var spanErrorTypes = []struct {
	err  error
	name string
}{
	{ErrLeaderRequestNotSent, "leader_request_not_sent"},
	{ErrLeaderUnreachable, "leader_unreachable"},
	{ErrOutcomeUnknown, "outcome_unknown"},
	{hraft.ErrNotLeader, "not_leader"},
	{context.DeadlineExceeded, "timeout"},
	{context.Canceled, "canceled"},
}

func recordSpanError(span trace.Span, err error) {
	span.SetAttributes(spanErrorType(err))
	span.SetStatus(codes.Error, err.Error())
}

func spanErrorType(err error) attribute.KeyValue {
	if code := ForwardErrorCode(err); code != "" {
		return semconv.ErrorTypeKey.String(code)
	}

	for _, t := range spanErrorTypes {
		if errors.Is(err, t.err) {
			return semconv.ErrorTypeKey.String(t.name)
		}
	}

	return semconv.ErrorType(err)
}
