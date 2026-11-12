// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/pki"
	"github.com/prometheus/client_golang/prometheus"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	SEPPCATableName         = "sepp_ca"
	SEPPIdentitiesTableName = "sepp_identities"
)

const (
	SEPPIdentityGenerated = "generated"
	SEPPIdentityImported  = "imported"

	MinSEPPID = 1
	MaxSEPPID = MaxAMFPointer
)

const (
	getSEPPCASQL                 = "SELECT certPEM, keyPEM FROM " + SEPPCATableName + " WHERE singleton=TRUE"
	insertSEPPCAIfAbsentSQL      = "INSERT INTO " + SEPPCATableName + " (singleton, certPEM, keyPEM) VALUES (TRUE, ?, ?) ON CONFLICT(singleton) DO NOTHING"
	upsertSEPPCASQL              = "INSERT INTO " + SEPPCATableName + " (singleton, certPEM, keyPEM) VALUES (TRUE, ?, ?) ON CONFLICT(singleton) DO UPDATE SET certPEM=excluded.certPEM, keyPEM=excluded.keyPEM"
	deleteSEPPCASQL              = "DELETE FROM " + SEPPCATableName
	getSEPPIdentitySQL           = "SELECT seppID, fqdn, source, certChainPEM, keyPEM FROM " + SEPPIdentitiesTableName + " WHERE seppID==?"
	listSEPPIdentitiesSQL        = "SELECT seppID, fqdn, source, certChainPEM, keyPEM FROM " + SEPPIdentitiesTableName + " ORDER BY seppID"
	upsertSEPPIdentitySQL        = "INSERT INTO " + SEPPIdentitiesTableName + " (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (?, ?, ?, ?, ?) ON CONFLICT(seppID) DO UPDATE SET fqdn=excluded.fqdn, source=excluded.source, certChainPEM=excluded.certChainPEM, keyPEM=excluded.keyPEM"
	deleteSEPPIdentitiesSQL      = "DELETE FROM " + SEPPIdentitiesTableName
	deleteGeneratedSEPPIdentsSQL = "DELETE FROM " + SEPPIdentitiesTableName + " WHERE source='" + SEPPIdentityGenerated + "'"
	deleteGeneratedSEPPIdentSQL  = "DELETE FROM " + SEPPIdentitiesTableName + " WHERE seppID==? AND source='" + SEPPIdentityGenerated + "'"
	getSEPPIdentityByFQDNSQL     = "SELECT seppID, fqdn, source, certChainPEM, keyPEM FROM " + SEPPIdentitiesTableName + " WHERE fqdn==?"
	deleteSEPPIdentitySQL        = "DELETE FROM " + SEPPIdentitiesTableName + " WHERE seppID==?"
	upsertGeneratedSEPPIdentSQL  = "INSERT INTO " + SEPPIdentitiesTableName + " (seppID, fqdn, source, certChainPEM, keyPEM) VALUES (?, ?, ?, ?, ?) ON CONFLICT(seppID) DO UPDATE SET fqdn=excluded.fqdn, certChainPEM=excluded.certChainPEM, keyPEM=excluded.keyPEM WHERE source='" + SEPPIdentityGenerated + "'"
)

type SEPPCA struct {
	CertPEM string
	KeyPEM  string
}

func (c SEPPCA) Validate() error {
	if _, _, err := pki.ValidateSEPPCA([]byte(c.CertPEM), []byte(c.KeyPEM)); err != nil {
		return err
	}

	return nil
}

func (c SEPPCA) Certificate() (*x509.Certificate, error) {
	return pki.ParseCertPEM([]byte(c.CertPEM))
}

type SEPPIdentity struct {
	SEPPID       int
	FQDN         string
	Source       string
	CertChainPEM string
	KeyPEM       string
}

func ValidSEPPID(id int) bool {
	return id >= MinSEPPID && id <= MaxSEPPID
}

