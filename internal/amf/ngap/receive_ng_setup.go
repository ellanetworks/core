// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ngSetupRequestMessageType is what the network event log calls the message,
// matching what getMessageType returns for the free5gc-decoded procedures.
const ngSetupRequestMessageType = "NGSetupRequest"

// handleNGSetup answers an NG Setup Request using the in-house NGAP codec, and
// reports whether it consumed the message. It returns false for anything that
// is not an NG Setup Request, including octets that do not decode at all — the
// caller's existing path reports those.
func handleNGSetup(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg []byte, span trace.Span) bool {
	pdu, err := ngap.Unmarshal(msg)
	if err != nil {
		return false
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcNGSetup {
		return false
	}

	span.SetAttributes(
		attribute.String("ngap.message_type", ngSetupRequestMessageType),
		attribute.Int("ngap.message_size", len(msg)),
		attribute.String("network.protocol.name", "ngap"),
		attribute.String("network.transport", "sctp"),
	)

	req, parseErr := ngap.ParseNGSetupRequest(im.Value)

	// The peer's RAN node name is applied before the inbound event is logged,
	// so the log keeps chronological order with the outbound response.
	if parseErr == nil && req.RANNodeName != nil && *req.RANNodeName != "" {
		amfInstance.UpdateRadioName(ran, *req.RANNodeName)
	}

	amfInstance.LogNetworkEvent(ctx, ran.Conn, ngSetupRequestMessageType, logger.DirectionInbound, msg)

	if parseErr != nil {
		span.RecordError(parseErr)
		span.SetStatus(codes.Error, "failed to decode NG Setup Request")

		var ase *ngap.AbstractSyntaxError
		if errors.As(parseErr, &ase) {
			sendNGSetupProtocolFailure(ctx, ran, ase)

			return true
		}

		// Octets that decoded as an NG Setup envelope but not as its body:
		// there is nothing to cite in Criticality Diagnostics (§10.2).
		logger.From(ctx, ran.Log).Error("NG Setup Request decode error", zap.Error(parseErr))
		sendProtocolErrorIndication(ctx, ran, ngapType.CauseProtocolPresentTransferSyntaxError)

		return true
	}

	HandleNGSetupRequest(ctx, amfInstance, ran, req)

	return true
}
