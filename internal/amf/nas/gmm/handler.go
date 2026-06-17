// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

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
// macFailed is set when the message arrived with a failed integrity check
// but was admitted by the decoder verdict (plain message allowed, etc.);
// handlers that vary their response based on integrity status receive it.
func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, msg *nas.GmmMessage, macFailed bool) error {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, amfInstance, ue, msg, macFailed)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, amfInstance, ue, msg.ServiceRequest, macFailed)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, amfInstance, ue, msg.ULNASTransport, macFailed)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(amfInstance, ue, macFailed)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, amfInstance, ue, msg.NotificationResponse, macFailed)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration, macFailed)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ue, msg.Status5GMM, macFailed)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, amfInstance, ue, msg.IdentityResponse, macFailed)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, amfInstance, ue, msg.AuthenticationResponse)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, amfInstance, ue, msg.AuthenticationFailure)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, amfInstance, ue, msg.SecurityModeComplete, macFailed)
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
