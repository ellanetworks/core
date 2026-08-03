// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
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

	// Only Supported TAs need validating against what this MME serves; §8.7.4.2
	// leaves everything else alone, so an update that carries none is accepted
	// without consulting the operator configuration at all.
	var (
		plmn s1ap.PLMNIdentity
		tacs []uint16
	)

	if len(req.SupportedTAs) > 0 {
		var err error

		plmn, tacs, err = servedPLMNAndTACs(ctx, m)
		if err != nil {
			logger.From(ctx, radio.Log).Error("Could not get operator info", zap.Error(err))
			sendENBConfigurationUpdateFailure(m, ctx, radio, causeUnspecified, nil)

			return
		}
	}

	tais, out, accepted, reason, err := enbConfigUpdateOutcomeFor(req, plmn, tacs)
	if err != nil {
		// §8.7.4.3 obliges an answer whenever the MME cannot accept the update,
		// which includes being unable to build its own response.
		logger.From(ctx, radio.Log).Error("failed to handle ENB Configuration Update", zap.Error(err))
		sendENBConfigurationUpdateFailure(m, ctx, radio, causeUnspecified, nil)

		return
	}

	if !accepted {
		m.SendToRadio(ctx, radio.Conn, mme.S1APProcedureENBConfigUpdateFailure, out)

		logger.From(ctx, radio.Log).Warn("ENB Configuration Update rejected",
			zap.String("reason", reason),
			zap.Any("enb_tai_list", tais),
			zap.Any("core_tac_list", tacs))

		return
	}

	// Commit only what the message carried: §8.7.4.2 leaves everything else as
	// it was, so an absent IE must not clear the stored configuration.
	if name := enbName(req.ENBName); name != "" {
		m.UpdateRadioName(radio, name)
	}

	if len(req.SupportedTAs) > 0 {
		m.UpdateRadioSupportedTAs(radio, tais)
	}

	m.SendToRadio(ctx, radio.Conn, mme.S1APProcedureENBConfigUpdateAck, out)

	logger.From(ctx, radio.Log).Info("ENB Configuration Update acknowledged",
		zap.String("enb-name", radio.NodeName()))
}

// servedPLMNAndTACs reads the operator configuration this MME serves, encoded
// as S1AP carries it.
func servedPLMNAndTACs(ctx context.Context, m *mme.MME) (s1ap.PLMNIdentity, []uint16, error) {
	operator, err := m.Operator(ctx)
	if err != nil {
		return s1ap.PLMNIdentity{}, nil, fmt.Errorf("mme: get operator: %w", err)
	}

	plmn, err := mme.EncodePLMN(operator.PLMN())
	if err != nil {
		return s1ap.PLMNIdentity{}, nil, fmt.Errorf("mme: encode served PLMN: %w", err)
	}

	tacs, err := operator.TACs()
	if err != nil {
		return s1ap.PLMNIdentity{}, nil, fmt.Errorf("mme: get operator TACs: %w", err)
	}

	return plmn, tacs, nil
}

// sendENBConfigurationUpdateFailure answers with an ENB CONFIGURATION UPDATE
// FAILURE carrying cause and, where the rejection answers a protocol error, the
// per-IE diagnostics §10.3.5 wants. §8.7.4.3 obliges a response whenever the MME
// cannot accept the update, including when it cannot read its own configuration.
func sendENBConfigurationUpdateFailure(m *mme.MME, ctx context.Context, radio *mme.Radio, cause s1ap.Cause, diag *s1ap.CriticalityDiagnostics) {
	pkt, err := (&s1ap.ENBConfigurationUpdateFailure{Cause: &cause, CriticalityDiagnostics: diag}).Marshal()
	if err != nil {
		logger.From(ctx, radio.Log).Error("error building ENB Configuration Update Failure", zap.Error(err))
		return
	}

	m.SendToRadio(ctx, radio.Conn, mme.S1APProcedureENBConfigUpdateFailure, pkt)
}

// rejectENBConfigurationUpdate answers an undecodable update with ENB
// CONFIGURATION UPDATE FAILURE, falling back to the Error Indication procedure
// where the outcome cannot be built (TS 36.413 §10.3.5).
func rejectENBConfigurationUpdate(m *mme.MME, ctx context.Context, radio *mme.Radio, err error) {
	rejectWithFailure(m, ctx, radio.Conn, s1ap.ProcENBConfigurationUpdate, err,
		func(cause s1ap.Cause, diag *s1ap.CriticalityDiagnostics) ([]byte, error) {
			return (&s1ap.ENBConfigurationUpdateFailure{Cause: &cause, CriticalityDiagnostics: diag}).Marshal()
		}, mme.S1APProcedureENBConfigUpdateFailure)
}

// enbConfigUpdateOutcomeFor returns an Acknowledge when the update may be taken
// into use, otherwise a Failure (TS 36.413 §8.7.4.3). reason is a
// human-readable rejection summary, empty when accepted; tais is what the eNB
// broadcasts, which the caller commits to the Radio only on accept.
//
// An update carrying no Supported TAs is always accepted: it changes something
// else, and §8.7.4.2 leaves the stored TAs alone. plmn and tacs are consulted
// only in that case and so may be zero otherwise.
func enbConfigUpdateOutcomeFor(req *s1ap.ENBConfigurationUpdate, plmn s1ap.PLMNIdentity, tacs []uint16) (tais []mme.SupportedTAI, out []byte, accepted bool, reason string, err error) {
	if len(req.SupportedTAs) > 0 {
		tais = mme.EnbSupportedTAIs(req.SupportedTAs)

		if cause, ok := servedTAICause(req.SupportedTAs, plmn, tacs); !ok {
			out, err = (&s1ap.ENBConfigurationUpdateFailure{Cause: s1ap.Ptr(cause)}).Marshal()
			if err != nil {
				return tais, nil, false, "", fmt.Errorf("mme: marshal ENB Configuration Update Failure: %w", err)
			}

			reason = "eNB broadcasts no PLMN served by this MME (Unknown PLMN)"
			if cause == causeNoServedTAC {
				reason = "eNB broadcasts a served PLMN but no TAC served by this MME"
			}

			if len(tais) == 0 {
				reason = "eNB broadcasts no supported TA"
			}

			return tais, out, false, reason, nil
		}
	}

	ack := &s1ap.ENBConfigurationUpdateAcknowledge{}

	// §10.3.4.2 reports in the response message of the procedure where it has one.
	if diag := req.Diagnostics(); diag.ReportRequired() {
		ack.CriticalityDiagnostics = &s1ap.CriticalityDiagnostics{
			ProcedureCriticality:      s1ap.Ptr(s1ap.ProcedureCriticality(s1ap.ProcENBConfigurationUpdate)),
			IEsCriticalityDiagnostics: diag.Report(),
		}
	}

	out, err = ack.Marshal()
	if err != nil {
		return tais, nil, false, "", fmt.Errorf("mme: marshal ENB Configuration Update Acknowledge: %w", err)
	}

	return tais, out, true, "", nil
}
