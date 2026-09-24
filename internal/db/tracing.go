// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"strconv"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

var spanErrorTypes = []struct {
	err  error
	name string
}{
	{ErrNotFound, "not_found"},
	{ErrAlreadyExists, "already_exists"},
	{ErrDataNetworkNotFound, "data_network_not_found"},
	{ErrNoMatchingPolicy, "no_matching_policy"},
	{ErrDNNNotInSlice, "dnn_not_in_slice"},
	{ErrRestoreInProgress, "restore_in_progress"},
	{ErrInvalidBackupFile, "invalid_backup_file"},
	{ErrProposeTimeout, "propose_timeout"},
	{ErrOutcomeUnknown, "outcome_unknown"},
	{ErrMigrationPending, "migration_pending"},
	{ErrJoinTokenAlreadyConsumed, "join_token_already_consumed"},
	{ErrJoinTokenNodeMismatch, "join_token_node_mismatch"},
	{ErrNodeIdentityBound, "node_identity_bound"},
	{ErrJoinTokenExpired, "join_token_expired"},
	{ErrUnknownOperation, "unknown_operation"},
	{ErrRetiredOperation, "retired_operation"},
	{ellaraft.ErrLeaderRequestNotSent, "leader_request_not_sent"},
	{ellaraft.ErrLeaderUnreachable, "leader_unreachable"},
	{ellaraft.ErrBarrierTimeout, "barrier_timeout"},
	{hraft.ErrNotLeader, "not_leader"},
	{hraft.ErrLeadershipLost, "leadership_lost"},
	{context.DeadlineExceeded, "timeout"},
	{context.Canceled, "canceled"},
}

func recordSpanError(span trace.Span, err error) {
	span.SetAttributes(spanErrorAttributes(err)...)
	span.SetStatus(codes.Error, err.Error())
}

func spanErrorAttributes(err error) []attribute.KeyValue {
	var se sqlite3.Error
	if errors.As(err, &se) {
		code := strconv.Itoa(int(se.ExtendedCode))

		return []attribute.KeyValue{
			semconv.DBResponseStatusCode(code),
			semconv.ErrorTypeKey.String(code),
		}
	}

	for _, t := range spanErrorTypes {
		if errors.Is(err, t.err) {
			return []attribute.KeyValue{semconv.ErrorTypeKey.String(t.name)}
		}
	}

	return []attribute.KeyValue{semconv.ErrorType(err)}
}
