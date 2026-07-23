// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import "github.com/ellanetworks/core/internal/upf/ebpf"

// SuppressDownlinkDataNotification keeps the session's downlink data notification
// deduped after a failed page, so an unreachable idle UE is not re-paged by every
// downlink packet (TS 23.401 §5.3.4.3; TS 23.502 §4.2.3.3).
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
