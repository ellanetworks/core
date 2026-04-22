// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/canonical/sqlair"
)

// Table names. Kept short so the schema grep'd in migration tests stays
// readable.
const (
	ClusterPKIRootsTableName         = "cluster_pki_roots"
	ClusterPKIIntermediatesTableName = "cluster_pki_intermediates"
	ClusterIssuedCertsTableName      = "cluster_issued_certs"
	ClusterRevokedCertsTableName     = "cluster_revoked_certs"
	ClusterJoinTokensTableName       = "cluster_join_tokens"
	ClusterPKIStateTableName         = "cluster_pki_state"
)

// PKI cert lifecycle status values. Kept in sync with the CHECK constraint
// in migration_v9.
const (
	PKIStatusActive     = "active"
	PKIStatusVerifyOnly = "verify-only"
	PKIStatusRetired    = "retired"
)

// Prepared-statement templates. `%s` takes the table name; see
// PrepareStatements for the bindings.
const (
	listPKIRootsStmtStr     = "SELECT &ClusterPKIRoot.* FROM %s ORDER BY addedAt ASC"
	insertPKIRootStmtStr    = "INSERT INTO %s (fingerprint, certPEM, crossSignedPEM, keyPEM, addedAt, status) VALUES ($ClusterPKIRoot.fingerprint, $ClusterPKIRoot.certPEM, $ClusterPKIRoot.crossSignedPEM, $ClusterPKIRoot.keyPEM, $ClusterPKIRoot.addedAt, $ClusterPKIRoot.status)"
	setPKIRootStatusStmtStr = "UPDATE %s SET status=$ClusterPKIRoot.status, keyPEM=$ClusterPKIRoot.keyPEM WHERE fingerprint=$ClusterPKIRoot.fingerprint"
	deletePKIRootStmtStr    = "DELETE FROM %s WHERE fingerprint=$ClusterPKIRoot.fingerprint"

	listPKIIntermediatesStmtStr     = "SELECT &ClusterPKIIntermediate.* FROM %s ORDER BY notAfter ASC"
	insertPKIIntermediateStmtStr    = "INSERT INTO %s (fingerprint, certPEM, crossSignedPEM, keyPEM, rootFingerprint, notAfter, status) VALUES ($ClusterPKIIntermediate.fingerprint, $ClusterPKIIntermediate.certPEM, $ClusterPKIIntermediate.crossSignedPEM, $ClusterPKIIntermediate.keyPEM, $ClusterPKIIntermediate.rootFingerprint, $ClusterPKIIntermediate.notAfter, $ClusterPKIIntermediate.status)"
	setPKIIntermediateStatusStmtStr = "UPDATE %s SET status=$ClusterPKIIntermediate.status, keyPEM=$ClusterPKIIntermediate.keyPEM WHERE fingerprint=$ClusterPKIIntermediate.fingerprint"
	deletePKIIntermediateStmtStr    = "DELETE FROM %s WHERE fingerprint=$ClusterPKIIntermediate.fingerprint"

	insertIssuedCertStmtStr         = "INSERT INTO %s (serial, nodeID, notAfter, intermediateFingerprint, issuedAt) VALUES ($ClusterIssuedCert.serial, $ClusterIssuedCert.nodeID, $ClusterIssuedCert.notAfter, $ClusterIssuedCert.intermediateFingerprint, $ClusterIssuedCert.issuedAt)"
	listIssuedCertsByNodeStmtStr    = "SELECT &ClusterIssuedCert.* FROM %s WHERE nodeID=$ClusterIssuedCert.nodeID AND notAfter>$ClusterIssuedCert.notAfter"
	listIssuedCertsActiveStmtStr    = "SELECT &ClusterIssuedCert.* FROM %s WHERE notAfter>$ClusterIssuedCert.notAfter"
	deleteIssuedCertsExpiredStmtStr = "DELETE FROM %s WHERE notAfter<$ClusterIssuedCert.notAfter"

	insertRevokedCertStmtStr        = "INSERT INTO %s (serial, nodeID, revokedAt, reason, purgeAfter) VALUES ($ClusterRevokedCert.serial, $ClusterRevokedCert.nodeID, $ClusterRevokedCert.revokedAt, $ClusterRevokedCert.reason, $ClusterRevokedCert.purgeAfter) ON CONFLICT(serial) DO NOTHING"
	listRevokedCertsStmtStr         = "SELECT &ClusterRevokedCert.* FROM %s"
	deleteRevokedCertsPurgedStmtStr = "DELETE FROM %s WHERE purgeAfter<$ClusterRevokedCert.purgeAfter"

	insertJoinTokenStmtStr       = "INSERT INTO %s (id, nodeID, claimsJSON, expiresAt, consumedAt, consumedBy) VALUES ($ClusterJoinToken.id, $ClusterJoinToken.nodeID, $ClusterJoinToken.claimsJSON, $ClusterJoinToken.expiresAt, 0, 0)" // #nosec G101 -- SQL statement, not a credential
	getJoinTokenStmtStr          = "SELECT &ClusterJoinToken.* FROM %s WHERE id=$ClusterJoinToken.id"
	consumeJoinTokenStmtStr      = "UPDATE %s SET consumedAt=$ClusterJoinToken.consumedAt, consumedBy=$ClusterJoinToken.consumedBy WHERE id=$ClusterJoinToken.id AND consumedAt=0" // #nosec G101 -- SQL statement, not a credential
	deleteJoinTokensStaleStmtStr = "DELETE FROM %s WHERE expiresAt<$ClusterJoinToken.expiresAt OR (consumedAt>0 AND consumedAt<$ClusterJoinToken.consumedAt)"                      // #nosec G101 -- SQL statement, not a credential

	initPKIStateStmtStr   = "INSERT INTO %s (id, hmacKey, serialCounter) VALUES (1, $ClusterPKIState.hmacKey, 0) ON CONFLICT(id) DO NOTHING"
	getPKIStateStmtStr    = "SELECT &ClusterPKIState.* FROM %s WHERE id=1"
	allocateSerialStmtStr = "UPDATE %s SET serialCounter=serialCounter+1 WHERE id=1 RETURNING serialCounter AS &ClusterPKIState.serialCounter"
)

