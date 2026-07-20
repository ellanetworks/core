// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

// ClearDownlinkDataNotification re-arms downlink-data paging for a session by
// clearing its buffered-data notification state, so the next downlink packet
// raises a fresh Downlink Data Notification (TS 23.401 §5.3.4.3, TS 23.502 §4.2.3.3).
func (conn *SessionEngine) ClearDownlinkDataNotification(seid uint64) {
	if conn.BpfObjects == nil {
		return
	}

	conn.BpfObjects.ClearNotifiedForSEID(seid)
}
