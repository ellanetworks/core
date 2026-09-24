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
func handleUplinkNASTransport(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUplinkNASTransport(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcUplinkNASTransport, s1ap.TriggeringInitiatingMessage, err)
		return
	}

	ue, ueConn, ok := resolveUE(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcUplinkNASTransport, s1ap.TriggeringInitiatingMessage, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), msg.Diagnostics())

	ue.TouchLastSeen()

	// Track the UE's current serving-cell TAI so a later TAU is gated on where the UE
	// now is (TS 36.413: UPLINK NAS TRANSPORT carries the current TAI). An omitted TAI
	// leaves the last known one standing (§10.3.5).
	if msg.TAI != nil {
		ueConn.ServingTAI = *msg.TAI

		if msg.EUTRANCGI != nil {
			ueConn.UpdateLocation(*msg.EUTRANCGI, *msg.TAI)
		}
	}

	// resolveUE guarantees the UE is connected on this association, so ueConn is
	// the connection the message arrived on.
	m.NAS.HandleNAS(ctx, ueConn, []byte(msg.NASPDU))
}
