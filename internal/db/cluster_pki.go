// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// SPDX-FileCopyrightText: Ella Networks Inc.

// Cluster PKI storage. Cluster TLS trusts a peer when the peer's
// leaf SHA-256 matches a row in cluster_node_certs; removing a node
// from the cluster deletes its row. The join-token registry
// authorises a node's first contact (when no pin yet exists) using
// an HMAC key from the cluster_join_hmac singleton.

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	ClusterNodeCertsTableName  = "cluster_node_certs"
	ClusterJoinTokensTableName = "cluster_join_tokens"
	ClusterJoinHMACTableName   = "cluster_join_hmac"
)

// Names of tables dropped by migration v12. Referenced only by the
// migration's DROP TABLE statements.
const (
	ClusterPKIRootsTableName         = "cluster_pki_roots"
	ClusterPKIIntermediatesTableName = "cluster_pki_intermediates"
	ClusterIssuedCertsTableName      = "cluster_issued_certs"
	ClusterRevokedCertsTableName     = "cluster_revoked_certs"
	ClusterPKIStateTableName         = "cluster_pki_state"
)

const (
	listNodeCertsStmt        = "SELECT nodeID, fingerprint, certPEM, addedAt FROM " + ClusterNodeCertsTableName + " ORDER BY nodeID ASC"
	getNodeCertByFPStmt      = "SELECT nodeID, fingerprint, certPEM, addedAt FROM " + ClusterNodeCertsTableName + " WHERE fingerprint=?"
	getNodeCertByNodeStmt    = "SELECT nodeID, fingerprint, certPEM, addedAt FROM " + ClusterNodeCertsTableName + " WHERE nodeID=?"
	upsertNodeCertStmt       = "INSERT INTO " + ClusterNodeCertsTableName + " (nodeID, fingerprint, certPEM, addedAt) VALUES (?, ?, ?, ?) ON CONFLICT(nodeID) DO UPDATE SET fingerprint=excluded.fingerprint, certPEM=excluded.certPEM, addedAt=excluded.addedAt"
	deleteNodeCertByNodeStmt = "DELETE FROM " + ClusterNodeCertsTableName + " WHERE nodeID=?"

	insertJoinTokenStmt       = "INSERT INTO " + ClusterJoinTokensTableName + " (id, claimsJSON, expiresAt, consumedAt, consumedBy) VALUES (?, ?, ?, 0, '')"           // #nosec G101 -- SQL statement
	insertJoinTokenPreV20Stmt = "INSERT INTO " + ClusterJoinTokensTableName + " (id, nodeID, claimsJSON, expiresAt, consumedAt, consumedBy) VALUES (?, 0, ?, ?, 0, 0)" // #nosec G101 -- SQL statement
	getJoinTokenStmt          = "SELECT id, claimsJSON, expiresAt, consumedAt, consumedBy FROM " + ClusterJoinTokensTableName + " WHERE id=?"
	consumeJoinTokenStmt      = "UPDATE " + ClusterJoinTokensTableName + " SET consumedAt=?, consumedBy=? WHERE id=? AND consumedAt=0" // #nosec G101 -- SQL statement
	deleteJoinTokensStaleStmt = "DELETE FROM " + ClusterJoinTokensTableName + " WHERE expiresAt<? OR (consumedAt>0 AND consumedAt<?)"  // #nosec G101 -- SQL statement

	getJoinHMACStmt  = "SELECT id, hmacKey FROM " + ClusterJoinHMACTableName + " WHERE id=1"
	initJoinHMACStmt = "INSERT INTO " + ClusterJoinHMACTableName + " (id, hmacKey) VALUES (1, ?) ON CONFLICT(id) DO NOTHING"
)

// ClusterNodeCert pins one cluster member's self-signed leaf by
// SHA-256. One row per nodeID. Inserted by the join flow; removed by
// RemoveClusterMember.
type ClusterNodeCert struct {
	NodeID      string
	Fingerprint string
	CertPEM     string
	AddedAt     int64
}

// ClusterJoinToken is a row in cluster_join_tokens. The HMAC tag is
// embedded in the token string the admin copies to the joining node;
// this row only records the token's identity and consumption state so
// replays against a different voter fail.
type ClusterJoinToken struct {
	ID         string
	ClaimsJSON string
	ExpiresAt  int64
	ConsumedAt int64
	ConsumedBy string
}

// ClusterJoinHMAC is the singleton row in cluster_join_hmac.
type ClusterJoinHMAC struct {
	ID      int
	HMACKey []byte
}

