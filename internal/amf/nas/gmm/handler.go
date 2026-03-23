package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("ella-core/amf/nas/handler")

func HandleGmmMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, msg *nas.GmmMessage) error {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, amfInstance, ue, msg)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, amfInstance, ue, msg.ServiceRequest)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, amfInstance, ue, msg.ULNASTransport)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(amfInstance, ue)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, amfInstance, ue, msg.NotificationResponse)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ue, msg.Status5GMM)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, amfInstance, ue, msg.IdentityResponse)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, amfInstance, ue, msg.AuthenticationResponse)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, amfInstance, ue, msg.AuthenticationFailure)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, amfInstance, ue, msg.SecurityModeComplete)
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
