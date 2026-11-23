// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var migrationV31Tables = []string{
	"subscribers", "policies", "profiles", "data_networks", "slice_data_networks", "network_slices",
	"network_rules", "subscriber_framed_routes", "ip_leases", "users", "api_tokens", "sessions",
	"home_network_keys", "retention_policies", "operator", "jwt_secret", "cluster_members",
	"cluster_node_certs", "cluster_join_tokens", "cluster_join_hmac", "daily_usage", "cell_positions",
	"ue_registrations", "sms_settings", "sms_smscs", "diameter_peers", "sms_smsc_service_centres",
	"sms_message_waiting", "sms_message_waiting_centres", "pcscf_addresses", "ims_settings", "ims_peers",
	"ims_registrations", "ims_registration_sets", "ip_sm_gw_registrations", "ims_application_servers",
	"ims_public_service_identities", "ims_service_profiles", "ims_initial_filter_criteria",
	"ims_service_profile_assignments", "roaming_partners", "roaming_partner_plmns", "visitors", "sepp_ca",
	"sepp_identities", "user_identities", "oidc_settings", "oidc_group_roles", "oidc_used_states",
	"radio_event_settings",
}

func migrateV31(ctx context.Context, tx *sql.Tx) error {
	if err := checkForeignKeys(ctx, tx); err != nil {
		return err
	}

	for _, stmt := range []string{createSubscriberCountSQL, seedSubscriberCountSQL, createLatestLeaseIndexSQL} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate to schema 31: %w", err)
		}
	}

	return nil
}

func checkForeignKeys(ctx context.Context, tx *sql.Tx) error {
	var violations []string

	for _, table := range migrationV31Tables {
		found, err := foreignKeyViolations(ctx, tx, table)
		if err != nil {
			return err
		}

		violations = append(violations, found...)
	}

	if len(violations) > 0 {
		return fmt.Errorf("%d foreign key violations: %s", len(violations), strings.Join(violations[:min(len(violations), 10)], "; "))
	}

	return nil
}

func foreignKeyViolations(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_check(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("check foreign keys of %s: %w", table, err)
	}

	defer func() { _ = rows.Close() }()

	var violations []string

	for rows.Next() {
		var (
			child, parent string
			rowid         sql.NullInt64
			fkid          int
		)

		if err := rows.Scan(&child, &rowid, &parent, &fkid); err != nil {
			return nil, fmt.Errorf("scan foreign key violation of %s: %w", table, err)
		}

		violations = append(violations, fmt.Sprintf("%s row %d references a missing %s row (constraint %d)", child, rowid.Int64, parent, fkid))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("check foreign keys of %s: %w", table, err)
	}

	return violations, nil
}
