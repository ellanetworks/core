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

// handleHandoverNotify completes the handover once the UE reaches the target: it
// switches the user plane, commits the {NH, NCC} chain, moves the active S1
// connection to the target, and releases the source by its own MME-UE-S1AP-ID
// (TS 36.413 §8.4.3, TS 23.401 §5.5.1.2.2 steps 13-19). conn is the target.
func handleHandoverNotify(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	notify, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		handleParseError(m, conn, s1ap.ProcHandoverNotification, err)
		return
	}

	ue, ok := m.LookupUe(notify.MMEUES1APID)
	if !ok {
		return
	}

	admitted, releaseEBIs, ok := m.BeginHandoverCommit(ue, conn, notify.ENBUES1APID)
	if !ok {
		logger.MmeLog.Warn("Handover Notify with no matching prepared handover", zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	// Switch the downlink only at notify (TS 23.401 §5.5.1.2.2 step 15).
	for _, a := range admitted {
		p := m.LookupPDN(ue, a.Ebi)
		if p == nil {
			continue
		}

		if err := m.Session.ModifyEPSSession(ctx, ue.IMSI(), a.Ebi, a.EnbFTEID); err != nil {
			logger.MmeLog.Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", a.Ebi), zap.Error(err))

			continue
		}

		m.SetPDNEnbFTEID(ue, p, a.EnbFTEID)
	}

	// Release the PDN connections whose default bearer the target rejected.
	for _, ebi := range releaseEBIs {
		if err := m.Session.ReleaseEPSSession(ctx, ue.IMSI(), ebi); err != nil {
			logger.MmeLog.Error("failed to release a rejected PDN connection after handover",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", ebi), zap.Error(err))
		}

		m.DropPDN(ue, ebi)
	}

	mme.EnsureDefaultPDN(ue, admitted)

	sourceConn, sourceMMEID, sourceENBID, targetMMEID, ok := m.FinishHandoverCommit(ue, conn, notify.ENBUES1APID)
	if !ok {
		// A concurrent release (e.g. the source association dropping) tore the UE
		// down during the unlocked user-plane switch above and cleared the handover;
		// it is moot, so leave the UE released.
		logger.MmeLog.Warn("Handover Notify: UE released during the user-plane switch",
			zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	if notify.TAI.TAC != 0 {
		ue.TouchLastSeen()
	}

	logger.MmeLog.Info("Handover Notify",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.Uint32("target-enb-ue-id", uint32(notify.ENBUES1APID)))

	mme.SendUEContextRelease(m, ctx, sourceConn, sourceMMEID, sourceENBID)
}
