// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

var errorTypes = []struct {
	err  error
	name string
}{
	{ErrNotFound, "not_found"},
	{sql.ErrNoRows, "not_found"},
	{ErrAlreadyExists, "already_exists"},
	{ErrInUse, "in_use"},
	{ErrMSISDNInUse, "msisdn_in_use"},
	{ErrDataNetworkNotFound, "data_network_not_found"},
	{ErrNoMatchingPolicy, "no_matching_policy"},
	{ErrDNNNotInSlice, "dnn_not_in_slice"},
	{ErrSubscriberLimit, "subscriber_limit"},
	{ErrPSIConflict, "psi_conflict"},
	{ErrIMSPeerConflict, "ims_peer_conflict"},
	{ErrSMSCConflict, "smsc_conflict"},
	{ErrRoamingConflict, "roaming_conflict"},
	{ErrRoamingRefused, "roaming_refused"},
	{ErrRoamerDeregistering, "roamer_deregistering"},
	{ErrRestoreInProgress, "restore_in_progress"},
	{ErrInvalidBackupFile, "invalid_backup_file"},
	{ErrRestoreBackupAhead, "restore_backup_ahead"},
	{ErrProposeTimeout, "propose_timeout"},
	{ErrOutcomeUnknown, "outcome_unknown"},
	{ErrMigrationPending, "migration_pending"},
	{ErrSchemaNewerThanBinary, "schema_newer_than_binary"},
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

var outcomes = []error{
	ErrNotFound, sql.ErrNoRows, ErrAlreadyExists, ErrInUse, ErrMSISDNInUse, ErrDataNetworkNotFound,
	ErrNoMatchingPolicy, ErrDNNNotInSlice, ErrSubscriberLimit, ErrPSIConflict, ErrIMSPeerConflict,
	ErrSMSCConflict, ErrRoamingConflict, ErrRoamingRefused, ErrRoamerDeregistering, ErrInvalidBackupFile,
	ErrRestoreBackupAhead, ErrJoinTokenAlreadyConsumed, ErrJoinTokenNodeMismatch, ErrNodeIdentityBound,
	ErrJoinTokenExpired, context.Canceled,
}

func failed(err error) bool {
	return err != nil && !slices.ContainsFunc(outcomes, func(o error) bool { return errors.Is(err, o) })
}

func errorType(err error) string {
	var se sqlite3.Error
	if errors.As(err, &se) {
		return strconv.Itoa(int(se.ExtendedCode))
	}

	for _, t := range errorTypes {
		if errors.Is(err, t.err) {
			return t.name
		}
	}

	return "_OTHER"
}

func recordSpanError(span trace.Span, err error) {
	span.SetAttributes(spanErrorAttributes(err)...)
	span.SetStatus(codes.Error, err.Error())
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		recordSpanError(span, err)
	}

	span.End()
}

func spanErrorAttributes(err error) []attribute.KeyValue {
	attrs := []attribute.KeyValue{semconv.ErrorTypeKey.String(errorType(err))}

	var se sqlite3.Error
	if errors.As(err, &se) {
		attrs = append(attrs, semconv.DBResponseStatusCode(strconv.Itoa(int(se.ExtendedCode))))
	}

	return attrs
}

func startDBQuery(ctx context.Context, table, operation string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	return startDBQueryNamed(ctx, operation+" "+table, table, operation, attrs...)
}

func startDBQueryNamed(ctx context.Context, summary, table, operation string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	ctx, span := tracer.Start(
		ctx,
		summary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(append([]attribute.KeyValue{
			semconv.DBQuerySummary(summary),
			semconv.DBSystemNameSQLite,
			semconv.DBNamespace(dbNamespace),
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(table),
		}, attrs...)...),
	)

	start := time.Now()
	label := operationLabel(operation)

	return ctx, func(err error) {
		errType := ""

		if failed(err) {
			errType = errorType(err)
			recordSpanError(span, err)
		}

		DBQueriesTotal.WithLabelValues(table, label, errType).Inc()
		DBQueryDuration.WithLabelValues(table, label).Observe(time.Since(start).Seconds())

		span.End()
	}
}

const dbNamespace = "main"

func operationLabel(operation string) string {
	return strings.ReplaceAll(strings.ToLower(operation), " ", "_")
}
