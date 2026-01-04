package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("ella-core/amf/nas/handler")

func HandleGmmMessage(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, amf, ue, msg)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, amf, ue, msg.ServiceRequest)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, amf, ue, msg.ULNASTransport)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(ue)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, ue, msg.NotificationResponse)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg.DeregistrationRequestUEOriginatingDeregistration)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ue, msg.Status5GMM)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, amf, ue, msg.IdentityResponse)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, amf, ue, msg.AuthenticationResponse)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, amf, ue, msg.AuthenticationFailure)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, amf, ue, msg.SecurityModeComplete)
	case nas.MsgTypeSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, msg.SecurityModeReject)
	case nas.MsgTypeRegistrationComplete:
		return handleRegistrationComplete(ctx, ue)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return handleDeregistrationAccept(ctx, ue)
	default:
		return fmt.Errorf("message type %d handling not implemented", msgType)
	}
}
