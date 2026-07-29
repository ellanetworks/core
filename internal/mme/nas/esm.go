// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// handleESMMessage routes a decoded ESM message, or answers a type this MME does
// not implement with an ESM STATUS (TS 24.301 §7.4).
func handleESMMessage(ctx context.Context, m *mme.MME, ue *mme.UeContext, msg eps.Message) nasreply.Disposition {
	ctx, span := mme.Tracer.Start(ctx, "mme/esm",
		trace.WithAttributes(attribute.String("esm.message_type", messageName(msg))))
	defer span.End()

	switch msg := msg.(type) {
	case *eps.PDNConnectivityRequest:
		return handlePDNConnectivityRequest(ctx, m, ue, msg)
	case *eps.PDNDisconnectRequest:
		return handlePDNDisconnectRequest(ctx, m, ue, msg)
	case *eps.BearerResourceAllocationRequest:
		return handleBearerResourceAllocationRequest(ctx, ue, msg)
	case *eps.BearerResourceModificationRequest:
		return handleBearerResourceModificationRequest(ctx, ue, msg)
	case *eps.ActivateDefaultEPSBearerContextAccept:
		return handleActivateDefaultBearerAccept(m, ue, msg)
	case *eps.ActivateDefaultEPSBearerContextReject:
		return handleActivateDefaultBearerReject(ctx, m, ue, msg)
	case *eps.DeactivateEPSBearerContextAccept:
		return handleDeactivateBearerAccept(ctx, m, ue, msg)
	case *eps.ModifyEPSBearerContextAccept:
		return handleModifyBearerAccept(m, ue, msg)
	case *eps.ModifyEPSBearerContextReject:
		return handleModifyBearerReject(m, ue, msg)
	case *eps.ESMStatus:
		return handleESMStatus(ctx, m, ue, msg)
	case eps.ESMMessage:
		logger.From(ctx, logger.MmeLog).Warn("unhandled ESM message", zap.String("message-type", messageName(msg)))

		return nasreply.StatusSM(nasreply.CauseMessageTypeNotImplemented)
	default:
		// An EMM message type this MME does not implement (TS 24.301 §7.4).
		logger.From(ctx, logger.MmeLog).Warn("unhandled EMM message", zap.String("message-type", messageName(msg)))

		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}
}
