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
		// A bare connection binds a persistent context only for an ATTACH REQUEST —
		// the only message warranting one (TS 24.301) — so an unauthenticated peer
		// cannot exhaust UE contexts. A connection left bare here is released by the
		// S1AP layer.
		if !isAttachRequest(pdu) {
			return nasreply.Silent(nasreply.ReasonNoContext)
		}

		ue = mme.NewUeContext()
		m.AttachUeConn(ue, conn)
	}

	// Resolve-first: for an as-yet-unsecured context (a fresh Attach), a native GUTI
	// that verifies against a held EPS security context adopts it before decode, so
	// everything below runs on the right context.
	if !ue.Secured() {
		resolved, drop := resolveAttachContext(ctx, m, ue, pdu)
		if drop {
			return nasreply.Silent(nasreply.ReasonUnspecified)
		}

		ue = resolved
	}

	if conn := ue.Conn(); conn != nil && conn.Log != nil {
		ctx = logger.Into(ctx, conn.Log)
	}

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

	return HandleEmmMessage(ctx, m, ue, result.Plain, result.IntegrityVerified)
}

// HandleEmmMessage routes a plain NAS message to its procedure handler and reports the single
// outcome the ingress finalizer applies.
func HandleEmmMessage(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	msg, err := eps.ParseMessage(plain, nas.DirectionUplink)
	if err != nil && !nas.SoftOnly(err) {
		return rejectUndecodable(ctx, m, ue, plain, err)
	}

	if !decoded(ctx, messageName(msg), err) {
		return rejectUndecodable(ctx, m, ue, plain, err)
	}

	ctx, span := mme.Tracer.Start(ctx, "pdu/receive",
		trace.WithAttributes(attribute.String("nas.message_type", messageName(msg))))
	defer span.End()

	switch msg := msg.(type) {
	case *eps.AttachRequest:
		return handleAttachRequest(ctx, m, ue, msg, plain, integrityVerified)
	case *eps.IdentityResponse:
		return handleIdentityResponse(ctx, m, ue, msg)
	case *eps.AuthenticationResponse:
		return handleAuthenticationResponse(ctx, m, ue, msg)
	case *eps.AuthenticationFailure:
		return handleAuthenticationFailure(ctx, m, ue, msg)
	case *eps.SecurityModeComplete:
		return handleSecurityModeComplete(ctx, m, ue, msg)
	case *eps.SecurityModeReject:
		return handleSecurityModeReject(ctx, m, ue, msg)
	case *eps.AttachComplete:
		return handleAttachComplete(ctx, m, ue)
	case *eps.GUTIReallocationComplete:
		return handleGUTIReallocationComplete(ctx, m, ue)
	case *eps.DetachRequestUE:
		return handleDetachRequest(ctx, m, ue, msg, integrityVerified)
	case *eps.DetachAccept:
		return handleDetachAccept(ctx, m, ue)
	case *eps.TrackingAreaUpdateRequest:
		return handleTrackingAreaUpdate(ctx, m, ue, msg, plain)
	case *eps.TrackingAreaUpdateComplete:
		return handleTrackingAreaUpdateComplete(ctx, m, ue)
	case *eps.EMMStatus:
		return handleEMMStatus(msg)
	case *eps.UnknownEMMMessage:
		// TS 24.301 §7.4: a message type the receiver does not implement draws a
		// STATUS in its own protocol. The ESM counterpart reaches
		// handleESMMessage through the default arm below.
		logger.From(ctx, logger.MmeLog).Warn("unimplemented NAS message type", zap.Stringer("message", msg))

		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	default:
		return handleESMMessage(ctx, m, ue, msg)
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
func rejectUndecodable(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte, err error) nasreply.Disposition {
	logger.From(ctx, logger.MmeLog).Warn("failed to decode NAS message", zap.Error(err))

	if mt, perr := eps.PeekMessageType(plain); perr == nil && mt == eps.MsgAttachRequest {
		rejectAttach(ctx, m, ue, eps.EMMCauseInvalidMandatoryInformation)

		return nasreply.Handled()
	}

	return nasreply.StatusMM(nasreply.CauseInvalidMandatoryInfo)
}

func isAttachRequest(pdu []byte) bool {
	pd, err := eps.PeekProtocolDiscriminator(pdu)
	if err != nil || pd != eps.PDEMM {
		return false
	}

	sht, err := eps.PeekSecurityHeaderType(pdu)
	if err != nil {
		return false
	}

	body := pdu

	switch sht {
	case eps.SHTPlain:
	case eps.SHTIntegrityProtected, eps.SHTIntegrityProtectedNewContext:
		if len(pdu) < 6 {
			return false
		}

		body = pdu[6:]
	default:
		return false
	}

	mt, err := eps.PeekMessageType(body)

	return err == nil && mt == eps.MsgAttachRequest
}
