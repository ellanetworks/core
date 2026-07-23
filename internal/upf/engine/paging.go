// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import "github.com/ellanetworks/core/internal/upf/ebpf"

// SuppressDownlinkDataNotification keeps a session's downlink data-notification
// dedup set after a failed page, so an unreachable idle UE is not re-paged by every
// subsequent downlink packet (TS 23.401 §5.3.4.3; TS 23.502 §4.2.3.3). The dedup is
// released when the UE returns and the session is reactivated (ClearNotified in
// establish/modify) or on delete, so downlink reachability recovers with no extra
// state to unwind.
func (conn *SessionEngine) SuppressDownlinkDataNotification(seid uint64) {
	if conn.BpfObjects == nil {
		return
	}

	session := conn.GetSession(seid)
	if session == nil {
		return
	}

	for _, pdr := range session.ListPDRs() {
		if !pdr.UEIP.IsValid() {
			continue
		}

		conn.BpfObjects.MarkNotified(ebpf.DataNotification{
			LocalSEID: seid,
			PdrID:     uint16(pdr.PdrID),
			QFI:       pdr.PdrInfo.Qer.Qfi,
		})
	}
}
