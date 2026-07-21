// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

func (conn *SessionEngine) ClearDownlinkDataNotification(seid uint64) {
	if conn.BpfObjects == nil {
		return
	}

	conn.BpfObjects.ClearNotifiedForSEID(seid)
}