func (s SEPPIdentity) Validate() error {
	if !ValidSEPPID(s.SEPPID) {
		return fmt.Errorf("SEPP ID must be between %d and %d, got %d", MinSEPPID, MaxSEPPID, s.SEPPID)
	}

	if s.Source != SEPPIdentityGenerated && s.Source != SEPPIdentityImported {
		return fmt.Errorf("SEPP identity source must be %s or %s, got %q", SEPPIdentityGenerated, SEPPIdentityImported, s.Source)
	}

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

func (db *Database) ownPLMN(ctx context.Context) (pki.PLMN, error) {
	op, err := getOperator(ctx, db.runner(ctx))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return pki.PLMN{}, fmt.Errorf("query failed: %w", err)
	}

	return pki.PLMN{MCC: op.Mcc, MNC: op.Mnc}, nil
}

func (db *Database) checkSEPPIdentity(ctx context.Context, s *SEPPIdentity) error {
	plmn, err := db.ownPLMN(ctx)
	if err != nil {
		return err
	}

	if !strings.HasSuffix(s.FQDN, "."+plmn.Domain()) {
		return fmt.Errorf("%w: SEPP FQDN %s is not under the home PLMN domain", ErrRoamingConflict, s.FQDN)
	}

	other, err := scanSEPPIdentity(db.runner(ctx).QueryRowContext(ctx, getSEPPIdentityByFQDNSQL, s.FQDN))

	switch {
	case err == nil && other.SEPPID != s.SEPPID:
		return fmt.Errorf("%w: SEPP FQDN %s is already the identity of SEPP %d", ErrRoamingConflict, s.FQDN, other.SEPPID)
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("query failed: %w", err)
	}

	if s.Source != SEPPIdentityGenerated {
		return nil
	}

	if s.FQDN != plmn.SEPPFQDN(s.SEPPID) {
		return fmt.Errorf("%w: a generated SEPP identity must use %s", ErrRoamingConflict, plmn.SEPPFQDN(s.SEPPID))
	}

	ca, err := scanSEPPCA(db.runner(ctx).QueryRowContext(ctx, getSEPPCASQL))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: no SEPP CA", ErrRoamingConflict)
		}

		return fmt.Errorf("query failed: %w", err)
	}

	caCert, err := ca.Certificate()
	if err != nil {
		return fmt.Errorf("stored SEPP CA: %w", err)
	}

	chain, err := pki.ParseCertChainPEM([]byte(s.CertChainPEM))
	if err != nil {
		return err
	}

	if !chain[len(chain)-1].Equal(caCert) {
		return fmt.Errorf("%w: the generated SEPP certificate was not issued by the current SEPP CA", ErrRoamingConflict)
	}

	return nil
}