// ClusterPKIRoot is a row in cluster_pki_roots. KeyPEM holds the PKCS#8
// private-key PEM; a CHECK constraint enforces that it is populated iff
// status='active'.
type ClusterPKIRoot struct {
	Fingerprint    string `db:"fingerprint"`
	CertPEM        string `db:"certPEM"`
	CrossSignedPEM string `db:"crossSignedPEM"`
	KeyPEM         []byte `db:"keyPEM"`
	AddedAt        int64  `db:"addedAt"`
	Status         string `db:"status"`
}

// ClusterPKIIntermediate is a row in cluster_pki_intermediates. See
// ClusterPKIRoot for the KeyPEM / status invariant.
type ClusterPKIIntermediate struct {
	Fingerprint     string `db:"fingerprint"`
	CertPEM         string `db:"certPEM"`
	CrossSignedPEM  string `db:"crossSignedPEM"`
	KeyPEM          []byte `db:"keyPEM"`
	RootFingerprint string `db:"rootFingerprint"`
	NotAfter        int64  `db:"notAfter"`
	Status          string `db:"status"`
}

// ClusterIssuedCert is a row in cluster_issued_certs. The serial is
// allocated from the replicated counter (ClusterPKIState.serialCounter).
type ClusterIssuedCert struct {
	Serial                  int64  `db:"serial"`
	NodeID                  int    `db:"nodeID"`
	NotAfter                int64  `db:"notAfter"`
	IntermediateFingerprint string `db:"intermediateFingerprint"`
	IssuedAt                int64  `db:"issuedAt"`
}

// ClusterRevokedCert is a row in cluster_revoked_certs.
type ClusterRevokedCert struct {
	Serial     int64  `db:"serial"`
	NodeID     int    `db:"nodeID"`
	RevokedAt  int64  `db:"revokedAt"`
	Reason     string `db:"reason"`
	PurgeAfter int64  `db:"purgeAfter"`
}

// ClusterJoinToken is a row in cluster_join_tokens. The HMAC tag is
// embedded in the token string the admin copies to the joining node; this
// row only records the token's identity and consumption state so replays
// against a different voter fail.
type ClusterJoinToken struct {
	ID         string `db:"id"`
	NodeID     int    `db:"nodeID"`
	ClaimsJSON string `db:"claimsJSON"`
	ExpiresAt  int64  `db:"expiresAt"`
	ConsumedAt int64  `db:"consumedAt"`
	ConsumedBy int    `db:"consumedBy"`
}

// ClusterPKIState is the cluster_pki_state singleton row.
type ClusterPKIState struct {
	HMACKey       []byte `db:"hmacKey"`
	SerialCounter int64  `db:"serialCounter"`
}

