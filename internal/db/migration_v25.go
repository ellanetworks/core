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
			id                  TEXT PRIMARY KEY,
			name                TEXT NOT NULL UNIQUE CHECK (name != ''),
			kind                TEXT NOT NULL CHECK (kind IN ('%s', '%s')),
			applicationServerID TEXT NOT NULL REFERENCES %s(id)
		)`, IMSServicesTableName, IMSServiceKindMCPTT, IMSServiceKindCustom, IMSApplicationServersTableName),
		fmt.Sprintf("CREATE INDEX idx_ims_services_as ON %s(applicationServerID)", IMSServicesTableName),
		fmt.Sprintf(`CREATE TABLE %s (
			serviceID               TEXT    NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			position                INTEGER NOT NULL CHECK (position >= 0),
			triggerPoint            TEXT    NOT NULL CHECK (json_valid(triggerPoint)),
			defaultHandling         TEXT    NOT NULL CHECK (defaultHandling IN ('%s', '%s')),
			includeRegisterRequest  BOOLEAN NOT NULL CHECK (includeRegisterRequest IN (0, 1)),
			includeRegisterResponse BOOLEAN NOT NULL CHECK (includeRegisterResponse IN (0, 1)),
			PRIMARY KEY (serviceID, position)
		)`, IMSInitialFilterCriteriaTableName, IMSServicesTableName, IMSDefaultHandlingSessionContinued, IMSDefaultHandlingSessionTerminated),
		fmt.Sprintf(`CREATE TABLE %s (
			profileID TEXT    NOT NULL REFERENCES %s(id) ON DELETE CASCADE,
			serviceID TEXT    NOT NULL REFERENCES %s(id),
			position  INTEGER NOT NULL CHECK (position >= 0),
			PRIMARY KEY (profileID, serviceID),
			UNIQUE (profileID, position)
		)`, ProfileIMSServicesTableName, ProfilesTableName, IMSServicesTableName),
		fmt.Sprintf("CREATE INDEX idx_profile_ims_services_service ON %s(serviceID)", ProfileIMSServicesTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("add IMS services schema: %w", err)
		}
	}

	return nil
}
