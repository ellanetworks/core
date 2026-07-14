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

func handleESM(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	mt, err := eps.PeekESMMessageType(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read ESM message type", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	ctx, span := mme.Tracer.Start(ctx, "mme/esm",
		trace.WithAttributes(attribute.Int("esm.message_type", int(mt))))
	defer span.End()

	switch mt {
	case eps.MsgPDNConnectivityRequest:
		return handlePDNConnectivityRequest(ctx, m, ue, plain)
	case eps.MsgPDNDisconnectRequest:
		return handlePDNDisconnectRequest(ctx, m, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextAccept:
		return handleActivateDefaultBearerAccept(m, ue, plain)
	case eps.MsgActivateDefaultEPSBearerContextReject:
		return handleActivateDefaultBearerReject(ctx, m, ue, plain)
	case eps.MsgDeactivateEPSBearerContextAccept:
		return handleDeactivateBearerAccept(ctx, m, ue, plain)
	case eps.MsgModifyEPSBearerContextAccept:
		return handleModifyBearerAccept(m, ue, plain)
	case eps.MsgModifyEPSBearerContextReject:
		return handleModifyBearerReject(m, ue, plain)
	case eps.MsgESMStatus:
		return handleESMStatus(ctx, m, ue, plain)
	default:
		// TS 24.301 §7.4: an unimplemented ESM message type is answered with an ESM STATUS.
		logger.From(ctx, logger.MmeLog).Warn("unhandled ESM message", zap.Int("message-type-value", int(mt)))
		return nasreply.StatusSM(nasreply.CauseMessageTypeNotImplemented)
	}
}
