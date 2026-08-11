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

// handleHandoverNotify completes the handover once the UE reaches the target
// (TS 36.413 §8.4.3, TS 23.401 §5.5.1.2.2 steps 13-19).
func handleHandoverNotify(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	notify, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcHandoverNotification, err)
		return
	}

	ue, ok := m.LookupUe(notify.MMEUES1APID)
	if !ok {
		sendErrorIndication(m, radio.Conn, &notify.MMEUES1APID, &notify.ENBUES1APID, causeUnknownMMEUES1APID)

		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcHandoverNotification, s1ap.TriggeringInitiatingMessage, ueAssociated(notify.MMEUES1APID, notify.ENBUES1APID), notify.Diagnostics())

	admitted, ok := m.MarkHandoverCommitting(ue, radio.Conn, notify.ENBUES1APID)
	if !ok {
		if _, _, valid := resolveUE(m, radio.Conn, notify.MMEUES1APID, notify.ENBUES1APID); valid {
			logger.From(ctx, logger.MmeLog).Warn("Handover Notify with no matching prepared handover", zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))
		}

		return
	}

	present := make([]mme.RANBearer, 0, len(admitted))
	for _, a := range admitted {
		present = append(present, mme.RANBearer(a))
	}

	m.ReconcileBearersToRAN(ctx, ue, mme.RANBearers{
		Present:       present,
		Authoritative: true,
	})

	sourceConn, sourceMMEID, sourceENBID, targetMMEID, ok := m.FinishHandoverCommit(ue, radio.Conn, notify.ENBUES1APID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Handover Notify: UE released during the user-plane switch",
			zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	ue.TouchLastSeen()

	if ueConn := ue.Conn(); ueConn != nil && notify.EUTRANCGI != nil && notify.TAI != nil {
		ueConn.UpdateLocation(*notify.EUTRANCGI, *notify.TAI)
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Notify",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.Uint32("target-enb-ue-id", uint32(notify.ENBUES1APID)))

	// No source eNB: the UE arrived from 5GS, and the source is released by the
	// peer AMF once it is told the handover completed (TS 23.502 §4.11.1.2.1).
	if sourceConn == nil {
		m.CompleteRelocation(ctx, ue)

		return
	}

	mme.SendUEContextRelease(ctx, m, sourceConn, sourceMMEID, sourceENBID, true, mme.CauseHandoverSuccess)
}
