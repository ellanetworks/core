// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("ella-core/amf/nas/handler")

// HandleGmmMessage dispatches an inbound GMM message to its handler.
// integrityVerified is true when the message was integrity-protected and its MAC
// verified; it is false when the message was admitted without verified integrity
// by the decoder verdict (plain or mac-failed but whitelisted). Handlers that
// vary their response based on integrity status receive it.
func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nas.GmmMessage, integrityVerified bool) error {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, amfInstance, ue, msg, integrityVerified)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, amfInstance, ue, msg.ServiceRequest, integrityVerified)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, amfInstance, ue, msg.ULNASTransport, integrityVerified)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(amfInstance, ue, integrityVerified)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, amfInstance, ue, msg.NotificationResponse, integrityVerified)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration, integrityVerified)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ue, msg.Status5GMM, integrityVerified)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, amfInstance, ue, msg.IdentityResponse, integrityVerified)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, amfInstance, ue, msg.AuthenticationResponse)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, amfInstance, ue, msg.AuthenticationFailure)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, amfInstance, ue, msg.SecurityModeComplete, integrityVerified)
	case nas.MsgTypeSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, msg.SecurityModeReject)
	case nas.MsgTypeRegistrationComplete:
		return handleRegistrationComplete(ctx, amfInstance, ue)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return handleDeregistrationAccept(ctx, ue)
	default:
		return fmt.Errorf("message type %d handling not implemented", msgType)
	}
}
