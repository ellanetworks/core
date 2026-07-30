// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleENBConfigurationUpdate validates any updated supported TAs against the
// served PLMN/TAC and acknowledges, or fails the update (TS 36.413 §8.7.4).
func handleENBConfigurationUpdate(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	req, err := s1ap.ParseENBConfigurationUpdate(value)
	if err != nil {
		logger.From(ctx, radio.Log).Warn("failed to decode ENB Configuration Update", zap.Error(err))
		rejectENBConfigurationUpdate(m, ctx, radio, err)

		return
	}

	reportDiagnostics(m, radio.Conn, s1ap.ProcENBConfigurationUpdate, req.Diagnostics())

	operator, err := m.Operator(ctx)
	if err != nil {
		logger.From(ctx, radio.Log).Error("failed to get operator for ENB Configuration Update", zap.Error(err))
		return
	}

	plmn := operator.PLMN()

	tacs, err := operator.TACs()
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

	m.SendToRadio(ctx, radio.Conn, msgType, out)

	if !accepted {
		logger.From(ctx, radio.Log).Warn("ENB Configuration Update rejected: eNB broadcasts no TAI (PLMN + TAC) served by this MME")
		return
	}

	if req.ENBName != nil {
		m.UpdateRadioName(radio, *req.ENBName)
	}

	if len(req.SupportedTAs) > 0 {
		m.UpdateRadioSupportedTAs(radio, mme.EnbSupportedTAIs(req.SupportedTAs))
	}

	logger.From(ctx, radio.Log).Info("ENB Configuration Update acknowledged", zap.String("enb-name", enbName(req.ENBName)))
}

// rejectENBConfigurationUpdate answers an undecodable update with ENB
// CONFIGURATION UPDATE FAILURE. TS 36.413 §10.3.4.2, §10.3.5 and §10.3.6 all
// reject using the procedure's unsuccessful outcome, which is always
// constructible here, so the Error Indication fallback does not apply.
func rejectENBConfigurationUpdate(m *mme.MME, ctx context.Context, radio *mme.Radio, err error) {
	fail := &s1ap.ENBConfigurationUpdateFailure{
		Cause: new(s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolTransferSyntaxError}),
	}

	if ase, ok := errors.AsType[*s1ap.AbstractSyntaxError](err); ok {
		diag := ase.OutcomeDiagnostics()
		fail.Cause, fail.CriticalityDiagnostics = &ase.Cause, &diag
	}

	out, err := fail.Marshal()
	if err != nil {
		logger.From(ctx, radio.Log).Error("failed to marshal ENB Configuration Update Failure", zap.Error(err))

		return
	}

	m.SendToRadio(ctx, radio.Conn, mme.S1APProcedureENBConfigUpdateFailure, out)
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
			out, err = (&s1ap.ENBConfigurationUpdateFailure{Cause: s1ap.Ptr(cause)}).Marshal()
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
