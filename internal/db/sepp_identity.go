// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/canonical/sqlair"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const SEPPIdentityTableName = "sepp_identity"

const (
	getSEPPIdentityStmt            = "SELECT &SEPPIdentity.* FROM %s WHERE singleton=TRUE"
	insertSEPPIdentityIfAbsentStmt = "INSERT INTO %s (singleton, fqdn, certChainPEM, keyPEM) VALUES (TRUE, $SEPPIdentity.fqdn, $SEPPIdentity.certChainPEM, $SEPPIdentity.keyPEM) ON CONFLICT(singleton) DO NOTHING"
	deleteSEPPIdentityStmt         = "DELETE FROM %s WHERE singleton=TRUE"
	upsertSEPPIdentityStmt         = "INSERT INTO %s (singleton, fqdn, certChainPEM, keyPEM) VALUES (TRUE, $SEPPIdentity.fqdn, $SEPPIdentity.certChainPEM, $SEPPIdentity.keyPEM) ON CONFLICT(singleton) DO UPDATE SET fqdn=excluded.fqdn, certChainPEM=excluded.certChainPEM, keyPEM=excluded.keyPEM"
)

type SEPPIdentity struct {
	FQDN         string `db:"fqdn"`
	CertChainPEM string `db:"certChainPEM"`
	KeyPEM       string `db:"keyPEM"`
}

func (s SEPPIdentity) Validate() error {
	if strings.TrimSpace(s.CertChainPEM) == "" {
		return errors.New("SEPP certificate chain is required")
	}

	if strings.TrimSpace(s.KeyPEM) == "" {
		return errors.New("SEPP private key is required")
	}

	if s.FQDN == "" || s.FQDN != strings.ToLower(s.FQDN) {
		return fmt.Errorf("SEPP FQDN must be a lowercase name, got %q", s.FQDN)
	}

	if _, err := pki.ValidateSEPPIdentity([]byte(s.CertChainPEM), []byte(s.KeyPEM), s.FQDN); err != nil {
		return fmt.Errorf("SEPP identity: %w", err)
	}

	return nil
}

func (db *Database) checkSEPPIdentityPLMN(ctx context.Context, s *SEPPIdentity) error {
	var op Operator

	if err := db.runner(ctx).Query(ctx, db.getOperatorStmt).Get(&op); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("query failed: %w", err)
	}

	if !strings.HasSuffix(s.FQDN, "."+pki.PLMN{MCC: op.Mcc, MNC: op.Mnc}.Domain()) {
		return fmt.Errorf("%w: SEPP FQDN %s is not under the home PLMN domain", ErrRoamingConflict, s.FQDN)
	}

	return nil
}

func (db *Database) GetSEPPIdentity(ctx context.Context) (*SEPPIdentity, error) {
	ctx, span := startSEPPIdentitySpan(ctx, "SELECT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SEPPIdentityTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SEPPIdentityTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	var identity SEPPIdentity

	if err := db.conn().Query(ctx, db.getSEPPIdentityStmt).Get(&identity); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &identity, nil
}

func (db *Database) PutSEPPIdentity(ctx context.Context, identity *SEPPIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}

	ctx, span := startSEPPIdentitySpan(ctx, "UPSERT")
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SEPPIdentityTableName, "upsert"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SEPPIdentityTableName, "upsert").Inc()

	if _, err := opPutSEPPIdentity.Invoke(ctx, db, identity); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyPutSEPPIdentity(ctx context.Context, s *SEPPIdentity) (any, error) {
	if err := db.checkSEPPIdentityPLMN(ctx, s); err != nil {
		return nil, err
	}

	if err := db.runner(ctx).Query(ctx, db.upsertSEPPIdentityStmt, s).Run(); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) deleteSEPPIdentity(ctx context.Context) (bool, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.deleteSEPPIdentityStmt).Get(&outcome); err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}

	return rowsAffected > 0, nil
}

func (db *Database) insertSEPPIdentityIfAbsent(ctx context.Context, s *SEPPIdentity) (bool, error) {
	if s == nil {
		return false, nil
	}

	if err := db.checkSEPPIdentityPLMN(ctx, s); err != nil {
		return false, err
	}

	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.insertSEPPIdentityIfAbsentStmt, s).Get(&outcome); err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	rowsAffected, err := outcome.Result().RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}

	return rowsAffected == 1, nil
}

func startSEPPIdentitySpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, SEPPIdentityTableName)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(SEPPIdentityTableName),
		),
	)
}
