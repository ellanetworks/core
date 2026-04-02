// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V7 migration — Data model redesign: profiles, policies, slices.
//
// Introduces the new three-level subscription model:
//   - network_slices: first-class S-NSSAI entities (replaces operator.sst/sd)
//   - profiles: named subscriber groupings with UE-AMBR
//   - policies (new schema): per-(profile, slice, DNN) QoS configuration
//
// Steps:
//  1. Create network_slices table, seed from operator.sst/sd.
//  2. Create profiles table, populate from old policies.
//  3. Rebuild policies table with new schema (profileID, sliceID, dataNetworkID).
//  4. Re-key network_rules.policy_id from old to new policy IDs.
//  5. Rebuild subscribers table: policyID → profileID.
//  6. Rebuild operator table: remove sst/sd columns.
// ---------------------------------------------------------------------------

func migrateV7(ctx context.Context, tx *sql.Tx) error {
	// -----------------------------------------------------------------------
	// 1. Create network_slices and seed from operator.sst / operator.sd.
	// -----------------------------------------------------------------------
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id   INTEGER PRIMARY KEY AUTOINCREMENT,
			sst  INTEGER NOT NULL,
			sd   TEXT,
			name TEXT NOT NULL UNIQUE,
			UNIQUE(sst, sd)
		)`, NetworkSlicesTableName))
	if err != nil {
		return fmt.Errorf("failed to create network_slices table: %w", err)
	}

	// operator.sd is a 3-byte BLOB; convert to 6-char hex TEXT.
	// If sd is NULL, the hex() will produce an empty string — store NULL instead.
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (sst, sd, name)
		SELECT sst,
		       CASE WHEN sd IS NOT NULL AND length(sd) > 0 THEN lower(hex(sd)) ELSE NULL END,
		       'default'
		FROM %s WHERE id = 1`,
		NetworkSlicesTableName, OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to seed network_slices from operator: %w", err)
	}

	// -----------------------------------------------------------------------
	// 2. Create profiles table, populate from old policies.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT NOT NULL UNIQUE,
			ueAmbrUplink   TEXT NOT NULL,
			ueAmbrDownlink TEXT NOT NULL
		)`, ProfilesTableName))
	if err != nil {
		return fmt.Errorf("failed to create profiles table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, ueAmbrUplink, ueAmbrDownlink)
		SELECT name, bitrateUplink, bitrateDownlink
		FROM %s`, ProfilesTableName, PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to populate profiles from old policies: %w", err)
	}

	// -----------------------------------------------------------------------
	// 3. Rebuild policies table with new schema.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s RENAME TO %s_old`, PoliciesTableName, PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to rename policies to policies_old: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			name                TEXT    NOT NULL UNIQUE,
			profileID           INTEGER NOT NULL,
			sliceID             INTEGER NOT NULL,
			dataNetworkID       INTEGER NOT NULL,
			var5qi              INTEGER NOT NULL,
			arp                 INTEGER NOT NULL,
			sessionAmbrUplink   TEXT    NOT NULL,
			sessionAmbrDownlink TEXT    NOT NULL,
			FOREIGN KEY (profileID)     REFERENCES %s (id) ON DELETE RESTRICT,
			FOREIGN KEY (sliceID)       REFERENCES %s (id) ON DELETE RESTRICT,
			FOREIGN KEY (dataNetworkID) REFERENCES %s (id) ON DELETE RESTRICT,
			UNIQUE(profileID, sliceID, dataNetworkID)
		)`, PoliciesTableName, ProfilesTableName, NetworkSlicesTableName, DataNetworksTableName))
	if err != nil {
		return fmt.Errorf("failed to create new policies table: %w", err)
	}

	// Populate: each old policy maps to a profile with the same name, uses the
	// single migrated slice, keeps the same DNN and QoS, and copies bitrate as
	// session AMBR.
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (name, profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		SELECT
			pol.name,
			p.id,
			(SELECT id FROM %s LIMIT 1),
			pol.dataNetworkID,
			pol.var5qi,
			pol.arp,
			pol.bitrateUplink,
			pol.bitrateDownlink
		FROM %s_old pol
		JOIN %s p ON p.name = pol.name`,
		PoliciesTableName, NetworkSlicesTableName, PoliciesTableName, ProfilesTableName))
	if err != nil {
		return fmt.Errorf("failed to populate new policies from old: %w", err)
	}

	// -----------------------------------------------------------------------
	// 4. Rebuild network_rules with FK pointing to the new policies table.
	//
	// After the policies RENAME, SQLite (with PRAGMA foreign_keys = ON)
	// rewrites the FK in network_rules to reference policies_old. If we
	// just UPDATE policy_id in-place, the subsequent DROP of policies_old
	// would cascade-delete every rule. Rebuilding the table avoids this.
	//
	// ID mapping: old policy name → new policy ID.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			policy_id INTEGER NOT NULL,
			description TEXT NOT NULL,
			direction TEXT NOT NULL,
			remote_prefix TEXT,
			protocol INTEGER DEFAULT 255,
			port_low INTEGER DEFAULT 0,
			port_high INTEGER DEFAULT 0,
			action TEXT NOT NULL,
			precedence INTEGER NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (policy_id) REFERENCES %s (id) ON DELETE CASCADE,
			UNIQUE(policy_id, precedence, direction)
		)`, NetworkRulesTableName, PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to create network_rules_new table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s_new (id, policy_id, description, direction, remote_prefix, protocol, port_low, port_high, action, precedence, created_at, updated_at)
		SELECT
			nr.id,
			new_pol.id,
			nr.description,
			nr.direction,
			nr.remote_prefix,
			nr.protocol,
			nr.port_low,
			nr.port_high,
			nr.action,
			nr.precedence,
			nr.created_at,
			nr.updated_at
		FROM %s nr
		JOIN %s_old old_pol ON old_pol.id = nr.policy_id
		JOIN %s new_pol ON new_pol.name = old_pol.name`,
		NetworkRulesTableName,
		NetworkRulesTableName,
		PoliciesTableName,
		PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to copy network_rules with re-keyed policy_id: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, NetworkRulesTableName))
	if err != nil {
		return fmt.Errorf("failed to drop old network_rules table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s_new RENAME TO %s`, NetworkRulesTableName, NetworkRulesTableName))
	if err != nil {
		return fmt.Errorf("failed to rename network_rules_new: %w", err)
	}

	// -----------------------------------------------------------------------
	// 5. Rebuild subscribers: policyID → profileID.
	//
	// Map via: subscribers.policyID → policies_old.id → policies_old.name
	//        → profiles.name → profiles.id
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s_new (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			imsi           TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
			sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
			permanentKey   TEXT NOT NULL CHECK (length(permanentKey) = 32),
			opc            TEXT NOT NULL CHECK (length(opc) = 32),
			profileID      INTEGER NOT NULL,
			FOREIGN KEY (profileID) REFERENCES %s (id) ON DELETE RESTRICT
		)`, SubscribersTableName, ProfilesTableName))
	if err != nil {
		return fmt.Errorf("failed to create subscribers_new table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s_new (id, imsi, sequenceNumber, permanentKey, opc, profileID)
		SELECT s.id, s.imsi, s.sequenceNumber, s.permanentKey, s.opc, p.id
		FROM %s s
		JOIN %s_old pol ON pol.id = s.policyID
		JOIN %s p ON p.name = pol.name`,
		SubscribersTableName,
		SubscribersTableName,
		PoliciesTableName,
		ProfilesTableName))
	if err != nil {
		return fmt.Errorf("failed to copy subscribers with profileID mapping: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to drop old subscribers table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s_new RENAME TO %s`, SubscribersTableName, SubscribersTableName))
	if err != nil {
		return fmt.Errorf("failed to rename subscribers_new: %w", err)
	}

	// -----------------------------------------------------------------------
	// 6. Rebuild operator table without sst/sd columns.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE %s_new (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			mcc           TEXT NOT NULL,
			mnc           TEXT NOT NULL,
			operatorCode  TEXT NOT NULL,
			supportedTACs TEXT DEFAULT '[]',
			ciphering     TEXT NOT NULL DEFAULT '["NEA2","NEA1","NEA0"]',
			integrity     TEXT NOT NULL DEFAULT '["NIA2","NIA1","NIA0"]',
			spnFullName   TEXT NOT NULL DEFAULT 'Ella Networks',
			spnShortName  TEXT NOT NULL DEFAULT 'Ella'
		)`, OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to create operator_new table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s_new (id, mcc, mnc, operatorCode, supportedTACs, ciphering, integrity, spnFullName, spnShortName)
		SELECT id, mcc, mnc, operatorCode, supportedTACs, ciphering, integrity, spnFullName, spnShortName
		FROM %s`, OperatorTableName, OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to copy operator data: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to drop old operator table: %w", err)
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s_new RENAME TO %s`, OperatorTableName, OperatorTableName))
	if err != nil {
		return fmt.Errorf("failed to rename operator_new: %w", err)
	}

	// -----------------------------------------------------------------------
	// 7. Drop policies_old.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s_old`, PoliciesTableName))
	if err != nil {
		return fmt.Errorf("failed to drop policies_old: %w", err)
	}

	return nil
}
