// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.opentelemetry.io/otel"
)

var gmmTracer = otel.Tracer("ella-core/amf/nas/handler")

// HandleGmmMessage dispatches an inbound GMM message to its handler. integrityVerified
// is true only when the message carried a verified MAC; it is false when the decoder
// admitted the message without verified integrity (plain, or MAC-failed but whitelisted).
func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msgType uint8, plain []byte, integrityVerified bool) nasreply.Disposition {
	switch fgs.MessageType(msgType) {
	case fgs.MsgRegistrationRequest:
		return handleRegistrationRequest(ctx, amfInstance, ue, plain, integrityVerified)
	case fgs.MsgULNASTransport:
		return handleULNASTransport(ctx, amfInstance, ue, plain)
	case fgs.MsgConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(amfInstance, ue)
	case fgs.MsgNotificationResponse:
		return handleNotificationResponse(ctx, amfInstance, ue, plain)
	case fgs.MsgDeregistrationRequestUEOrig:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, plain, integrityVerified)
	case fgs.MsgGMMStatus:
		return handleStatus5GMM(ctx, ue, plain)
	case fgs.MsgIdentityResponse:
		return handleIdentityResponse(ctx, amfInstance, ue, plain, integrityVerified)
	case fgs.MsgAuthenticationResponse:
		return handleAuthenticationResponse(ctx, amfInstance, ue, plain)
	case fgs.MsgAuthenticationFailure:
		return handleAuthenticationFailure(ctx, amfInstance, ue, plain)
	case fgs.MsgSecurityModeComplete:
		return handleSecurityModeComplete(ctx, amfInstance, ue, plain, integrityVerified)
	case fgs.MsgSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, plain)
	case fgs.MsgRegistrationComplete:
		return handleRegistrationComplete(ctx, amfInstance, ue)
	case fgs.MsgDeregistrationAcceptUETerm:
		return handleDeregistrationAccept(ctx, ue)
	default:
		// TS 24.501 §7.4: a message type not implemented by the receiver is ignored, but a
		// 5GMM STATUS with cause #97 "message type non-existent or not implemented" should
		// be returned.
		logger.From(ctx, logger.AmfLog).Warn("unhandled GMM message", logger.MessageType(amf.GmmMessageTypeName(msgType)))
		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}
}
