// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	ctx "context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/gmm"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/omec-project/nas"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func messageTypeName(code uint8) string {
	switch code {
	case 65:
		return "MsgTypeRegistrationRequest"
	case 66:
		return "MsgTypeRegistrationAccept"
	case 67:
		return "MsgTypeRegistrationComplete"
	case 68:
		return "MsgTypeRegistrationReject"
	case 69:
		return "MsgTypeDeregistrationRequestUEOriginatingDeregistration"
	case 70:
		return "MsgTypeDeregistrationAcceptUEOriginatingDeregistration"
	case 71:
		return "MsgTypeDeregistrationRequestUETerminatedDeregistration"
	case 72:
		return "MsgTypeDeregistrationAcceptUETerminatedDeregistration"
	case 76:
		return "MsgTypeServiceRequest"
	case 77:
		return "MsgTypeServiceReject"
	case 78:
		return "MsgTypeServiceAccept"
	case 84:
		return "MsgTypeConfigurationUpdateCommand"
	case 85:
		return "MsgTypeConfigurationUpdateComplete"
	case 86:
		return "MsgTypeAuthenticationRequest"
	case 87:
		return "MsgTypeAuthenticationResponse"
	case 88:
		return "MsgTypeAuthenticationReject"
	case 89:
		return "MsgTypeAuthenticationFailure"
	case 90:
		return "MsgTypeAuthenticationResult"
	case 91:
		return "MsgTypeIdentityRequest"
	case 92:
		return "MsgTypeIdentityResponse"
	case 93:
		return "MsgTypeSecurityModeCommand"
	case 94:
		return "MsgTypeSecurityModeComplete"
	case 95:
		return "MsgTypeSecurityModeReject"
	case 100:
		return "MsgTypeStatus5GMM"
	case 101:
		return "MsgTypeNotification"
	case 102:
		return "MsgTypeNotificationResponse"
	case 103:
		return "MsgTypeULNASTransport"
	case 104:
		return "MsgTypeDLNASTransport"
	default:
		return fmt.Sprintf("Unknown message type: %d", code)
	}
}

func Dispatch(ctext ctx.Context, ue *context.AmfUe, accessType models.AccessType, procedureCode int64, msg *nas.Message) error {
	if msg.GmmMessage == nil {
		return errors.New("gmm message is nil")
	}

	if msg.GsmMessage != nil {
		return errors.New("gsm message is not nil")
	}

	if ue.State[accessType] == nil {
		return fmt.Errorf("ue state is empty for access type: %v", accessType)
	}

	msgTypeName := messageTypeName(msg.GmmMessage.GmmHeader.GetMessageType())
	spanName := fmt.Sprintf("nas.%s", msgTypeName)

	_, span := tracer.Start(ctext, spanName,
		trace.WithAttributes(
			attribute.String("nas.accessType", string(accessType)),
			attribute.Int64("nas.procedureCode", procedureCode),
			attribute.String("nas.messageType", msgTypeName),
		),
	)
	defer span.End()

	return gmm.GmmFSM.SendEvent(ue.State[accessType], gmm.GmmMessageEvent, fsm.ArgsType{
		gmm.ArgAmfUe:         ue,
		gmm.ArgAccessType:    accessType,
		gmm.ArgNASMessage:    msg.GmmMessage,
		gmm.ArgProcedureCode: procedureCode,
	})
}