func (db *Database) GetSEPPCA(ctx context.Context) (*SEPPCA, error) {
	ctx, span := startSEPPIdentitySpan(ctx, "SELECT", SEPPCATableName)
	defer span.End()

	DBQueriesTotal.WithLabelValues(SEPPCATableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	ca, err := scanSEPPCA(db.reader().QueryRowContext(ctx, getSEPPCASQL))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &ca, nil
}

func (db *Database) GetSEPPIdentity(ctx context.Context, seppID int) (*SEPPIdentity, error) {
	ctx, span := startSEPPIdentitySpan(ctx, "SELECT", SEPPIdentitiesTableName)
	defer span.End()

	timer := prometheus.NewTimer(DBQueryDuration.WithLabelValues(SEPPIdentitiesTableName, "select"))
	defer timer.ObserveDuration()

	DBQueriesTotal.WithLabelValues(SEPPIdentitiesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, ErrNotFound
	}

	identity, err := scanSEPPIdentity(db.reader().QueryRowContext(ctx, getSEPPIdentitySQL, seppID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return &identity, nil
}

func (db *Database) ListSEPPIdentities(ctx context.Context) ([]SEPPIdentity, error) {
	ctx, span := startSEPPIdentitySpan(ctx, "SELECT", SEPPIdentitiesTableName)
	defer span.End()

	DBQueriesTotal.WithLabelValues(SEPPIdentitiesTableName, "select").Inc()

	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return nil, nil
	}

	identities, err := queryAll(ctx, db.reader(), listSEPPIdentitiesSQL, scanSEPPIdentity)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("query failed: %w", err)
	}

	return identities, nil
}

func (db *Database) PutSEPPIdentity(ctx context.Context, identity *SEPPIdentity) error {
	if err := identity.Validate(); err != nil {
		return err
	}

	ctx, span := startSEPPIdentitySpan(ctx, "UPSERT", SEPPIdentitiesTableName)
	defer span.End()

	DBQueriesTotal.WithLabelValues(SEPPIdentitiesTableName, "upsert").Inc()

	if _, err := opPutSEPPIdentity.Invoke(ctx, db, identity); err != nil {
		recordSpanError(span, err)
		return err
	}

	return nil
}

func (db *Database) applyPutSEPPIdentity(ctx context.Context, s *SEPPIdentity) (any, error) {
	if err := db.checkSEPPIdentity(ctx, s); err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, upsertSEPPIdentitySQL, s.SEPPID, s.FQDN, s.Source, s.CertChainPEM, s.KeyPEM); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyPutGeneratedSEPPIdentity(ctx context.Context, s *SEPPIdentity) (any, error) {
	if s.Source != SEPPIdentityGenerated {
		return nil, fmt.Errorf("a generated SEPP identity must have source %q, got %q", SEPPIdentityGenerated, s.Source)
	}

	if err := db.checkSEPPIdentity(ctx, s); err != nil {
		return nil, err
	}

	if _, err := db.runner(ctx).ExecContext(ctx, upsertGeneratedSEPPIdentSQL, s.SEPPID, s.FQDN, s.Source, s.CertChainPEM, s.KeyPEM); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyDeleteSEPPIdentity(ctx context.Context, s *SEPPIdentity) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteSEPPIdentitySQL, s.SEPPID); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return nil, nil
}

func (db *Database) applyInitSEPPCA(ctx context.Context, ca *SEPPCA) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, insertSEPPCAIfAbsentSQL, ca.CertPEM, ca.KeyPEM)
	if err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}

	return n == 1, nil
}

func (db *Database) applyReplaceSEPPCA(ctx context.Context, ca *SEPPCA) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, upsertSEPPCASQL, ca.CertPEM, ca.KeyPEM); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if _, err := db.runner(ctx).ExecContext(ctx, deleteGeneratedSEPPIdentsSQL); err != nil {
		return nil, fmt.Errorf("delete generated SEPP identities: %w", err)
	}

	return nil, nil
}

func (db *Database) deleteGeneratedSEPPIdentity(ctx context.Context, seppID int) error {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteGeneratedSEPPIdentSQL, seppID); err != nil {
		return fmt.Errorf("delete the generated SEPP identity %d: %w", seppID, err)
	}

	return nil
}

