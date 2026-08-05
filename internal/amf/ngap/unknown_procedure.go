// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// respondToUnknownProcedure answers a decoded message naming a procedure this
// AMF does not implement, on the criticality the sender assigned it
// (TS 38.413 §10.3.4.1). Ignore draws no reply.
func respondToUnknownProcedure(ctx context.Context, ran *amf.Radio, pdu ngap.PDU) {
	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		// A successful or unsuccessful outcome answers a procedure the AMF never
		// initiated, so there is nothing to reject: it is left to local error
		// handling.
		logger.From(ctx, ran.Log).Warn("ignoring unsupported procedure outcome")

		return
	}

	var cause int

	switch im.Criticality {
	case ngap.CriticalityReject:
		cause = ngap.CauseProtocolAbstractSyntaxErrorReject
	case ngap.CriticalityNotify:
		cause = ngap.CauseProtocolAbstractSyntaxErrorIgnoreAndNotify
	default:
		return
	}

	logger.From(ctx, ran.Log).Warn("unsupported initiating procedure",
		zap.String("procedure", im.ProcedureCode.String()))

	proc := im.ProcedureCode
	trigger := ngap.TriggeringInitiatingMessage
	crit := im.Criticality

	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		Cause: &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: cause},
		CriticalityDiagnostics: &ngap.CriticalityDiagnostics{
			ProcedureCode:        &proc,
			TriggeringMessage:    &trigger,
			ProcedureCriticality: &crit,
		},
	})
}

// sendProtocolErrorIndication answers octets that did not decode at all, where
// no procedure or IE can be named (TS 38.413 §10.2).
func sendProtocolErrorIndication(ctx context.Context, ran *amf.Radio, cause int) {
	emitErrorIndication(ctx, ran, &ngap.ErrorIndication{
		Cause: &ngap.Cause{Group: ngap.CauseGroupProtocol, Value: cause},
	})
}
