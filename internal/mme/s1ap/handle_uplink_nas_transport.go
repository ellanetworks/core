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

	// Track the UE's current serving-cell TAI so a later TAU is gated on where the UE
	// now is (TS 36.413: UPLINK NAS TRANSPORT carries the current TAI).
	ue.Conn().ServingTAI = msg.TAI
	ue.Conn().UpdateLocation(msg.EUTRANCGI, msg.TAI)

	// resolveUE guarantees the UE is connected on this association, so ue.Conn() is
	// the connection the message arrived on.
	m.NAS.HandleNAS(ctx, ue.Conn(), []byte(msg.NASPDU))
}