// ---------------------------------------------------------------------------
// Apply methods — invoked from the typed-op dispatch layer.
// ---------------------------------------------------------------------------

func (db *Database) applyUpsertNodeCert(ctx context.Context, r *ClusterNodeCert) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, upsertNodeCertStmt, r.NodeID, r.Fingerprint, r.CertPEM, r.AddedAt); err != nil {
		return nil, err
	}

	logger.DBLog.Debug("applied cluster_node_certs upsert",
		zap.String("node_id", r.NodeID),
		zap.String("fingerprint", r.Fingerprint))

	return nil, nil
}

func (db *Database) applyDeleteNodeCert(ctx context.Context, r *ClusterNodeCert) (any, error) {
	if _, err := db.runner(ctx).ExecContext(ctx, deleteNodeCertByNodeStmt, r.NodeID); err != nil {
		return nil, err
	}

	logger.DBLog.Debug("applied cluster_node_certs delete",
		zap.String("node_id", r.NodeID))

	return nil, nil
}

func (db *Database) applyInsertJoinToken(ctx context.Context, r *ClusterJoinToken) (any, error) {
	query := insertJoinTokenStmt
	if !db.appliedSchemaAtLeast(ctx, clusterMemberIdentitySchema) {
		query = insertJoinTokenPreV20Stmt
	}

	_, err := db.runner(ctx).ExecContext(ctx, query, r.ID, r.ClaimsJSON, r.ExpiresAt)

	return nil, err
}

func (db *Database) applyConsumeJoinToken(ctx context.Context, r *ClusterJoinToken) (any, error) {
	res, err := db.runner(ctx).ExecContext(ctx, consumeJoinTokenStmt, r.ConsumedAt, r.ConsumedBy, r.ID)
	if err != nil {
		return nil, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return nil, ErrJoinTokenAlreadyConsumed
	}

	return nil, nil
}

type redeemJoinTokenPayload struct {
	TokenID     string     `json:"token_id"`
	NodeID      pki.NodeID `json:"node_id"`
	Fingerprint string     `json:"fingerprint"`
	CertPEM     string     `json:"cert_pem"`
}

type RedeemJoinTokenResult struct {
	Pins []ClusterNodeCert `json:"pins"`
}

func (db *Database) applyRedeemJoinToken(ctx context.Context, p *redeemJoinTokenPayload) (any, error) {
	if err := db.requireIdentitySchemaFor(ctx, string(p.NodeID)); err != nil {
		return nil, err
	}

	runner := db.runner(ctx)
	now := time.Now().Unix()

	token, err := scanClusterJoinToken(runner.QueryRowContext(ctx, getJoinTokenStmt, p.TokenID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get join token: %w", err)
	}

	if token.ExpiresAt < now-int64(pki.JoinTokenClockSkew.Seconds()) {
		return nil, ErrJoinTokenExpired
	}

	if token.ConsumedAt != 0 {
		if token.ConsumedBy != string(p.NodeID) {
			return nil, ErrJoinTokenAlreadyConsumed
		}

		existing, err := scanClusterNodeCert(runner.QueryRowContext(ctx, getNodeCertByNodeStmt, string(p.NodeID)))
		if err != nil {
			return nil, ErrJoinTokenAlreadyConsumed
		}

		if existing.Fingerprint != p.Fingerprint {
			return nil, ErrJoinTokenAlreadyConsumed
		}

		return db.pinSnapshot(ctx, runner)
	}

	if bound, err := scanClusterNodeCert(runner.QueryRowContext(ctx, getNodeCertByNodeStmt, string(p.NodeID))); err == nil && bound.Fingerprint != p.Fingerprint {
		return nil, ErrNodeIdentityBound
	}

	token.ConsumedAt = now
	token.ConsumedBy = string(p.NodeID)

	res, err := runner.ExecContext(ctx, consumeJoinTokenStmt, token.ConsumedAt, token.ConsumedBy, token.ID)
	if err != nil {
		return nil, fmt.Errorf("consume join token: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return nil, ErrJoinTokenAlreadyConsumed
	}

	cert := ClusterNodeCert{
		NodeID:      string(p.NodeID),
		Fingerprint: p.Fingerprint,
		CertPEM:     p.CertPEM,
		AddedAt:     now,
	}
	if _, err := runner.ExecContext(ctx, upsertNodeCertStmt, cert.NodeID, cert.Fingerprint, cert.CertPEM, cert.AddedAt); err != nil {
		return nil, fmt.Errorf("upsert node cert: %w", err)
	}

	logger.DBLog.Info("redeemed cluster join token",
		zap.String("node_id", string(p.NodeID)),
		zap.String("fingerprint", p.Fingerprint))

	return db.pinSnapshot(ctx, runner)
}

func (db *Database) pinSnapshot(ctx context.Context, runner querier) (any, error) {
	pins, err := queryAll(ctx, runner, listNodeCertsStmt, scanClusterNodeCert)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}

	return &RedeemJoinTokenResult{Pins: pins}, nil
}

func (db *Database) applyDeleteJoinTokensStale(ctx context.Context, cutoff *ClusterJoinToken) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, deleteJoinTokensStaleStmt, cutoff.ExpiresAt, cutoff.ConsumedAt)

	return nil, err
}

