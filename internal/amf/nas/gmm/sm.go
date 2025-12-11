package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("ella-core/amf/nas/handler")

func HandleGmmMessage(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	msgType := msg.GetMessageType()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, ue, msg)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, ue, msg)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, ue, msg)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(ctx, ue)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, ue, msg)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ctx, ue, msg)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, ue, msg)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, ue, msg)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, ue, msg)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, ue, msg)
	case nas.MsgTypeSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, msg)
	case nas.MsgTypeRegistrationComplete:
		return handleRegistrationComplete(ctx, ue)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return handleDeregistrationAccept(ctx, ue)
	default:
		return fmt.Errorf("message type %d handling not implemented", msg.GetMessageType())
	}
}
