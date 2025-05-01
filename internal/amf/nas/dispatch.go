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
	case 0x00:
		return "SecurityHeaderTypePlainNas"
	case 0x01:
		return "SecurityHeaderTypeIntegrityProtected"
	case 0x02:
		return "SecurityHeaderTypeIntegrityProtectedAndCiphered"
	case 0x03:
		return "SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext"
	case 0x04:
		return "SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext"
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