// ---------------------------------------------------------------------------
// Apply methods — invoked from proposeChangeset; capture SQL mutations.
// ---------------------------------------------------------------------------

func (db *Database) applyInsertPKIRoot(ctx context.Context, r *ClusterPKIRoot) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.insertPKIRootStmt, r).Run()
}

func (db *Database) applySetPKIRootStatus(ctx context.Context, r *ClusterPKIRoot) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.setPKIRootStatusStmt, r).Run()
}

func (db *Database) applyDeletePKIRoot(ctx context.Context, r *ClusterPKIRoot) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.deletePKIRootStmt, r).Run()
}

func (db *Database) applyInsertPKIIntermediate(ctx context.Context, r *ClusterPKIIntermediate) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.insertPKIIntermediateStmt, r).Run()
}

func (db *Database) applySetPKIIntermediateStatus(ctx context.Context, r *ClusterPKIIntermediate) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.setPKIIntermediateStatusStmt, r).Run()
}

func (db *Database) applyDeletePKIIntermediate(ctx context.Context, r *ClusterPKIIntermediate) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.deletePKIIntermediateStmt, r).Run()
}

func (db *Database) applyInsertIssuedCert(ctx context.Context, r *ClusterIssuedCert) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.insertIssuedCertStmt, r).Run()
}

func (db *Database) applyDeleteIssuedCertsExpired(ctx context.Context, cutoff *ClusterIssuedCert) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.deleteIssuedCertsExpiredStmt, cutoff).Run()
}

func (db *Database) applyInsertRevokedCert(ctx context.Context, r *ClusterRevokedCert) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.insertRevokedCertStmt, r).Run()
}

func (db *Database) applyDeleteRevokedCertsPurged(ctx context.Context, cutoff *ClusterRevokedCert) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.deleteRevokedCertsPurgedStmt, cutoff).Run()
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

func (db *Database) applyInitPKIState(ctx context.Context, r *ClusterPKIState) (any, error) {
	return nil, db.runner(ctx).Query(ctx, db.initPKIStateStmt, r).Run()
}

func (db *Database) applyAllocateSerial(ctx context.Context) (any, error) {
	var out ClusterPKIState

	err := db.runner(ctx).Query(ctx, db.allocateSerialStmt).Get(&out)
	if err != nil {
		return nil, err
	}

	return out.SerialCounter, nil
}

// ---------------------------------------------------------------------------
// Public methods — all mutations go through proposeChangeset so followers
// replicate via the standard sqlite changeset path.
// ---------------------------------------------------------------------------

// ListPKIRoots returns every root in the bundle, regardless of status.
func (db *Database) ListPKIRoots(ctx context.Context) ([]ClusterPKIRoot, error) {
	var rows []ClusterPKIRoot

	if err := db.conn().Query(ctx, db.listPKIRootsStmt).GetAll(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("list pki roots: %w", err)
	}

	return rows, nil
}

// InsertPKIRoot persists a root. Used at bootstrap and root rotation.
func (db *Database) InsertPKIRoot(ctx context.Context, r *ClusterPKIRoot) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyInsertPKIRoot(ctx, r)
	}, "InsertPKIRoot")

	return err
}

// SetPKIRootStatus transitions a root to verify-only or retired and
// NULLs its keyPEM in the same changeset, so the old signing key is
// compacted out of the raft log at the next snapshot. Introducing a
// new active row is done via InsertPKIRoot, not this path.
func (db *Database) SetPKIRootStatus(ctx context.Context, fingerprint, status string) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applySetPKIRootStatus(ctx, &ClusterPKIRoot{
			Fingerprint: fingerprint,
			Status:      status,
			KeyPEM:      nil,
		})
	}, "SetPKIRootStatus")

	return err
}

// DeletePKIRoot drops a root from the bundle. Caller must verify no live
// issued certs chain through it.
func (db *Database) DeletePKIRoot(ctx context.Context, fingerprint string) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyDeletePKIRoot(ctx, &ClusterPKIRoot{Fingerprint: fingerprint})
	}, "DeletePKIRoot")

	return err
}

// ListPKIIntermediates returns every intermediate in the bundle.
func (db *Database) ListPKIIntermediates(ctx context.Context) ([]ClusterPKIIntermediate, error) {
	var rows []ClusterPKIIntermediate

	if err := db.conn().Query(ctx, db.listPKIIntermediatesStmt).GetAll(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("list pki intermediates: %w", err)
	}

	return rows, nil
}

