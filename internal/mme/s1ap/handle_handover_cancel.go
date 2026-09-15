// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleHandoverCancel releases any prepared target resources and acknowledges,
// leaving the UE on the source (TS 36.413 §8.4.5).
func handleHandoverCancel(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	cancel, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcHandoverCancel, err)
		return
	}

	ue, ueConn, ok := resolveUE(ctx, m, radio.Conn, cancel.MMEUES1APID, cancel.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcHandoverCancel, s1ap.TriggeringInitiatingMessage, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), cancel.Diagnostics())

	ue.TouchLastSeen()

	// Relay the source's HANDOVER CANCEL Cause to the target when releasing its
	// prepared resources (TS 36.413 §8.4.5). An omitted Cause is an ignore-criticality
	// absence, so the target is still released, under a generic cause (§10.3.5).
	releaseCause := causeHandoverPrepUnspecific
	if cancel.Cause != nil {
		releaseCause = *cancel.Cause
	}

	if id, toFiveGS := m.RelocationToFiveGS(ue); toFiveGS {
		err := m.CancelRelocationToFiveGS(ctx, ue, id)

		switch {
		case errors.Is(err, interworking.ErrRelocationTooLate):
			logger.From(ctx, logger.MmeLog).Info("the UE has already reached 5GS; leaving the handover to complete",
				zap.Error(err))
			sendHandoverCancelAcknowledge(ctx, m, radio, cancel)

			return
		case err != nil:
			logger.From(ctx, logger.MmeLog).Info("the 5GS peer had no handover to cancel; unwinding locally",
				zap.Error(err))
		}
	}

	releaseConn, releaseMMEID, releaseENBID, pair, has, aborted := m.CancelHandover(ue)
	if aborted {
		m.CloseForwardingTunnels(ctx, ue)
	}

	if has {
		mme.SendUEContextRelease(ctx, m, releaseConn, releaseMMEID, releaseENBID, pair, releaseCause)
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Cancel", logger.MMEUeS1apID(uint32(cancel.MMEUES1APID)))
	sendHandoverCancelAcknowledge(ctx, m, radio, cancel)
}

func sendHandoverCancelAcknowledge(ctx context.Context, m *mme.MME, radio *mme.Radio, cancel *s1ap.HandoverCancel) {
	ack := &s1ap.HandoverCancelAcknowledge{MMEUES1APID: s1ap.Ptr(cancel.MMEUES1APID), ENBUES1APID: s1ap.Ptr(cancel.ENBUES1APID)}

	b, err := ack.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	m.SendToRadio(ctx, radio.Conn, mme.S1APProcedureHandoverCancelAcknowledge, b)
}
