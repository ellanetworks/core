// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

const (
	NATDefaultEnabled = true
)

const NATSettingsTableName = "nat_settings"

const getNATSettingsStmt = `SELECT &NATSettings.* FROM %s WHERE singleton=TRUE;`

type NATSettings struct {
	Enabled bool `db:"enabled"`
}
