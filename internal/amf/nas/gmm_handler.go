// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var gmmTracer = otel.Tracer("ella-core/amf/nas/handler")

// HandleGmmMessage dispatches an inbound GMM message to its handler. integrityVerified
// is true only when the message carried a verified MAC; it is false when the decoder
// admitted the message without verified integrity (plain, or MAC-failed but whitelisted).
func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msgType uint8, plain []byte, integrityVerified bool) nasreply.Disposition {
	msg, err := fgs.ParseMessage(plain)
	if err != nil && !nas.SoftOnly(err) {
		logger.From(ctx, logger.AmfLog).Warn("failed to decode GMM message",
			logger.MessageType(amf.GmmMessageTypeName(msgType)), zap.Error(err))

		return nasreply.StatusMM(amf.GmmDecodeFailureCause(plain))
	}

	if !decoded(ctx, amf.GmmMessageTypeName(msgType), err) {
		return nasreply.StatusMM(amf.GmmDecodeFailureCause(plain))
	}

	switch msg := msg.(type) {
	case *fgs.RegistrationRequest:
		return handleRegistrationRequest(ctx, amfInstance, ue, msg, plain, integrityVerified)
	case *fgs.ULNASTransport:
		return handleULNASTransport(ctx, amfInstance, ue, msg)
	case *fgs.ConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(amfInstance, ue)
	case *fgs.NotificationResponse:
		return handleNotificationResponse(ctx, amfInstance, ue, msg)
	case *fgs.DeregistrationRequestUEOriginating:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg, integrityVerified)
	case *fgs.GMMStatus:
		return handleStatus5GMM(ctx, ue, msg)
	case *fgs.IdentityResponse:
		return handleIdentityResponse(ctx, amfInstance, ue, msg, integrityVerified)
	case *fgs.AuthenticationResponse:
		return handleAuthenticationResponse(ctx, amfInstance, ue, msg)
	case *fgs.AuthenticationFailure:
		return handleAuthenticationFailure(ctx, amfInstance, ue, msg)
	case *fgs.SecurityModeComplete:
		return handleSecurityModeComplete(ctx, amfInstance, ue, msg, integrityVerified)
	case *fgs.SecurityModeReject:
		return handleSecurityModeReject(ctx, ue, msg)
	case *fgs.RegistrationComplete:
		return handleRegistrationComplete(ctx, amfInstance, ue)
	case *fgs.DeregistrationAcceptUETerminated:
		// The UE answers a network-initiated de-registration (TS 24.501 §8.2.15);
		// §8.2.13 is the accept this AMF sends, and never one it receives.
		return handleDeregistrationAccept(ctx, ue)
	default:
		// TS 24.501 §7.4: a message type not implemented by the receiver is ignored, but a
		// 5GMM STATUS with cause #97 "message type non-existent or not implemented" should
		// be returned.
		logger.From(ctx, logger.AmfLog).Warn("unhandled GMM message", logger.MessageType(amf.GmmMessageTypeName(msgType)))

		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}
}
