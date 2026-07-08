// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// handleUplinkNASTransport routes an uplink NAS message to its UE context
// (TS 36.413).
func handleUplinkNASTransport(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUplinkNASTransport(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcUplinkNASTransport, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	// resolveUE guarantees the UE is connected on this association, so ue.Conn() is
	// the connection the message arrived on.
	m.NAS.HandleNAS(ctx, ue.Conn(), []byte(msg.NASPDU))
}
