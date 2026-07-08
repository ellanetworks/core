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

// Route dispatches a decoded UE-associated S1AP PDU to its procedure handler
// (TS 36.413).
func Route(m *mme.MME, ctx context.Context, radio *mme.Radio, pdu any) {
	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		switch p.ProcedureCode {
		case s1ap.ProcInitialUEMessage:
			HandleInitialUEMessage(m, ctx, radio, p.Value)
		case s1ap.ProcUplinkNASTransport:
			handleUplinkNASTransport(m, ctx, radio, p.Value)
		case s1ap.ProcNASNonDeliveryIndication:
			handleNASNonDeliveryIndication(m, ctx, radio, p.Value)
		case s1ap.ProcUEContextReleaseRequest:
			handleUEContextReleaseRequest(m, ctx, radio, p.Value)
		case s1ap.ProcUECapabilityInfoIndication:
			handleUECapabilityInfoIndication(m, radio, p.Value)
		case s1ap.ProcPathSwitchRequest:
			handlePathSwitchRequest(m, ctx, radio, p.Value)
		case s1ap.ProcHandoverPreparation:
			handleHandoverRequired(m, ctx, radio, p.Value)
		case s1ap.ProcHandoverNotification:
			handleHandoverNotify(m, ctx, radio, p.Value)
		case s1ap.ProcENBStatusTransfer:
			handleENBStatusTransfer(m, ctx, radio, p.Value)
		case s1ap.ProcHandoverCancel:
			handleHandoverCancel(m, ctx, radio, p.Value)
		case s1ap.ProcErrorIndication:
			handleErrorIndication(m, ctx, radio, p.Value)
		case s1ap.ProcReset:
			handleReset(m, radio, p.Value)
		default:
			logger.From(ctx, logger.MmeLog).Debug("ignoring S1AP initiating message", zap.Int("procedure-code", int(p.ProcedureCode)))
		}
	case *s1ap.SuccessfulOutcome:
		switch p.ProcedureCode {
		case s1ap.ProcInitialContextSetup:
			handleInitialContextSetupResponse(m, ctx, radio, p.Value)
		case s1ap.ProcUEContextRelease:
			HandleUEContextReleaseComplete(m, ctx, radio, p.Value)
		case s1ap.ProcERABSetup:
			HandleERABSetupResponse(m, ctx, radio, p.Value)
		case s1ap.ProcERABModify:
			handleERABModifyResponse(m, p.Value)
		case s1ap.ProcERABRelease:
			HandleERABReleaseResponse(m, radio, p.Value)
		case s1ap.ProcHandoverResourceAllocation:
			handleHandoverRequestAcknowledge(m, ctx, radio, p.Value)
		default:
			logger.From(ctx, logger.MmeLog).Debug("ignoring S1AP successful outcome", zap.Int("procedure-code", int(p.ProcedureCode)))
		}
	case *s1ap.UnsuccessfulOutcome:
		switch p.ProcedureCode {
		case s1ap.ProcInitialContextSetup:
			handleInitialContextSetupFailure(m, radio, p.Value)
		case s1ap.ProcHandoverResourceAllocation:
			handleHandoverFailure(m, ctx, radio, p.Value)
		default:
			logger.From(ctx, logger.MmeLog).Debug("ignoring S1AP unsuccessful outcome", zap.Int("procedure-code", int(p.ProcedureCode)))
		}
	default:
		logger.From(ctx, logger.MmeLog).Debug("ignoring S1AP PDU")
	}
}
