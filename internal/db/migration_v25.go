// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

func migrateV25(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN ims BOOLEAN NOT NULL DEFAULT FALSE CHECK (ims IN (0, 1))", DataNetworksTableName),
		fmt.Sprintf("UPDATE %s SET ims = TRUE WHERE name = '%s'", DataNetworksTableName, models.IMSDataNetworkName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN preemptionCapability TEXT NOT NULL DEFAULT '%s' CHECK (preemptionCapability IN ('%s', '%s'))",
			PoliciesTableName, models.PreemptionCapabilityNotPreempt, models.PreemptionCapabilityNotPreempt, models.PreemptionCapabilityMayPreempt),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN preemptionVulnerability TEXT NOT NULL DEFAULT '%s' CHECK (preemptionVulnerability IN ('%s', '%s'))",
			PoliciesTableName, models.PreemptionVulnerabilityNotPreemptable, models.PreemptionVulnerabilityNotPreemptable, models.PreemptionVulnerabilityPreemptable),
		fmt.Sprintf(`CREATE TABLE %s (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL UNIQUE CHECK (name != ''),
			serverName TEXT NOT NULL UNIQUE CHECK (serverName != '')
		)`, IMSApplicationServersTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			identity            TEXT PRIMARY KEY CHECK (identity != ''),
			applicationServerID TEXT NOT NULL REFERENCES %s(id) ON DELETE CASCADE
		)`, IMSPublicServiceIdentitiesTableName, IMSApplicationServersTableName),
		fmt.Sprintf("CREATE INDEX idx_ims_public_service_identities_as ON %s(applicationServerID)", IMSPublicServiceIdentitiesTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			profileID               TEXT    NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			position                INTEGER NOT NULL CHECK (position >= 0),
			applicationServerID     TEXT    NOT NULL REFERENCES %s(id),
			triggerPoint            TEXT    CHECK (triggerPoint IS NULL OR json_valid(triggerPoint)),
			profilePart             TEXT    CHECK (profilePart IS NULL OR profilePart IN ('%s', '%s')),
			defaultHandling         TEXT    CHECK (defaultHandling IS NULL OR defaultHandling IN ('%s', '%s')),
			serviceInfo             TEXT,
			includeRegisterRequest  BOOLEAN NOT NULL CHECK (includeRegisterRequest IN (0, 1)),
			includeRegisterResponse BOOLEAN NOT NULL CHECK (includeRegisterResponse IN (0, 1)),
			PRIMARY KEY (profileID, position)
		)`, IMSInitialFilterCriteriaTableName, ProfilesTableName, IMSApplicationServersTableName, IMSProfilePartRegistered, IMSProfilePartUnregistered,
			IMSDefaultHandlingSessionContinued, IMSDefaultHandlingSessionTerminated),
		fmt.Sprintf("CREATE INDEX idx_ims_initial_filter_criteria_as ON %s(applicationServerID)", IMSInitialFilterCriteriaTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN privateIdentity TEXT NOT NULL DEFAULT '' CHECK (state = 0 OR privateIdentity != '')", IMSRegistrationsTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN defaultIdentity TEXT NOT NULL DEFAULT ''", IMSRegistrationsTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN profileDigest TEXT NOT NULL DEFAULT '' CHECK (state = 0 OR profileDigest != '')", IMSRegistrationsTableName),
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN emergency BOOLEAN NOT NULL DEFAULT FALSE CHECK (emergency IN (0, 1) AND (emergency = 0 OR state = 2))", IMSRegistrationsTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			imsi          TEXT PRIMARY KEY REFERENCES subscribers(imsi) ON DELETE CASCADE,
			number        TEXT NOT NULL CHECK (number != ''),
			diameterHost  TEXT NOT NULL CHECK (diameterHost != ''),
			diameterRealm TEXT NOT NULL CHECK (diameterRealm != '')
		)`, IPSMGWRegistrationsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add IMS services schema: %w", err)
		}
	}

	return nil
}
