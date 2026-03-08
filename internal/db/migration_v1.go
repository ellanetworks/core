// Copyright 2026 Ella Networks

package db

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateV1 creates the baseline schema. Every CREATE TABLE / CREATE INDEX
// uses IF NOT EXISTS so this migration is safe for both:
//   - Fresh databases (tables don't exist yet)
//   - Existing databases being migrated for the first time (tables already exist,
//     statements are no-ops)
//
// After this migration ships, these DDL constants become the historical record
// of the V1 schema. Future schema changes go in V2+ — never modify this function.
func migrateV1(ctx context.Context, tx *sql.Tx) error {
	// Ordered so that tables with foreign key dependencies are created after
	// the tables they reference.
	stmts := []string{
		// Independent tables (no FK deps)
		fmt.Sprintf(QueryCreateOperatorTable, OperatorTableName),
		fmt.Sprintf(QueryCreateRoutesTable, RoutesTableName),
		fmt.Sprintf(QueryCreateRetentionPolicyTable, RetentionPolicyTableName),
		fmt.Sprintf(QueryCreateNATSettingsTable, NATSettingsTableName),
		fmt.Sprintf(QueryCreateFlowAccountingSettingsTable, FlowAccountingSettingsTableName),
		fmt.Sprintf(QueryCreateN3SettingsTable, N3SettingsTableName),
		fmt.Sprintf(QueryCreateAuditLogsTable, AuditLogsTableName),

		// Radio events + indexes
		fmt.Sprintf(QueryCreateRadioEventsTable, RadioEventsTableName),

		// Data networks → policies → subscribers (FK chain)
		fmt.Sprintf(QueryCreateDataNetworksTable, DataNetworksTableName),
		fmt.Sprintf(QueryCreatePoliciesTable, PoliciesTableName),
		fmt.Sprintf(QueryCreateSubscribersTable, SubscribersTableName),

		// Users → sessions, api_tokens (FK chain)
		fmt.Sprintf(QueryCreateUsersTable, UsersTableName),
		createSessionsTableSQL,
		fmt.Sprintf(QueryCreateAPITokensTable, APITokensTableName),

		// Tables depending on subscribers
		fmt.Sprintf(QueryCreateDailyUsageTable, DailyUsageTableName),
		fmt.Sprintf(QueryCreateFlowReportsTable, FlowReportsTableName),
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to execute DDL: %w\nStatement: %s", err, stmt)
		}
	}

	// Index creation statements are multi-statement strings (separated by ;).
	// tx.ExecContext with go-sqlite3 supports multi-statement execution.
	indexStmts := []string{
		QueryCreateRadioEventsIndex,
		QueryCreateFlowReportsIndex,
	}

	for _, stmt := range indexStmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("failed to create indexes: %w\nStatement: %s", err, stmt)
		}
	}

	return nil
}
