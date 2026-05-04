// Copyright 2026 Ella Networks

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

	"github.com/canonical/sqlair"
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
	listNodeCertsStmtStr        = "SELECT &ClusterNodeCert.* FROM %s ORDER BY nodeID ASC"
	getNodeCertByFPStmtStr      = "SELECT &ClusterNodeCert.* FROM %s WHERE fingerprint=$ClusterNodeCert.fingerprint"
	upsertNodeCertStmtStr       = "INSERT INTO %s (nodeID, fingerprint, certPEM, addedAt) VALUES ($ClusterNodeCert.nodeID, $ClusterNodeCert.fingerprint, $ClusterNodeCert.certPEM, $ClusterNodeCert.addedAt) ON CONFLICT(nodeID) DO UPDATE SET fingerprint=excluded.fingerprint, certPEM=excluded.certPEM, addedAt=excluded.addedAt"
	deleteNodeCertByNodeStmtStr = "DELETE FROM %s WHERE nodeID=$ClusterNodeCert.nodeID"

	insertJoinTokenStmtStr       = "INSERT INTO %s (id, nodeID, claimsJSON, expiresAt, consumedAt, consumedBy) VALUES ($ClusterJoinToken.id, $ClusterJoinToken.nodeID, $ClusterJoinToken.claimsJSON, $ClusterJoinToken.expiresAt, 0, 0)" // #nosec G101 -- SQL statement
	getJoinTokenStmtStr          = "SELECT &ClusterJoinToken.* FROM %s WHERE id=$ClusterJoinToken.id"
	consumeJoinTokenStmtStr      = "UPDATE %s SET consumedAt=$ClusterJoinToken.consumedAt, consumedBy=$ClusterJoinToken.consumedBy WHERE id=$ClusterJoinToken.id AND consumedAt=0" // #nosec G101 -- SQL statement
	deleteJoinTokensStaleStmtStr = "DELETE FROM %s WHERE expiresAt<$ClusterJoinToken.expiresAt OR (consumedAt>0 AND consumedAt<$ClusterJoinToken.consumedAt)"                      // #nosec G101 -- SQL statement

	getJoinHMACStmtStr  = "SELECT &ClusterJoinHMAC.* FROM %s WHERE id=1"
	initJoinHMACStmtStr = "INSERT INTO %s (id, hmacKey) VALUES (1, $ClusterJoinHMAC.hmacKey) ON CONFLICT(id) DO NOTHING"
)

// ClusterNodeCert pins one cluster member's self-signed leaf by
// SHA-256. One row per nodeID. Inserted by the join flow; removed by
// RemoveClusterMember.
type ClusterNodeCert struct {
	NodeID      int    `db:"nodeID"`
	Fingerprint string `db:"fingerprint"`
	CertPEM     string `db:"certPEM"`
	AddedAt     int64  `db:"addedAt"`
}

// ClusterJoinToken is a row in cluster_join_tokens. The HMAC tag is
// embedded in the token string the admin copies to the joining node;
// this row only records the token's identity and consumption state so
// replays against a different voter fail.
type ClusterJoinToken struct {
	ID         string `db:"id"`
	NodeID     int    `db:"nodeID"`
	ClaimsJSON string `db:"claimsJSON"`
	ExpiresAt  int64  `db:"expiresAt"`
	ConsumedAt int64  `db:"consumedAt"`
	ConsumedBy int    `db:"consumedBy"`
}

// ClusterJoinHMAC is the singleton row in cluster_join_hmac.
type ClusterJoinHMAC struct {
	ID      int    `db:"id"`
	HMACKey []byte `db:"hmacKey"`
}

// ---------------------------------------------------------------------------
// Apply methods — invoked from the typed-op dispatch layer.
// ---------------------------------------------------------------------------

func (db *Database) applyUpsertNodeCert(ctx context.Context, r *ClusterNodeCert) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.upsertNodeCertStmt, r).Run()
}

func (db *Database) applyDeleteNodeCert(ctx context.Context, r *ClusterNodeCert) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.deleteNodeCertByNodeStmt, r).Run()
}

func (db *Database) applyInsertJoinToken(ctx context.Context, r *ClusterJoinToken) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.insertJoinTokenStmt, r).Run()
}

func (db *Database) applyConsumeJoinToken(ctx context.Context, r *ClusterJoinToken) (any, error) {
	var outcome sqlair.Outcome

	if err := db.runner(ctx).Query(ctx, db.consumeJoinTokenStmt, r).Get(&outcome); err != nil {
		return nil, err
	}

	rows, err := outcome.Result().RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("rows affected: %w", err)
	}

	if rows == 0 {
		return nil, ErrJoinTokenAlreadyConsumed
	}

	return nil, nil
}

