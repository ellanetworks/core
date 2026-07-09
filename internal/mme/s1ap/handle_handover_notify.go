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
		return
	}

	admitted, releaseEBIs, ok := m.MarkHandoverCommitting(ue, radio.Conn, notify.ENBUES1APID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("Handover Notify with no matching prepared handover", zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	// Switch the downlink only at notify (TS 23.401 §5.5.1.2.2 step 15).
	for _, a := range admitted {
		p := m.LookupPDN(ue, a.Ebi)
		if p == nil {
			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), a.Ebi, a.EnbFTEID); err != nil {
			logger.From(ctx, logger.MmeLog).Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", a.Ebi), zap.Error(err))

			continue
		}

		m.SetPDNEnbFTEID(ue, p, a.EnbFTEID)
	}

	for _, ebi := range releaseEBIs {
		if p := m.LookupPDN(ue, ebi); p != nil {
			if err := m.Session.ReleaseEPSSession(ctx, p.SessionRef); err != nil {
				logger.From(ctx, logger.MmeLog).Error("failed to release a rejected PDN connection after handover",
					zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", ebi), zap.Error(err))
			}
		}

		m.DropPDN(ue, ebi)
	}

	mme.EnsureDefaultPDN(ue, admitted)

	sourceConn, sourceMMEID, sourceENBID, targetMMEID, ok := m.FinishHandoverCommit(ue, radio.Conn, notify.ENBUES1APID)
	if !ok {
		// A concurrent release (e.g. the source association dropping) tore the UE
		// down during the unlocked user-plane switch and cleared the handover; leave
		// it released.
		logger.From(ctx, logger.MmeLog).Warn("Handover Notify: UE released during the user-plane switch",
			zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	ue.TouchLastSeen()

	logger.From(ctx, logger.MmeLog).Info("Handover Notify",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.Uint32("target-enb-ue-id", uint32(notify.ENBUES1APID)))

	mme.SendUEContextRelease(ctx, m, sourceConn, sourceMMEID, sourceENBID, true, mme.CauseHandoverSuccess)
}