// InsertPKIIntermediate persists an intermediate. Used at bootstrap and
// intermediate rotation.
func (db *Database) InsertPKIIntermediate(ctx context.Context, r *ClusterPKIIntermediate) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyInsertPKIIntermediate(ctx, r)
	}, "InsertPKIIntermediate")

	return err
}

// SetPKIIntermediateStatus is the intermediate counterpart of
// SetPKIRootStatus.
func (db *Database) SetPKIIntermediateStatus(ctx context.Context, fingerprint, status string) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applySetPKIIntermediateStatus(ctx, &ClusterPKIIntermediate{
			Fingerprint: fingerprint,
			Status:      status,
			KeyPEM:      nil,
		})
	}, "SetPKIIntermediateStatus")

	return err
}

// DeletePKIIntermediate drops an intermediate.
func (db *Database) DeletePKIIntermediate(ctx context.Context, fingerprint string) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyDeletePKIIntermediate(ctx, &ClusterPKIIntermediate{Fingerprint: fingerprint})
	}, "DeletePKIIntermediate")

	return err
}

// RecordIssuedCert registers a newly issued leaf so it can be revoked on
// node removal and tidied after expiry.
func (db *Database) RecordIssuedCert(ctx context.Context, r *ClusterIssuedCert) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyInsertIssuedCert(ctx, r)
	}, "RecordIssuedCert")

	return err
}

// ListActiveIssuedCertsByNode returns every non-expired cert issued to
// nodeID. Used by RemoveClusterMember to drive revocation.
func (db *Database) ListActiveIssuedCertsByNode(ctx context.Context, nodeID int) ([]ClusterIssuedCert, error) {
	var rows []ClusterIssuedCert

	arg := ClusterIssuedCert{NodeID: nodeID, NotAfter: time.Now().Unix()}

	if err := db.conn().Query(ctx, db.listIssuedCertsByNodeStmt, arg).GetAll(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("list issued certs by node: %w", err)
	}

	return rows, nil
}

// CountActiveIssuedCerts returns how many non-expired issued certs exist.
// Used by root rotation to decide when the old root can be retired.
func (db *Database) CountActiveIssuedCerts(ctx context.Context) (int, error) {
	var rows []ClusterIssuedCert

	arg := ClusterIssuedCert{NotAfter: time.Now().Unix()}

	if err := db.conn().Query(ctx, db.listIssuedCertsActiveStmt, arg).GetAll(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}

		return 0, fmt.Errorf("count active issued certs: %w", err)
	}

	return len(rows), nil
}

// DeleteExpiredIssuedCerts removes rows where notAfter < now − 1h. Called
// by the tidy worker.
func (db *Database) DeleteExpiredIssuedCerts(ctx context.Context, cutoff time.Time) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyDeleteIssuedCertsExpired(ctx, &ClusterIssuedCert{NotAfter: cutoff.Unix()})
	}, "DeleteExpiredIssuedCerts")

	return err
}

// InsertRevokedCert records a revocation. Idempotent on serial.
func (db *Database) InsertRevokedCert(ctx context.Context, r *ClusterRevokedCert) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyInsertRevokedCert(ctx, r)
	}, "InsertRevokedCert")

	return err
}

// ListRevokedCerts returns every revocation row. Used by the cache to
// rebuild on leader change.
func (db *Database) ListRevokedCerts(ctx context.Context) ([]ClusterRevokedCert, error) {
	var rows []ClusterRevokedCert

	if err := db.conn().Query(ctx, db.listRevokedCertsStmt).GetAll(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, fmt.Errorf("list revoked certs: %w", err)
	}

	return rows, nil
}

// DeletePurgedRevocations drops rows where purgeAfter < now. Tidy worker.
func (db *Database) DeletePurgedRevocations(ctx context.Context, cutoff time.Time) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyDeleteRevokedCertsPurged(ctx, &ClusterRevokedCert{PurgeAfter: cutoff.Unix()})
	}, "DeletePurgedRevocations")

	return err
}

// MintJoinTokenRecord persists a join-token row so the HMAC-validated
// token can be checked for single-use. The token string itself is emitted
// by pki.MintJoinToken; this row only carries metadata.
func (db *Database) MintJoinTokenRecord(ctx context.Context, r *ClusterJoinToken) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyInsertJoinToken(ctx, r)
	}, "MintJoinToken")

	return err
}

