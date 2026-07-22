// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel"
)

var gmmTracer = otel.Tracer("ella-core/amf/nas/handler")

// HandleGmmMessage dispatches an inbound GMM message to its handler. integrityVerified
// is true only when the message carried a verified MAC; it is false when the decoder
// admitted the message without verified integrity (plain, or MAC-failed but whitelisted).
func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, plain []byte, integrityVerified bool) nasreply.Disposition {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, amfInstance, ue, msg, integrityVerified)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, amfInstance, ue, msg.ULNASTransport)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(amfInstance, ue)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, amfInstance, ue, msg.NotificationResponse)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration, integrityVerified)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ctx, ue, plain)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, amfInstance, ue, msg.IdentityResponse, integrityVerified)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, amfInstance, ue, msg.AuthenticationResponse)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, amfInstance, ue, msg.AuthenticationFailure)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, amfInstance, ue, msg.SecurityModeComplete, integrityVerified)
	case nas.MsgTypeSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, plain)
	case nas.MsgTypeRegistrationComplete:
		return handleRegistrationComplete(ctx, amfInstance, ue)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return handleDeregistrationAccept(ctx, ue)
	default:
		// TS 24.501 §7.4: a message type not implemented by the receiver is ignored, but a
		// 5GMM STATUS with cause #97 "message type non-existent or not implemented" should
		// be returned.
		logger.From(ctx, logger.AmfLog).Warn("unhandled GMM message", logger.MessageType(amf.GmmMessageTypeName(msgType)))
		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}
}
