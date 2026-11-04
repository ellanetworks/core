// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

func (db *Database) SetMaxSubscribers(n int) {
	db.maxSubscribers = n
}
