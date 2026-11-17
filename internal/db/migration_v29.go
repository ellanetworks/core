// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
)

func migrateV29(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN rolePinned BOOLEAN NOT NULL DEFAULT FALSE", UsersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			id          TEXT    PRIMARY KEY,
			userID      TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			issuer      TEXT    NOT NULL CHECK (issuer != ''),
			subject     TEXT    NOT NULL CHECK (length(CAST(subject AS BLOB)) BETWEEN 1 AND 255),
			email       TEXT    NOT NULL,
			createdAt   INTEGER NOT NULL,
			lastLoginAt INTEGER NOT NULL,
			UNIQUE (issuer, subject),
			UNIQUE (userID, issuer)
		)`, UserIdentitiesTableName),
		fmt.Sprintf("CREATE INDEX idx_user_identities_user ON %s (userID)", UserIdentitiesTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN method TEXT NOT NULL DEFAULT '%s' CHECK (method IN ('%s', '%s'))",
			SessionsTableName, SessionPassword, SessionPassword, SessionOIDC),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN identity_id TEXT REFERENCES %s(id) ON DELETE CASCADE",
			SessionsTableName, UserIdentitiesTableName),
		fmt.Sprintf("CREATE INDEX idx_sessions_identity ON %s (identity_id)", SessionsTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			singleton            BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
			enabled              BOOLEAN NOT NULL,
			externalURL          TEXT    NOT NULL,
			issuer               TEXT    NOT NULL,
			clientID             TEXT    NOT NULL,
			clientSecret         TEXT    NOT NULL,
			displayName          TEXT    NOT NULL,
			scopes               TEXT    NOT NULL,
			sessionLifetime      INTEGER NOT NULL CHECK (sessionLifetime > 0),
			trustUnverifiedEmail BOOLEAN NOT NULL,
			caCertificate        TEXT    NOT NULL,
			groupsClaim          TEXT    NOT NULL,
			CHECK (NOT enabled OR (externalURL != '' AND issuer != '' AND clientID != '' AND clientSecret != ''
				AND displayName != '' AND scopes != ''))
		)`, OIDCSettingsTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			position  INTEGER PRIMARY KEY CHECK (position > 0),
			groupName TEXT    NOT NULL UNIQUE CHECK (groupName != ''),
			roleID    INTEGER NOT NULL CHECK (roleID IN (1, 2, 3))
		)`, OIDCGroupRolesTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			stateHash BLOB    PRIMARY KEY CHECK (length(stateHash) = 32),
			expiresAt INTEGER NOT NULL
		) WITHOUT ROWID`, OIDCUsedStatesTableName),
		fmt.Sprintf("CREATE INDEX idx_oidc_used_states_expires ON %s (expiresAt)", OIDCUsedStatesTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add single sign-on: %w", err)
		}
	}

	return nil
}