func (db *Database) applyInitJoinHMAC(ctx context.Context, r *ClusterJoinHMAC) (any, error) {
	_, err := db.runner(ctx).ExecContext(ctx, initJoinHMACStmt, r.HMACKey)

	return nil, err
}

// ---------------------------------------------------------------------------
// Public methods.
// ---------------------------------------------------------------------------

// ListClusterNodeCerts returns every pinned per-node cert. The
// listener verifier consults this through an in-memory pin map
// rebuilt on a tick by the runtime.
func (db *Database) ListClusterNodeCerts(ctx context.Context) ([]ClusterNodeCert, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterNodeCertsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(ClusterNodeCertsTableName),
		),
	)
	defer span.End()

	rows, err := queryAll(ctx, db.reader(), listNodeCertsStmt, scanClusterNodeCert)
	if err != nil {
		recordSpanError(span, err)

		return nil, fmt.Errorf("list cluster node certs: %w", err)
	}

	return rows, nil
}

// GetClusterNodeCertByFingerprint returns the row matching
// fingerprint, or ErrNotFound. Diagnostic-path lookup; the verifier
// hot path uses an in-memory pin map.
func (db *Database) GetClusterNodeCertByFingerprint(ctx context.Context, fingerprint string) (*ClusterNodeCert, error) {
	querySummary := fmt.Sprintf("%s %s (by fingerprint)", "SELECT", ClusterNodeCertsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(ClusterNodeCertsTableName),
		),
	)
	defer span.End()

	row, err := scanClusterNodeCert(db.reader().QueryRowContext(ctx, getNodeCertByFPStmt, fingerprint))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("get cluster node cert: %w", err)
	}

	return &row, nil
}

// UpsertClusterNodeCert pins (or re-pins on rotation) a node's cert.
// The leader's /cluster/pki/register handler drives this.
func (db *Database) UpsertClusterNodeCert(ctx context.Context, r *ClusterNodeCert) error {
	querySummary := fmt.Sprintf("%s %s", "UPSERT", ClusterNodeCertsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPSERT"),
			semconv.DBCollectionName(ClusterNodeCertsTableName),
		),
	)
	defer span.End()

	_, err := opUpsertNodeCert.Invoke(ctx, db, r)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// DeleteClusterNodeCert removes a node's pin. Called from
// RemoveClusterMember; once the deletion replicates, peers reject
// the removed node's handshakes.
func (db *Database) DeleteClusterNodeCert(ctx context.Context, nodeID string) error {
	querySummary := fmt.Sprintf("%s %s", "DELETE", ClusterNodeCertsTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(ClusterNodeCertsTableName),
		),
	)
	defer span.End()

	_, err := opDeleteNodeCert.Invoke(ctx, db, &ClusterNodeCert{NodeID: nodeID})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// MintJoinTokenRecord persists a join-token row so the HMAC-validated
// token can be checked for single-use. The token string itself is
// emitted by pki.MintJoinToken; this row only carries metadata.
func (db *Database) MintJoinTokenRecord(ctx context.Context, r *ClusterJoinToken) error {
	querySummary := fmt.Sprintf("%s %s", "INSERT", ClusterJoinTokensTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(ClusterJoinTokensTableName),
		),
	)
	defer span.End()

	_, err := opMintJoinToken.Invoke(ctx, db, r)
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// GetJoinToken returns the token row for id, or ErrNotFound.
func (db *Database) GetJoinToken(ctx context.Context, id string) (*ClusterJoinToken, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterJoinTokensTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(ClusterJoinTokensTableName),
		),
	)
	defer span.End()

	row, err := scanClusterJoinToken(db.reader().QueryRowContext(ctx, getJoinTokenStmt, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("get join token: %w", err)
	}

	return &row, nil
}

