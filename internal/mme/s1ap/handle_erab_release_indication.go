// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func handleERABReleaseIndication(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseERABReleaseIndication(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcERABReleaseIndication, s1ap.TriggeringInitiatingMessage, err)
		return
	}

	ue, ueConn, ok := resolveUE(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcERABReleaseIndication, s1ap.TriggeringInitiatingMessage, ueAssociated(msg.MMEUES1APID, msg.ENBUES1APID), msg.Diagnostics())

	ue.TouchLastSeen()
	captureUserLocation(ueConn, msg.UserLocationInformation)

	seen := make(map[s1ap.ERABID]struct{}, len(msg.ERABReleased))

	for _, erab := range msg.ERABReleased {
		if _, dup := seen[erab.ERABID]; dup {
			continue
		}

		seen[erab.ERABID] = struct{}{}
		ebi := uint8(erab.ERABID)

		if m.DedicatedReleasedByRAN(ctx, ue, ebi) {
			continue
		}

		if p := m.LookupPDN(ue, ebi); p != nil {
			ueConn.Log(ctx).Warn("eNB released a default bearer's E-RAB; disconnecting its PDN connection (TS 23.401 §5.10.3)",
				logger.SUPI(ue.Supi().String()), logger.ERABID(ebi), zap.String("cause", erab.Cause.String()))
			m.DisconnectLostPDN(ctx, ue, p)

			continue
		}

		ueConn.Log(ctx).Info("eNB released an E-RAB the core does not hold", logger.SUPI(ue.Supi().String()), logger.ERABID(ebi))
	}
}
