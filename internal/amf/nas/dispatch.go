// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	ctxt "context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/gmm"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/free5gc/nas"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/nas")

func Dispatch(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, procedureCode int64, msg *nas.Message) error {
	if msg.GmmMessage == nil {
		return errors.New("gmm message is nil")
	}

	if msg.GsmMessage != nil {
		return errors.New("gsm message is not nil")
	}

	if ue.State[accessType] == nil {
		return fmt.Errorf("ue state is empty for access type: %v", accessType)
	}

	msgTypeName := MessageName(msg.GmmMessage.GmmHeader.GetMessageType())
	spanName := fmt.Sprintf("AMF NAS %s", msgTypeName)

	_, span := tracer.Start(ctx, spanName,
		trace.WithAttributes(
			attribute.String("nas.accessType", string(accessType)),
			attribute.Int64("nas.procedureCode", procedureCode),
			attribute.String("nas.messageType", msgTypeName),
		),
	)
	defer span.End()

	return gmm.GmmFSM.SendEvent(ctx, ue.State[accessType], gmm.GmmMessageEvent, fsm.ArgsType{
		gmm.ArgAmfUe:         ue,
		gmm.ArgAccessType:    accessType,
		gmm.ArgNASMessage:    msg.GmmMessage,
		gmm.ArgProcedureCode: procedureCode,
	})
}

func MessageName(code uint8) string {
	switch code {
	case nas.MsgTypeRegistrationRequest:
		return "RegistrationRequest"
	case nas.MsgTypeRegistrationAccept:
		return "RegistrationAccept"
	case nas.MsgTypeRegistrationComplete:
		return "RegistrationComplete"
	case nas.MsgTypeRegistrationReject:
		return "RegistrationReject"
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return "DeregistrationRequestUEOriginatingDeregistration"
	case nas.MsgTypeDeregistrationAcceptUEOriginatingDeregistration:
		return "DeregistrationAcceptUEOriginatingDeregistration"
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		return "DeregistrationRequestUETerminatedDeregistration"
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return "DeregistrationAcceptUETerminatedDeregistration"
	case nas.MsgTypeServiceRequest:
		return "ServiceRequest"
	case nas.MsgTypeServiceReject:
		return "ServiceReject"
	case nas.MsgTypeServiceAccept:
		return "ServiceAccept"
	case nas.MsgTypeConfigurationUpdateCommand:
		return "ConfigurationUpdateCommand"
	case nas.MsgTypeConfigurationUpdateComplete:
		return "ConfigurationUpdateComplete"
	case nas.MsgTypeAuthenticationRequest:
		return "AuthenticationRequest"
	case nas.MsgTypeAuthenticationResponse:
		return "AuthenticationResponse"
	case nas.MsgTypeAuthenticationReject:
		return "AuthenticationReject"
	case nas.MsgTypeAuthenticationFailure:
		return "AuthenticationFailure"
	case nas.MsgTypeAuthenticationResult:
		return "AuthenticationResult"
	case nas.MsgTypeIdentityRequest:
		return "IdentityRequest"
	case nas.MsgTypeIdentityResponse:
		return "IdentityResponse"
	case nas.MsgTypeSecurityModeCommand:
		return "SecurityModeCommand"
	case nas.MsgTypeSecurityModeComplete:
		return "SecurityModeComplete"
	case nas.MsgTypeSecurityModeReject:
		return "SecurityModeReject"
	case nas.MsgTypeStatus5GMM:
		return "Status5GMM"
	case nas.MsgTypeNotification:
		return "Notification"
	case nas.MsgTypeNotificationResponse:
		return "NotificationResponse"
	case nas.MsgTypeULNASTransport:
		return "ULNASTransport"
	case nas.MsgTypeDLNASTransport:
		return "DLNASTransport"
	default:
		return fmt.Sprintf("Unknown message type: %d", code)
	}
}
