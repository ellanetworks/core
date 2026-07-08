// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
)

var gmmTracer = otel.Tracer("ella-core/amf/nas/handler")

// HandleGmmMessage dispatches an inbound GMM message to its handler. integrityVerified
// is true only when the message carried a verified MAC; it is false when the decoder
// admitted the message without verified integrity (plain, or MAC-failed but whitelisted).
func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, integrityVerified bool) {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		handleRegistrationRequest(ctx, amfInstance, ue, msg, integrityVerified)
	case nas.MsgTypeServiceRequest:
		handleServiceRequest(ctx, amfInstance, ue, msg.ServiceRequest, integrityVerified)
	case nas.MsgTypeULNASTransport:
		handleULNASTransport(ctx, amfInstance, ue, msg.ULNASTransport)
	case nas.MsgTypeConfigurationUpdateComplete:
		handleConfigurationUpdateComplete(amfInstance, ue)
	case nas.MsgTypeNotificationResponse:
		handleNotificationResponse(ctx, amfInstance, ue, msg.NotificationResponse)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration, integrityVerified)
	case nas.MsgTypeStatus5GMM:
		handleStatus5GMM(ctx, ue, msg.Status5GMM)
	case nas.MsgTypeIdentityResponse:
		handleIdentityResponse(ctx, amfInstance, ue, msg.IdentityResponse, integrityVerified)
	case nas.MsgTypeAuthenticationResponse:
		handleAuthenticationResponse(ctx, amfInstance, ue, msg.AuthenticationResponse)
	case nas.MsgTypeAuthenticationFailure:
		handleAuthenticationFailure(ctx, amfInstance, ue, msg.AuthenticationFailure)
	case nas.MsgTypeSecurityModeComplete:
		handleSecurityModeComplete(ctx, amfInstance, ue, msg.SecurityModeComplete, integrityVerified)
	case nas.MsgTypeSecurityModeReject:
		handleSecurityModeReject(ctx, ue, msg.SecurityModeReject)
	case nas.MsgTypeRegistrationComplete:
		handleRegistrationComplete(ctx, amfInstance, ue)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		handleDeregistrationAccept(ctx, ue)
	default:
		// TS 24.501 §7.4: a message type not implemented by the receiver is ignored, but a
		// 5GMM STATUS with cause #97 "message type non-existent or not implemented" should
		// be returned (mirrors the MME's DispatchEMM default).
		logger.From(ctx, logger.AmfLog).Warn("unhandled GMM message", logger.MessageType(amf.GmmMessageTypeName(msgType)))
		sendStatus5GMM(ctx, ue, nasMessage.Cause5GMMMessageTypeNonExistentOrNotImplemented)
	}
}
