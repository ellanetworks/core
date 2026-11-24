// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
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
	violations, err := foreignKeyViolationsOf(ctx, tx)
	if err != nil {
		return err
	}

	if len(violations) > 0 {
		logger.DBLog.Warn("Replicated rows break a foreign key",
			zap.Int("count", len(violations)),
			zap.Strings("violations", violations[:min(len(violations), 10)]))
	}

	for _, stmt := range []string{createSubscriberCountSQL, seedSubscriberCountSQL, createLatestLeaseIndexSQL} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate to schema 31: %w", err)
		}
	}

	return nil
}

func foreignKeyViolationsOf(ctx context.Context, q rowQuerier) ([]string, error) {
	var violations []string

	for _, table := range migrationV31Tables {
		found, err := foreignKeyViolations(ctx, q, table)
		if err != nil {
			return nil, err
		}

		violations = append(violations, found...)
	}

	return violations, nil
}

func foreignKeyViolations(ctx context.Context, q rowQuerier, table string) ([]string, error) {
	rows, err := q.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_check(%q)", table))
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
