// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// HandleNAS is the MME's EMM entry point for an inbound NAS message on a UE
// connection.
func HandleNAS(ctx context.Context, m *mme.MME, conn *mme.UeConn, pdu []byte) {
	dispositionForNAS(ctx, m, conn, pdu).Finalize(ctx, egress{conn: conn})
}

// dispositionForNAS resolves an inbound NAS PDU to the single outcome the finalizer applies:
// a message the MME cannot process draws the STATUS the spec mandates or an audited silence,
// never a bare drop.
func dispositionForNAS(ctx context.Context, m *mme.MME, conn *mme.UeConn, pdu []byte) nasreply.Disposition {
	ue := conn.UeContext()
	if ue == nil {
		switch peekEMMMessageType(pdu) {
		case eps.MsgAttachRequest:
			ue = mme.NewUeContext()
			m.AttachUeConn(ue, conn)
		case eps.MsgTrackingAreaUpdateRequest:
			resolved, plain := recoverContextFrom5GS(ctx, m, conn, pdu)
			if resolved == nil {
				return nasreply.Silent(nasreply.ReasonNoContext)
			}

			ueConn := resolved.Conn()
			if ueConn == nil {
				return nasreply.Silent(nasreply.ReasonNoContext)
			}

			return HandleEmmMessage(ctx, m, resolved, ueConn, plain, true)
		default:
			return nasreply.Silent(nasreply.ReasonNoContext)
		}
	}

	if !ue.Secured() {
		resolved, drop := resolveAttachContext(ctx, m, ue, conn, pdu)
		if drop {
			return nasreply.Silent(nasreply.ReasonUnspecified)
		}

		ue = resolved
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	ctx = logger.Into(ctx, ueConn.Log())

	pd, err := eps.PeekProtocolDiscriminator(pdu)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read NAS protocol discriminator", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	if pd != eps.PDEMM {
		logger.From(ctx, logger.MmeLog).Debug("ignoring standalone ESM NAS message")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	result, err := mme.DecodeNASMessage(ue, pdu)
	if err != nil {
		return mme.DispositionForDecodeError(err)
	}

	return HandleEmmMessage(ctx, m, ue, ueConn, result.Plain, result.IntegrityVerified)
}

// HandleEmmMessage routes a plain NAS message to its procedure handler and reports the single
// outcome the ingress finalizer applies.
func HandleEmmMessage(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, plain []byte, integrityVerified bool) nasreply.Disposition {
	msg, err := eps.ParseMessage(plain, nas.DirectionUplink)
	if err != nil && !nas.SoftOnly(err) {
		return rejectUndecodable(ctx, m, ue, ueConn, plain, err)
	}

	if !decoded(ctx, messageName(msg), err) {
		return rejectUndecodable(ctx, m, ue, ueConn, plain, err)
	}

	ctx, span := mme.Tracer.Start(ctx, "pdu/receive",
		trace.WithAttributes(attribute.String("nas.message_type", messageName(msg))))
	defer span.End()

	switch msg := msg.(type) {
	case *eps.AttachRequest:
		return handleAttachRequest(ctx, m, ue, ueConn, msg, plain, integrityVerified)
	case *eps.IdentityResponse:
		return handleIdentityResponse(ctx, m, ue, ueConn, msg)
	case *eps.AuthenticationResponse:
		return handleAuthenticationResponse(ctx, m, ue, ueConn, msg)
	case *eps.AuthenticationFailure:
		return handleAuthenticationFailure(ctx, m, ue, ueConn, msg)
	case *eps.SecurityModeComplete:
		return handleSecurityModeComplete(ctx, m, ue, ueConn, msg)
	case *eps.SecurityModeReject:
		return handleSecurityModeReject(ctx, m, ue, ueConn, msg)
	case *eps.AttachComplete:
		return handleAttachComplete(ctx, m, ue, ueConn, msg)
	case *eps.GUTIReallocationComplete:
		return handleGUTIReallocationComplete(ctx, m, ue, ueConn)
	case *eps.DetachRequestUE:
		return handleDetachRequest(ctx, m, ue, ueConn, msg, integrityVerified)
	case *eps.DetachAccept:
		return handleDetachAccept(ctx, m, ue, ueConn)
	case *eps.TrackingAreaUpdateRequest:
		return handleTrackingAreaUpdate(ctx, m, ue, ueConn, msg, plain)
	case *eps.TrackingAreaUpdateComplete:
		return handleTrackingAreaUpdateComplete(ctx, m, ue, ueConn)
	case *eps.EMMStatus:
		return handleEMMStatus(msg)
	case *eps.UnknownEMMMessage:
		logger.From(ctx, logger.MmeLog).Warn("unimplemented NAS message type", zap.Stringer("message", msg))

		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	default:
		return handleESMMessage(ctx, m, ue, ueConn, msg)
	}
}

// messageName names a decoded message for logs and traces, falling back to the
// type octet of one this MME does not model.
func messageName(msg eps.Message) string {
	switch msg := msg.(type) {
	case eps.EMMMessage:
		return msg.MessageType().String()
	case eps.ESMMessage:
		return msg.MessageType().String()
	case *eps.ServiceRequest:
		return "SERVICE REQUEST"
	default:
		return "unknown message"
	}
}

// rejectUndecodable answers a message whose mandatory part did not decode. The
// answer depends on the message: TS 24.301 §5.5.1.2.7 b) rejects a malformed
// ATTACH REQUEST with EMM cause #96, and §7.5.1 answers anything else with an
// EMM STATUS carrying the same cause.
func rejectUndecodable(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, plain []byte, err error) nasreply.Disposition {
	logger.From(ctx, logger.MmeLog).Warn("failed to decode NAS message", zap.Error(err))

	if mt, perr := eps.PeekMessageType(plain); perr == nil && mt == eps.MsgAttachRequest {
		rejectAttach(ctx, m, ue, ueConn, eps.EMMCauseInvalidMandatoryInformation)

		return nasreply.Handled()
	}

	return nasreply.StatusMM(nasreply.CauseInvalidMandatoryInfo)
}

func peekEMMMessageType(pdu []byte) eps.MessageType {
	pd, err := eps.PeekProtocolDiscriminator(pdu)
	if err != nil || pd != eps.PDEMM {
		return 0
	}

	body, ok := unprotectedBody(pdu)
	if !ok {
		return 0
	}

	mt, err := eps.PeekMessageType(body)
	if err != nil {
		return 0
	}

	return mt
}