func (db *Database) deleteSEPPIdentities(ctx context.Context) (bool, error) {
	deleted := false

	for _, query := range []string{deleteSEPPIdentitiesSQL, deleteSEPPCASQL} {
		res, err := db.runner(ctx).ExecContext(ctx, query)
		if err != nil {
			return false, fmt.Errorf("query failed: %w", err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return false, fmt.Errorf("rows affected: %w", err)
		}

		deleted = deleted || n > 0
	}

	return deleted, nil
}

func (db *Database) newSEPPCA(ctx context.Context) (*SEPPCA, error) {
	plmn, err := db.ownPLMN(ctx)
	if err != nil {
		return nil, err
	}

	cert, key, err := pki.GenerateSEPPCA(plmn)
	if err != nil {
		return nil, err
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		return nil, err
	}

	return &SEPPCA{CertPEM: string(pki.EncodeCertPEM(cert)), KeyPEM: string(keyPEM)}, nil
}

func (db *Database) RegenerateSEPPCA(ctx context.Context) (*SEPPCA, error) {
	ca, err := db.newSEPPCA(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := opReplaceSEPPCA.Invoke(ctx, db, ca); err != nil {
		return nil, err
	}

	return ca, nil
}

func (db *Database) ensureSEPPCA(ctx context.Context) (*SEPPCA, error) {
	ca, err := db.GetSEPPCA(ctx)
	if err == nil || !errors.Is(err, ErrNotFound) {
		return ca, err
	}

	fresh, err := db.newSEPPCA(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := opInitSEPPCA.Invoke(ctx, db, fresh); err != nil {
		return nil, err
	}

	return db.GetSEPPCA(ctx)
}

func (db *Database) issueSEPPIdentity(ctx context.Context, seppID int) (*SEPPIdentity, error) {
	if !ValidSEPPID(seppID) {
		return nil, fmt.Errorf("SEPP ID must be between %d and %d, got %d", MinSEPPID, MaxSEPPID, seppID)
	}

	ca, err := db.ensureSEPPCA(ctx)
	if err != nil {
		return nil, fmt.Errorf("SEPP CA: %w", err)
	}

	caCert, caKey, err := pki.ValidateSEPPCA([]byte(ca.CertPEM), []byte(ca.KeyPEM))
	if err != nil {
		return nil, fmt.Errorf("stored SEPP CA: %w", err)
	}

	plmn, err := db.ownPLMN(ctx)
	if err != nil {
		return nil, err
	}

	fqdn := plmn.SEPPFQDN(seppID)

	leaf, key, err := pki.IssueSEPPCert(fqdn, plmn, caCert, caKey)
	if err != nil {
		return nil, err
	}

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		return nil, err
	}

	chain := pki.SEPPCertificate{Chain: []*x509.Certificate{leaf, caCert}, FQDN: fqdn}

	return &SEPPIdentity{SEPPID: seppID, FQDN: fqdn, Source: SEPPIdentityGenerated, CertChainPEM: chain.ChainPEM(), KeyPEM: string(keyPEM)}, nil
}

var (
	opPutGeneratedSEPPIdentity = registerChangesetOp("PutGeneratedSEPPIdentity", (*Database).applyPutGeneratedSEPPIdentity, RequireSchema(roamingSchema), AffectsTopic(TopicSEPPIdentity))
	opDeleteSEPPIdentity       = registerChangesetOp("DeleteSEPPIdentity", (*Database).applyDeleteSEPPIdentity, RequireSchema(roamingSchema), AffectsTopic(TopicSEPPIdentity))
)

func (db *Database) PutGeneratedSEPPIdentity(ctx context.Context, seppID int) error {
	identity, err := db.issueSEPPIdentity(ctx, seppID)
	if err != nil {
		return err
	}

	if err := identity.Validate(); err != nil {
		return err
	}

	_, err = opPutGeneratedSEPPIdentity.Invoke(ctx, db, identity)

	return err
}

func (db *Database) DeleteSEPPIdentity(ctx context.Context, seppID int) error {
	if _, err := db.GetSEPPIdentity(ctx, seppID); err != nil {
		return err
	}

	_, err := opDeleteSEPPIdentity.Invoke(ctx, db, &SEPPIdentity{SEPPID: seppID})

	return err
}

func startSEPPIdentitySpan(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	querySummary := fmt.Sprintf("%s %s", operation, table)

	return tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName(operation),
			semconv.DBCollectionName(table),
		),
	)
}

func scanSEPPIdentity(row scanner) (SEPPIdentity, error) {
	var v SEPPIdentity

	if err := row.Scan(&v.SEPPID, &v.FQDN, &v.Source, &v.CertChainPEM, &v.KeyPEM); err != nil {
		return SEPPIdentity{}, err
	}

	return v, nil
}

func scanSEPPCA(row scanner) (SEPPCA, error) {
	var v SEPPCA

	if err := row.Scan(&v.CertPEM, &v.KeyPEM); err != nil {
		return SEPPCA{}, err
	}

	return v, nil
}
