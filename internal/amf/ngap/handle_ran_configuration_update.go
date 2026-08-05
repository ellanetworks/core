// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleRANConfigurationUpdate applies an NG-RAN node's configuration update.
// Per TS 38.413 §8.7.2.2 an absent IE leaves the corresponding configuration
// unchanged, so a name- or DRX-only update is accepted: only a present Supported
// TA List is validated and committed, and one broadcasting no served TAI is
// answered with a Failure (§8.7.2.3).
func HandleRANConfigurationUpdate(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, req *ngap.RANConfigurationUpdate) {
	// Only a Supported TA List needs validating against what this AMF serves;
	// §8.7.2.2 leaves everything else alone, so an update that carries no TAs
	// is accepted without consulting the operator configuration at all.
	var operatorInfo *amf.OperatorInfo

	if len(req.SupportedTAList) > 0 {
		var err error

		operatorInfo, err = amfInstance.OperatorInfo(ctx)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Could not get operator info", zap.Error(err))
			sendRANConfigurationUpdateFailure(ctx, ran, causeUnspecified, nil)

			return
		}
	}

	tais, outBytes, accepted, reason, err := ranConfigUpdateOutcomeFor(req, operatorInfo)
	if err != nil {
		// §8.7.2.3 obliges an answer whenever the AMF cannot accept the update,
		// which includes being unable to build its own response.
		logger.WithTrace(ctx, ran.Log).Error("failed to handle RAN Configuration Update", zap.Error(err))
		sendRANConfigurationUpdateFailure(ctx, ran, causeUnspecified, nil)

		return
	}

	if !accepted {
		ran.SendToRadio(ctx, amf.NGAPProcedureRANConfigurationUpdateFailure, outBytes)

		logger.WithTrace(ctx, ran.Log).Warn("RAN Configuration Update rejected",
			zap.String("reason", reason),
			zap.Any("gnb_tai_list", tais),
			zap.Any("core_tai_list", operatorInfo.Tais))

		return
	}

	// Commit only what the message carried: §8.7.2.2 leaves everything else as
	// it was, so an absent IE must not clear the stored configuration.
	if name := ranNodeName(req.RANNodeName); name != "" {
		amfInstance.UpdateRadioName(ran, name)
	}

	if len(req.SupportedTAList) > 0 {
		amfInstance.UpdateRadioSupportedTAIs(ran, tais)
	}

	// §8.7.2.2: "If the Global RAN Node ID IE is included ... the AMF shall
	// associate the TNLA to the NG-C interface instance using the Global RAN
	// Node ID." Re-keying leaves UE contexts alone, as §8.7.2.1 requires.
	if req.GlobalRANNodeID != nil && !amfInstance.RebindRanID(ran, *req.GlobalRANNodeID) {
		logger.WithTrace(ctx, ran.Log).Warn("RAN Configuration Update names a Global RAN Node ID held by another association",
			zap.Any("global-ran-node-id", req.GlobalRANNodeID))
	}

	ran.SendToRadio(ctx, amf.NGAPProcedureRANConfigurationUpdateAcknowledge, outBytes)

	logger.WithTrace(ctx, ran.Log).Info("RAN Configuration Update acknowledged",
		zap.String("gnb-name", ran.NodeName()))
}

// ranConfigUpdateOutcomeFor returns an Acknowledge when the update may be taken
// into use, otherwise a Failure (TS 38.413 §8.7.2.3). reason is a
// human-readable rejection summary, empty when accepted; tais is what the gNB
// broadcasts, which the caller commits to the Radio only on accept.
//
// An update carrying no Supported TA List is always accepted: it changes
// something else, and §8.7.2.2 leaves the stored TAs alone. operatorInfo is
// consulted only in that case and so may be nil otherwise.
func ranConfigUpdateOutcomeFor(req *ngap.RANConfigurationUpdate, operatorInfo *amf.OperatorInfo) (tais []amf.SupportedTAI, out []byte, accepted bool, reason string, err error) {
	if len(req.SupportedTAList) > 0 {
		tais = supportedTAIs(req.SupportedTAList)

		if cause, ok := servedTAICause(tais, operatorInfo); !ok {
			out, err = (&ngap.RANConfigurationUpdateFailure{Cause: &cause}).Marshal()
			if err != nil {
				return tais, nil, false, "", fmt.Errorf("amf: marshal RAN Configuration Update Failure: %w", err)
			}

			reason = "gNB broadcasts no PLMN served by this AMF (Unknown PLMN)"
			if cause == causeNoServedTAC {
				reason = "gNB broadcasts a served PLMN but no TAC served by this AMF"
			}

			if len(tais) == 0 {
				reason = "gNB broadcasts no supported TA"
			}

			return tais, out, false, reason, nil
		}
	}

	ack := &ngap.RANConfigurationUpdateAcknowledge{}

	// §10.3.4.2 reports in the response message of the procedure where it has one.
	if diag := req.Diagnostics(); diag.ReportRequired() {
		ack.CriticalityDiagnostics = &ngap.CriticalityDiagnostics{
			ProcedureCriticality:      ngap.Ptr(ngap.ProcedureCriticality(ngap.ProcRANConfigurationUpdate)),
			IEsCriticalityDiagnostics: diag.Report(),
		}
	}

	out, err = ack.Marshal()
	if err != nil {
		return tais, nil, false, "", fmt.Errorf("amf: marshal RAN Configuration Update Acknowledge: %w", err)
	}

	return tais, out, true, "", nil
}

// sendRANConfigurationUpdateFailure answers with a RAN CONFIGURATION UPDATE
// FAILURE carrying cause and, where the rejection answers a protocol error, the
// per-IE diagnostics §10.3.5 wants.
func sendRANConfigurationUpdateFailure(ctx context.Context, ran *amf.Radio, cause ngap.Cause, diag *ngap.CriticalityDiagnostics) {
	pkt, err := (&ngap.RANConfigurationUpdateFailure{Cause: &cause, CriticalityDiagnostics: diag}).Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building RAN Configuration Update Failure", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, amf.NGAPProcedureRANConfigurationUpdateFailure, pkt)
}

// sendRANConfigurationUpdateProtocolFailure rejects an update that must not
// reach the application. The procedure defines an unsuccessful outcome, so
// TS 38.413 §10.3.5 reports the abstract syntax error there rather than in an
// Error Indication.
func sendRANConfigurationUpdateProtocolFailure(ctx context.Context, ran *amf.Radio, ase *ngap.AbstractSyntaxError) {
	diag := ase.OutcomeDiagnostics()

	sendRANConfigurationUpdateFailure(ctx, ran, ase.Cause, &diag)

	logger.WithTrace(ctx, ran.Log).Warn("RAN Configuration Update rejected", zap.Error(ase))
}
