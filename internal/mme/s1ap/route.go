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

// Route dispatches a decoded S1AP PDU — UE-associated and node-level (past the S1 Setup
// gate) — to its procedure handler (TS 36.413).
func Route(ctx context.Context, m *mme.MME, radio *mme.Radio, pdu any) {
	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		switch p.ProcedureCode {
		case s1ap.ProcInitialUEMessage:
			HandleInitialUEMessage(ctx, m, radio, p.Value)
		case s1ap.ProcUplinkNASTransport:
			handleUplinkNASTransport(ctx, m, radio, p.Value)
		case s1ap.ProcNASNonDeliveryIndication:
			handleNASNonDeliveryIndication(ctx, m, radio, p.Value)
		case s1ap.ProcUEContextReleaseRequest:
			handleUEContextReleaseRequest(ctx, m, radio, p.Value)
		case s1ap.ProcUECapabilityInfoIndication:
			handleUECapabilityInfoIndication(ctx, m, radio, p.Value)
		case s1ap.ProcPathSwitchRequest:
			handlePathSwitchRequest(ctx, m, radio, p.Value)
		case s1ap.ProcHandoverPreparation:
			handleHandoverRequired(ctx, m, radio, p.Value)
		case s1ap.ProcHandoverNotification:
			handleHandoverNotify(ctx, m, radio, p.Value)
		case s1ap.ProcENBStatusTransfer:
			handleENBStatusTransfer(ctx, m, radio, p.Value)
		case s1ap.ProcHandoverCancel:
			handleHandoverCancel(ctx, m, radio, p.Value)
		case s1ap.ProcErrorIndication:
			handleErrorIndication(ctx, m, radio, p.Value)
		case s1ap.ProcReset:
			handleReset(ctx, m, radio, p.Value)
		case s1ap.ProcENBConfigurationUpdate:
			handleENBConfigurationUpdate(ctx, m, radio, p.Value)
		case s1ap.ProcENBConfigurationTransfer:
			handleENBConfigurationTransfer(ctx, m, radio, p.Value)
		case s1ap.ProcERABModificationIndication:
			handleERABModificationIndication(ctx, m, radio, p.Value)
		case s1ap.ProcUplinkUEAssociatedLPPaTransport:
			handleUplinkLPPaTransport(ctx, m, radio, p.Value)
		case s1ap.ProcLocationReport:
			handleLocationReport(ctx, m, radio, p.Value)
		default:
			logger.From(ctx, radio.Log()).Warn("unsupported initiating procedure", zap.Int64("procedureCode", int64(p.ProcedureCode)))
			respondToUnknownProcedure(ctx, m, radio.Conn, p)
		}
	case *s1ap.SuccessfulOutcome:
		switch p.ProcedureCode {
		case s1ap.ProcInitialContextSetup:
			handleInitialContextSetupResponse(ctx, m, radio, p.Value)
		case s1ap.ProcUEContextRelease:
			HandleUEContextReleaseComplete(ctx, m, radio, p.Value)
		case s1ap.ProcERABSetup:
			HandleERABSetupResponse(ctx, m, radio, p.Value)
		case s1ap.ProcERABModify:
			handleERABModifyResponse(ctx, m, radio, p.Value)
		case s1ap.ProcERABRelease:
			HandleERABReleaseResponse(ctx, m, radio, p.Value)
		case s1ap.ProcHandoverResourceAllocation:
			handleHandoverRequestAcknowledge(ctx, m, radio, p.Value)
		case s1ap.ProcMMEConfigurationUpdate:
			handleMMEConfigurationUpdateAcknowledge(ctx, m, radio, p.Value)
		default:
			logger.From(ctx, radio.Log()).Warn("ignoring unsupported procedure", zap.String("kind", "successful-outcome"), zap.Int64("procedureCode", int64(p.ProcedureCode)))
		}
	case *s1ap.UnsuccessfulOutcome:
		switch p.ProcedureCode {
		case s1ap.ProcInitialContextSetup:
			handleInitialContextSetupFailure(ctx, m, radio, p.Value)
		case s1ap.ProcHandoverResourceAllocation:
			handleHandoverFailure(ctx, m, radio, p.Value)
		case s1ap.ProcMMEConfigurationUpdate:
			handleMMEConfigurationUpdateFailure(ctx, m, radio, p.Value)
		default:
			logger.From(ctx, radio.Log()).Warn("ignoring unsupported procedure", zap.String("kind", "unsuccessful-outcome"), zap.Int64("procedureCode", int64(p.ProcedureCode)))
		}
	default:
		logger.From(ctx, radio.Log()).Warn("ignoring unsupported procedure", zap.String("kind", "unknown-pdu"))
	}
}