func (db *Database) applyDeleteJoinTokensStale(ctx context.Context, cutoff *ClusterJoinToken) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.deleteJoinTokensStaleStmt, cutoff).Run()
}

func (db *Database) applyInitJoinHMAC(ctx context.Context, r *ClusterJoinHMAC) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.initJoinHMACStmt, r).Run()
}

// ---------------------------------------------------------------------------
// Public methods.
// ---------------------------------------------------------------------------

// ListClusterNodeCerts returns every pinned per-node cert. The
// listener verifier consults this through an in-memory pin map
// rebuilt on a tick by the runtime.
func (db *Database) ListClusterNodeCerts(ctx context.Context) ([]ClusterNodeCert, error) {
	var rows []ClusterNodeCert

	if err := db.conn().Query(ctx, db.listNodeCertsStmt).GetAll(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("list cluster node certs: %w", err)
	}

	return rows, nil
}

// GetClusterNodeCertByFingerprint returns the row matching
// fingerprint, or ErrNotFound. Diagnostic-path lookup; the verifier
// hot path uses an in-memory pin map.
func (db *Database) GetClusterNodeCertByFingerprint(ctx context.Context, fingerprint string) (*ClusterNodeCert, error) {
	row := ClusterNodeCert{Fingerprint: fingerprint}

	err := db.conn().Query(ctx, db.getNodeCertByFPStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get cluster node cert: %w", err)
	}

	return &row, nil
}

// UpsertClusterNodeCert pins (or re-pins on rotation) a node's cert.
// The leader's /cluster/pki/register handler drives this.
func (db *Database) UpsertClusterNodeCert(ctx context.Context, r *ClusterNodeCert) error {
	_, err := opUpsertNodeCert.Invoke(db, r)

	return err
}

// DeleteClusterNodeCert removes a node's pin. Called from
// RemoveClusterMember; once the deletion replicates, peers reject
// the removed node's handshakes.
func (db *Database) DeleteClusterNodeCert(ctx context.Context, nodeID int) error {
	_, err := opDeleteNodeCert.Invoke(db, &ClusterNodeCert{NodeID: nodeID})

	return err
}

// MintJoinTokenRecord persists a join-token row so the HMAC-validated
// token can be checked for single-use. The token string itself is
// emitted by pki.MintJoinToken; this row only carries metadata.
func (db *Database) MintJoinTokenRecord(ctx context.Context, r *ClusterJoinToken) error {
	_, err := opMintJoinToken.Invoke(db, r)

	return err
}

// GetJoinToken returns the token row for id, or ErrNotFound.
func (db *Database) GetJoinToken(ctx context.Context, id string) (*ClusterJoinToken, error) {
	row := ClusterJoinToken{ID: id}

	err := db.conn().Query(ctx, db.getJoinTokenStmt, row).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get join token: %w", err)
	}

	return &row, nil
}

// ConsumeJoinToken marks a token as consumed by the given nodeID. The
// UPDATE only matches unconsumed rows, so a second caller on a different
// voter (post-replication) finds nothing to update.
func (db *Database) ConsumeJoinToken(ctx context.Context, id string, nodeID int) error {
	_, err := opConsumeJoinToken.Invoke(db, &ClusterJoinToken{
		ID:         id,
		ConsumedAt: time.Now().Unix(),
		ConsumedBy: nodeID,
	})

	return err
}

// DeleteStaleJoinTokens removes expired tokens and tokens consumed
// more than an hour ago.
func (db *Database) DeleteStaleJoinTokens(ctx context.Context, now time.Time) error {
	cutoffConsumed := now.Add(-time.Hour).Unix()

	_, err := opDeleteStaleJoinTokens.Invoke(db, &ClusterJoinToken{
		ExpiresAt:  now.Unix(),
		ConsumedAt: cutoffConsumed,
	})

	return err
}

// GetClusterJoinHMACKey returns the join-token HMAC key, or
// ErrNotFound when the leader has not yet seeded it.
func (db *Database) GetClusterJoinHMACKey(ctx context.Context) ([]byte, error) {
	var row ClusterJoinHMAC

	err := db.conn().Query(ctx, db.getJoinHMACStmt).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get cluster join hmac key: %w", err)
	}

	return row.HMACKey, nil
}

// InitClusterJoinHMACKey seeds the singleton HMAC key on first
// leader promotion. Subsequent calls are no-ops (ON CONFLICT DO
// NOTHING) so the key is fixed for the cluster's lifetime.
func (db *Database) InitClusterJoinHMACKey(ctx context.Context, key []byte) error {
	_, err := opInitJoinHMAC.Invoke(db, &ClusterJoinHMAC{HMACKey: key})

	return err
}
