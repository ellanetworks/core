// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleENBConfigurationUpdate validates any updated supported TAs against the
// served PLMN/TAC, stores an updated eNB name, and acknowledges, or fails the
// update (TS 36.413 §8.7.4).
func handleENBConfigurationUpdate(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	req, err := s1ap.ParseENBConfigurationUpdate(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode ENB Configuration Update", zap.Error(err))
		return
	}

	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		logger.From(ctx, radio.Log).Error("failed to get operator PLMN for ENB Configuration Update", zap.Error(err))
		return
	}

	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		logger.From(ctx, radio.Log).Error("failed to get operator TACs for ENB Configuration Update", zap.Error(err))
		return
	}

	out, accepted, err := enbConfigUpdateOutcomeFor(req, plmn, tacs)
	if err != nil {
		logger.From(ctx, radio.Log).Error("failed to handle ENB Configuration Update", zap.Error(err))
		return
	}

	msgType := mme.S1APProcedureENBConfigUpdateAck
	if !accepted {
		msgType = mme.S1APProcedureENBConfigUpdateFailure
	}

	if _, err := radio.Conn.WriteMsg(out, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
		logger.From(ctx, radio.Log).Error("failed to send ENB Configuration Update response", zap.Error(err))
		return
	}

	m.LogNetworkEvent(ctx, radio.Conn, msgType, logger.DirectionOutbound, out)

	if !accepted {
		logger.From(ctx, radio.Log).Warn("ENB Configuration Update rejected: eNB broadcasts no TAI (PLMN + TAC) served by this MME")
		return
	}

	if req.ENBName != "" {
		m.UpdateRadioName(radio, req.ENBName)
	}

	if len(req.SupportedTAs) > 0 {
		m.UpdateRadioSupportedTAs(radio, mme.EnbSupportedTAIs(req.SupportedTAs))
	}

	logger.From(ctx, radio.Log).Info("ENB Configuration Update acknowledged", zap.String("enb-name", req.ENBName))
}

// enbConfigUpdateOutcomeFor returns an Acknowledge when any updated supported TAs
// still broadcast a served TAI, otherwise a Failure (TS 36.413 §8.7.4). An update
// carrying no supported TAs (a name- or DRX-only change) is always accepted.
func enbConfigUpdateOutcomeFor(req *s1ap.ENBConfigurationUpdate, plmn models.PlmnID, tacs []uint16) (out []byte, accepted bool, err error) {
	if len(req.SupportedTAs) > 0 {
		served, err := mme.EncodePLMN(plmn)
		if err != nil {
			return nil, false, fmt.Errorf("mme: encode served PLMN: %w", err)
		}

		cause, ok := servedTAICause(req.SupportedTAs, served, tacs)
		if !ok {
			out, err = (&s1ap.ENBConfigurationUpdateFailure{Cause: cause}).Marshal()
			if err != nil {
				return nil, false, fmt.Errorf("mme: marshal ENB Configuration Update Failure: %w", err)
			}

			return out, false, nil
		}
	}

	out, err = (&s1ap.ENBConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		return nil, false, fmt.Errorf("mme: marshal ENB Configuration Update Acknowledge: %w", err)
	}

	return out, true, nil
}
