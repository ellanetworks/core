// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// HandleNAS is the MME's EMM entry point for an inbound NAS message on a UE
// connection.
func HandleNAS(ctx context.Context, m *mme.MME, conn *mme.UeConn, nas []byte) {
	dispositionForNAS(ctx, m, conn, nas).Finalize(ctx, egress{conn: conn})
}

// dispositionForNAS resolves an inbound NAS PDU to the single outcome the finalizer applies:
// a message the MME cannot process draws the STATUS the spec mandates or an audited silence,
// never a bare drop.
func dispositionForNAS(ctx context.Context, m *mme.MME, conn *mme.UeConn, nas []byte) nasreply.Disposition {
	ue := conn.UeContext()
	if ue == nil {
		// A bare connection binds a persistent context only for an ATTACH REQUEST —
		// the only message warranting one (TS 24.301) — so an unauthenticated peer
		// cannot exhaust UE contexts. A connection left bare here is released by the
		// S1AP layer.
		if !isAttachRequest(nas) {
			return nasreply.Silent(nasreply.ReasonNoContext)
		}

		ue = mme.NewUeContext()
		m.AttachUeConn(ue, conn)
	}

	// Resolve-first: for an as-yet-unsecured context (a fresh Attach), a native GUTI
	// that verifies against a held EPS security context adopts it before decode, so
	// everything below runs on the right context.
	if !ue.Secured() {
		resolved, drop := resolveAttachContext(ctx, m, ue, nas)
		if drop {
			return nasreply.Silent(nasreply.ReasonUnspecified)
		}

		ue = resolved
	}

	if conn := ue.Conn(); conn != nil && conn.Log != nil {
		ctx = logger.Into(ctx, conn.Log)
	}

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read NAS protocol discriminator", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	if pd != eps.PDEMM {
		logger.From(ctx, logger.MmeLog).Debug("ignoring standalone ESM NAS message")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	result, err := mme.DecodeNASMessage(ue, nas)
	if err != nil {
		return mme.DispositionForDecodeError(err)
	}

	return HandleEmmMessage(ctx, m, ue, result.Plain, result.IntegrityVerified)
}

// HandleEmmMessage routes a plain NAS message to its procedure handler and reports the single
// outcome the ingress finalizer applies.
func HandleEmmMessage(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	if len(plain) > 0 && plain[0]&0x0F == eps.PDESM {
		return handleESM(ctx, m, ue, plain)
	}

	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read EMM message type", zap.Error(err))
		return nasreply.StatusMM(nasreply.CauseProtocolErrorUnspecified)
	}

	ctx, span := mme.Tracer.Start(ctx, "nas/receive",
		trace.WithAttributes(attribute.String("nas.message_type", mme.EmmMessageTypeName(mt))))
	defer span.End()

	switch mt {
	case eps.MsgAttachRequest:
		return handleAttachRequest(ctx, m, ue, plain, integrityVerified)
	case eps.MsgIdentityResponse:
		return handleIdentityResponse(ctx, m, ue, plain)
	case eps.MsgAuthenticationResponse:
		return handleAuthenticationResponse(ctx, m, ue, plain)
	case eps.MsgAuthenticationFailure:
		return handleAuthenticationFailure(ctx, m, ue, plain)
	case eps.MsgSecurityModeComplete:
		return handleSecurityModeComplete(ctx, m, ue, plain)
	case eps.MsgSecurityModeReject:
		return handleSecurityModeReject(ctx, m, ue, plain)
	case eps.MsgAttachComplete:
		return handleAttachComplete(ctx, m, ue, plain)
	case eps.MsgGUTIReallocationComplete:
		return handleGUTIReallocationComplete(ctx, m, ue, plain)
	case eps.MsgDetachRequest:
		return handleDetachRequest(ctx, m, ue, plain, integrityVerified)
	case eps.MsgDetachAccept:
		return handleDetachAccept(ctx, m, ue)
	case eps.MsgTrackingAreaUpdateRequest:
		return handleTrackingAreaUpdate(ctx, m, ue, plain)
	case eps.MsgTrackingAreaUpdateComplete:
		return handleTrackingAreaUpdateComplete(ctx, m, ue)
	case eps.MsgEMMStatus:
		return handleEMMStatus(plain)
	case eps.MsgULGenericNASTransport:
		return handleULGenericNASTransport(ctx, m, ue, plain)
	default:
		// TS 24.301 §7.4: a message type not implemented by the receiver is ignored, but an
		// EMM STATUS with cause #97 "message type non-existent or not implemented" should be
		// returned.
		logger.From(ctx, logger.MmeLog).Warn("unhandled EMM message",
			zap.String("message-type", mme.EmmMessageTypeName(mt)),
			zap.Int("message-type-value", int(mt)))

		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}
}

func isAttachRequest(nas []byte) bool {
	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil || pd != eps.PDEMM {
		return false
	}

	body := nas

	switch nas[0] >> 4 {
	case uint8(eps.SHTPlain):
	case uint8(eps.SHTIntegrityProtected), uint8(eps.SHTIntegrityProtectedNewContext):
		if len(nas) < 6 {
			return false
		}

		body = nas[6:]
	default:
		return false
	}

	mt, err := eps.PeekMessageType(body)

	return err == nil && mt == eps.MsgAttachRequest
}

// mobileIdentityDigits extracts the identity digits from a TS 24.008 Mobile
// identity value (first digit in the high nibble of octet 0, the rest packed
// BCD). It serves any BCD identity — IMSI, IMEI, or IMEISV.
func mobileIdentityDigits(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return string([]byte{'0' + (b[0] >> 4)}) + nascommon.DecodeTBCD(b[1:])
}
