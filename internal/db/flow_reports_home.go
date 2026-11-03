// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/canonical/sqlair"
)

const homeFlowReportsClause = "WHERE subscriber_id IN (SELECT imsi FROM %s WHERE roamingPartnerID IS NULL) AND"

func homeFlowReportStatsStmt(stmt string) string {
	return strings.Replace(fmt.Sprintf(stmt, FlowReportsTableName), "WHERE", fmt.Sprintf(homeFlowReportsClause, SubscribersTableName), 1)
}

func (db *Database) flowReportStatsStmts(ctx context.Context) (*sqlair.Statement, *sqlair.Statement) {
	if !db.appliedSchemaAtLeast(ctx, roamingSchema) {
		return db.flowReportProtocolCountsStmt, db.flowReportTopDestinationsUplinkStmt
	}

	return db.homeFlowReportProtocolCountsStmt, db.homeFlowReportTopDestinationsStmt
}
