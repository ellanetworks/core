// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import "github.com/ellanetworks/core/internal/upf/ebpf"

// SuppressDownlinkDataNotification keeps the session's downlink data notification
// deduped after a failed page, so an unreachable idle UE is not re-paged by every
// downlink packet (TS 23.401 §5.3.4.3; TS 23.502 §4.2.3.3).
func (conn *SessionEngine) SuppressDownlinkDataNotification(seid uint64) {
	conn.eachDownlinkNotification(seid, func(d ebpf.DataNotification) {
		conn.BpfObjects.MarkNotified(d)
	})
}

// ClearDownlinkDataNotification releases the suppression once the UE is reachable
// again, so subsequent downlink data pages it (TS 24.301 §5.3.5; TS 23.502 §4.2.3.3
// step 3c). It releases exactly the entries SuppressDownlinkDataNotification marks.
func (conn *SessionEngine) ClearDownlinkDataNotification(seid uint64) {
	conn.eachDownlinkNotification(seid, func(d ebpf.DataNotification) {
		conn.BpfObjects.ClearNotified(d.LocalSEID, d.PdrID, d.QFI)
	})
}

func (conn *SessionEngine) eachDownlinkNotification(seid uint64, apply func(ebpf.DataNotification)) {
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

		apply(ebpf.DataNotification{
			LocalSEID: seid,
			PdrID:     uint16(pdr.PdrID),
			QFI:       pdr.PdrInfo.Qer.Qfi,
		})
	}
}