// GetJoinToken looks up a token row by id. Returns ErrNotFound if absent.
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
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyConsumeJoinToken(ctx, &ClusterJoinToken{
			ID:         id,
			ConsumedAt: time.Now().Unix(),
			ConsumedBy: nodeID,
		})
	}, "ConsumeJoinToken")

	return err
}

// DeleteStaleJoinTokens removes expired and old-consumed tokens.
func (db *Database) DeleteStaleJoinTokens(ctx context.Context, now time.Time) error {
	cutoffConsumed := now.Add(-time.Hour).Unix()

	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyDeleteJoinTokensStale(ctx, &ClusterJoinToken{
			ExpiresAt:  now.Unix(),
			ConsumedAt: cutoffConsumed,
		})
	}, "DeleteStaleJoinTokens")

	return err
}

// InitializePKIState seeds the cluster_pki_state singleton if absent.
// Called from the first-leader bootstrap path exactly once.
func (db *Database) InitializePKIState(ctx context.Context, hmacKey []byte) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyInitPKIState(ctx, &ClusterPKIState{HMACKey: hmacKey})
	}, "InitializePKIState")

	return err
}

// PKIBootstrap carries the three rows written by BootstrapPKI.
type PKIBootstrap struct {
	HMACKey      []byte
	Root         *ClusterPKIRoot
	Intermediate *ClusterPKIIntermediate
}

// BootstrapPKI writes the PKI state row, the root, and the intermediate
// inside a single raft-replicated changeset: either all three persist
// or none do.
func (db *Database) BootstrapPKI(ctx context.Context, payload *PKIBootstrap) error {
	_, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		if _, err := db.applyInitPKIState(ctx, &ClusterPKIState{HMACKey: payload.HMACKey}); err != nil {
			return nil, fmt.Errorf("init pki state: %w", err)
		}

		if _, err := db.applyInsertPKIRoot(ctx, payload.Root); err != nil {
			return nil, fmt.Errorf("insert pki root: %w", err)
		}

		if _, err := db.applyInsertPKIIntermediate(ctx, payload.Intermediate); err != nil {
			return nil, fmt.Errorf("insert pki intermediate: %w", err)
		}

		return nil, nil
	}, "BootstrapPKI")

	return err
}

// GetPKIState returns the cluster_pki_state singleton. Returns ErrNotFound
// if bootstrap has not populated it.
func (db *Database) GetPKIState(ctx context.Context) (*ClusterPKIState, error) {
	var row ClusterPKIState

	err := db.conn().Query(ctx, db.getPKIStateStmt).Get(&row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, fmt.Errorf("get pki state: %w", err)
	}

	return &row, nil
}

// ReadClusterTrustBundlePEM opens the SQLite file at dbPath with a
// short-lived read-only connection and concatenates the cert PEMs of
// all non-retired roots and intermediates (roots first). Used at
// startup to seed the listener's trust cache before the full Database
// + raft manager is constructed (which itself requires the listener).
func ReadClusterTrustBundlePEM(ctx context.Context, dbPath string) ([]byte, error) {
	conn, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("open db read-only: %w", err)
	}

	defer func() { _ = conn.Close() }()

	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	var out []byte

	for _, q := range []struct {
		table string
		order string
	}{
		{ClusterPKIRootsTableName, "addedAt ASC"},
		{ClusterPKIIntermediatesTableName, "notAfter ASC"},
	} {
		stmt := fmt.Sprintf("SELECT certPEM FROM %s WHERE status != 'retired' ORDER BY %s", q.table, q.order)

		rows, err := conn.QueryContext(ctx, stmt)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", q.table, err)
		}

		for rows.Next() {
			var pem string
			if err := rows.Scan(&pem); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan %s: %w", q.table, err)
			}

			out = append(out, []byte(pem)...)
		}

		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate %s: %w", q.table, err)
		}

		_ = rows.Close()
	}

	return out, nil
}

// AllocatePKISerial atomically bumps the replicated counter and returns
// the new value. Must be called on the leader; followers return an error
// from the Propose path.
func (db *Database) AllocatePKISerial(ctx context.Context) (int64, error) {
	v, err := db.proposeChangeset(func(ctx context.Context) (any, error) {
		return db.applyAllocateSerial(ctx)
	}, "AllocatePKISerial")
	if err != nil {
		return 0, err
	}

	n, ok := v.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected type from serial allocation: %T", v)
	}

	return n, nil
}
