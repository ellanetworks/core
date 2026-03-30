// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// ---------------------------------------------------------------------------
// V5 migration — data model redesign: slices, profiles, and network configs.
//
// Replaces the flat policies table with:
//   - network_slices: first-class S-NSSAI entities (replaces operator.sst/sd)
//   - profiles: named service tiers
//   - profile_network_configs: per-slice, per-DNN QoS authorization
//
// Subscribers move from policyID to profileID. The operator table drops its
// sst/sd columns. All existing data is preserved through careful migration.
// ---------------------------------------------------------------------------

func migrateV5(ctx context.Context, tx *sql.Tx) error {
	// -----------------------------------------------------------------------
	// 1. Create network_slices table.
	// -----------------------------------------------------------------------
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE network_slices (
			id   INTEGER PRIMARY KEY,
			sst  INTEGER NOT NULL,
			sd   TEXT,
			name TEXT NOT NULL UNIQUE,
			UNIQUE(sst, sd)
		)`)
	if err != nil {
		return fmt.Errorf("failed to create network_slices table: %w", err)
	}

	// -----------------------------------------------------------------------
	// 2. Populate network_slices from operator.sst / operator.sd.
	//    operator.sd is a 3-byte BLOB — convert to uppercase hex TEXT.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `
		INSERT INTO network_slices (sst, sd, name)
		SELECT sst, hex(sd), 'default'
		FROM operator
		WHERE id = 1`)
	if err != nil {
		return fmt.Errorf("failed to seed network_slices from operator: %w", err)
	}

	// -----------------------------------------------------------------------
	// 3. Create profiles table.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE profiles (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			name           TEXT NOT NULL UNIQUE
		)`)
	if err != nil {
		return fmt.Errorf("failed to create profiles table: %w", err)
	}

	// -----------------------------------------------------------------------
	// 4. Create profile_network_configs table.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE profile_network_configs (
			id                  INTEGER PRIMARY KEY AUTOINCREMENT,
			profileID           INTEGER NOT NULL,
			sliceID             INTEGER NOT NULL,
			dataNetworkID       INTEGER NOT NULL,
			var5qi              INTEGER NOT NULL,
			arp                 INTEGER NOT NULL,
			sessionAmbrUplink   TEXT NOT NULL,
			sessionAmbrDownlink TEXT NOT NULL,
			UNIQUE(profileID, sliceID, dataNetworkID),
			FOREIGN KEY (profileID)     REFERENCES profiles(id)       ON DELETE CASCADE,
			FOREIGN KEY (sliceID)       REFERENCES network_slices(id) ON DELETE CASCADE,
			FOREIGN KEY (dataNetworkID) REFERENCES data_networks(id)  ON DELETE CASCADE
		)`)
	if err != nil {
		return fmt.Errorf("failed to create profile_network_configs table: %w", err)
	}

	// -----------------------------------------------------------------------
	// 5. Migrate data: policies → profiles + profile_network_configs.
	//    Each old policy becomes one profile and one config row in the single
	//    slice. The old bitrate is copied to both UE-AMBR and session AMBR.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `
		INSERT INTO profiles (name)
		SELECT name
		FROM policies`)
	if err != nil {
		return fmt.Errorf("failed to migrate policies to profiles: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO profile_network_configs
			(profileID, sliceID, dataNetworkID, var5qi, arp, sessionAmbrUplink, sessionAmbrDownlink)
		SELECT
			p.id,
			(SELECT id FROM network_slices LIMIT 1),
			pol.dataNetworkID,
			pol.var5qi,
			pol.arp,
			pol.bitrateUplink,
			pol.bitrateDownlink
		FROM policies pol
		JOIN profiles p ON p.name = pol.name`)
	if err != nil {
		return fmt.Errorf("failed to migrate policies to profile_network_configs: %w", err)
	}

	// -----------------------------------------------------------------------
	// 6. Rebuild subscribers: policyID → profileID.
	//    Map old policyID to new profileID via the name join.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE subscribers_new (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			imsi           TEXT NOT NULL UNIQUE CHECK (length(imsi) BETWEEN 6 AND 15 AND imsi GLOB '[0-9]*'),
			sequenceNumber TEXT NOT NULL CHECK (length(sequenceNumber) = 12),
			permanentKey   TEXT NOT NULL CHECK (length(permanentKey) = 32),
			opc            TEXT NOT NULL CHECK (length(opc) = 32),
			profileID      INTEGER NOT NULL,
			FOREIGN KEY (profileID) REFERENCES profiles(id) ON DELETE CASCADE
		)`)
	if err != nil {
		return fmt.Errorf("failed to create subscribers_new table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO subscribers_new (id, imsi, sequenceNumber, permanentKey, opc, profileID)
		SELECT
			s.id, s.imsi, s.sequenceNumber, s.permanentKey, s.opc,
			(SELECT p.id FROM profiles p JOIN policies pol ON pol.name = p.name WHERE pol.id = s.policyID)
		FROM subscribers s`)
	if err != nil {
		return fmt.Errorf("failed to copy subscribers with profileID mapping: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE subscribers`)
	if err != nil {
		return fmt.Errorf("failed to drop old subscribers table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE subscribers_new RENAME TO subscribers`)
	if err != nil {
		return fmt.Errorf("failed to rename subscribers_new to subscribers: %w", err)
	}

	// -----------------------------------------------------------------------
	// 7. Drop policies table.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `DROP TABLE policies`)
	if err != nil {
		return fmt.Errorf("failed to drop policies table: %w", err)
	}

	// -----------------------------------------------------------------------
	// 8. Rebuild operator table: remove sst/sd columns.
	// -----------------------------------------------------------------------
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE operator_new (
			id            INTEGER PRIMARY KEY CHECK (id = 1),
			mcc           TEXT NOT NULL,
			mnc           TEXT NOT NULL,
			operatorCode  TEXT NOT NULL,
			supportedTACs TEXT DEFAULT '[]',
			ciphering     TEXT NOT NULL DEFAULT '["NEA2","NEA1","NEA0"]',
			integrity     TEXT NOT NULL DEFAULT '["NIA2","NIA1","NIA0"]',
			spnFullName   TEXT NOT NULL DEFAULT 'Ella Networks',
			spnShortName  TEXT NOT NULL DEFAULT 'Ella'
		)`)
	if err != nil {
		return fmt.Errorf("failed to create operator_new table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO operator_new (id, mcc, mnc, operatorCode, supportedTACs, ciphering, integrity, spnFullName, spnShortName)
		SELECT id, mcc, mnc, operatorCode, supportedTACs, ciphering, integrity, spnFullName, spnShortName
		FROM operator`)
	if err != nil {
		return fmt.Errorf("failed to copy operator data: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE operator`)
	if err != nil {
		return fmt.Errorf("failed to drop old operator table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE operator_new RENAME TO operator`)
	if err != nil {
		return fmt.Errorf("failed to rename operator_new to operator: %w", err)
	}

	return nil
}