// ConsumeJoinToken marks a token as consumed by the given nodeID. The
// UPDATE only matches unconsumed rows, so a second caller on a different
// voter (post-replication) finds nothing to update.
func (db *Database) ConsumeJoinToken(ctx context.Context, id string, nodeID string) error {
	querySummary := fmt.Sprintf("%s %s (consume)", "UPDATE", ClusterJoinTokensTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName(ClusterJoinTokensTableName),
		),
	)
	defer span.End()

	_, err := opConsumeJoinToken.Invoke(ctx, db, &ClusterJoinToken{
		ID:         id,
		ConsumedAt: time.Now().Unix(),
		ConsumedBy: nodeID,
	})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func (db *Database) RedeemJoinToken(ctx context.Context, tokenID string, nodeID string, fingerprint, certPEM string) ([]ClusterNodeCert, error) {
	ctx, span := tracer.Start(ctx, "db/redeem_join_token",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	res, err := opRedeemJoinToken.Invoke(ctx, db, &redeemJoinTokenPayload{
		TokenID:     tokenID,
		NodeID:      pki.NodeID(nodeID),
		Fingerprint: fingerprint,
		CertPEM:     certPEM,
	})
	if err != nil {
		recordSpanError(span, err)

		return nil, err
	}

	if res == nil || len(res.Pins) == 0 {
		err = fmt.Errorf("redeem returned an empty pin snapshot")
		recordSpanError(span, err)

		return nil, err
	}

	return res.Pins, nil
}

// DeleteStaleJoinTokens removes expired tokens and tokens consumed
// more than an hour ago.
func (db *Database) DeleteStaleJoinTokens(ctx context.Context, now time.Time) error {
	querySummary := fmt.Sprintf("%s %s (stale)", "DELETE", ClusterJoinTokensTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("DELETE"),
			semconv.DBCollectionName(ClusterJoinTokensTableName),
		),
	)
	defer span.End()

	cutoffConsumed := now.Add(-time.Hour).Unix()

	_, err := opDeleteStaleJoinTokens.Invoke(ctx, db, &ClusterJoinToken{
		ExpiresAt:  now.Add(-pki.JoinTokenClockSkew).Unix(),
		ConsumedAt: cutoffConsumed,
	})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

// GetClusterJoinHMACKey returns the join-token HMAC key, or
// ErrNotFound when the leader has not yet seeded it.
func (db *Database) GetClusterJoinHMACKey(ctx context.Context) ([]byte, error) {
	querySummary := fmt.Sprintf("%s %s", "SELECT", ClusterJoinHMACTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName(ClusterJoinHMACTableName),
		),
	)
	defer span.End()

	row, err := scanClusterJoinHMAC(db.reader().QueryRowContext(ctx, getJoinHMACStmt))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		recordSpanError(span, err)

		return nil, fmt.Errorf("get cluster join hmac key: %w", err)
	}

	return row.HMACKey, nil
}

// InitClusterJoinHMACKey seeds the singleton HMAC key on first
// leader promotion. Subsequent calls are no-ops (ON CONFLICT DO
// NOTHING) so the key is fixed for the cluster's lifetime.
func (db *Database) InitClusterJoinHMACKey(ctx context.Context, key []byte) error {
	querySummary := fmt.Sprintf("%s %s", "INSERT", ClusterJoinHMACTableName)

	ctx, span := tracer.Start(
		ctx,
		querySummary,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBQuerySummary(querySummary),
			semconv.DBSystemNameSQLite,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName(ClusterJoinHMACTableName),
		),
	)
	defer span.End()

	_, err := opInitJoinHMAC.Invoke(ctx, db, &ClusterJoinHMAC{HMACKey: key})
	if err != nil {
		recordSpanError(span, err)

		return err
	}

	return nil
}

func scanClusterNodeCert(row scanner) (ClusterNodeCert, error) {
	var v ClusterNodeCert

	if err := row.Scan(&v.NodeID, &v.Fingerprint, &v.CertPEM, &v.AddedAt); err != nil {
		return ClusterNodeCert{}, err
	}

	return v, nil
}

func scanClusterJoinToken(row scanner) (ClusterJoinToken, error) {
	var v ClusterJoinToken

	if err := row.Scan(&v.ID, &v.ClaimsJSON, &v.ExpiresAt, &v.ConsumedAt, &v.ConsumedBy); err != nil {
		return ClusterJoinToken{}, err
	}

	return v, nil
}

func scanClusterJoinHMAC(row scanner) (ClusterJoinHMAC, error) {
	var v ClusterJoinHMAC

	if err := row.Scan(&v.ID, &v.HMACKey); err != nil {
		return ClusterJoinHMAC{}, err
	}

	return v, nil
}
