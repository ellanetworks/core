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
func HandleNAS(m *mme.MME, ctx context.Context, conn *mme.UeConn, nas []byte) {
	dispositionForNAS(m, ctx, conn, nas).Finalize(ctx, egress{conn: conn})
}

// dispositionForNAS resolves an inbound NAS PDU to the single outcome the finalizer applies:
// a message the MME cannot process draws the STATUS the spec mandates or an audited silence,
// never a bare drop.
func dispositionForNAS(m *mme.MME, ctx context.Context, conn *mme.UeConn, nas []byte) nasreply.Disposition {
	ue := conn.UeContext()
	if ue == nil {
		// A bare connection binds a persistent context only for an ATTACH REQUEST —
		// the only message warranting one (TS 24.301) — so an unauthenticated peer
		// cannot exhaust UE contexts. A connection left bare here is released by the
		// S1AP layer.
		if !isInitialAttach(nas) {
			// A first message that is not an ATTACH REQUEST resolved no context and cannot be
			// processed, but a message the MME can still classify draws an EMM STATUS
			// (TS 24.301 §7.4 / §7.5.1) rather than a silent drop.
			return dispositionForUnresolved(nas)
		}

		ue = mme.NewUeContext()
		m.AttachUeConn(ue, conn)
	}

	// Resolve-first: for an as-yet-unsecured context (a fresh Attach), a native GUTI
	// that verifies against a held EPS security context adopts it before decode, so
	// everything below runs on the right context.
	if !ue.Secured() {
		resolved, drop := resolveAttachContext(m, ctx, ue, nas)
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

	return HandleEmmMessage(m, ctx, ue, result.Plain, result.IntegrityVerified)
}

// HandleEmmMessage routes a plain NAS message to its procedure handler and reports the single
// outcome the ingress finalizer applies.
func HandleEmmMessage(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) nasreply.Disposition {
	if len(plain) > 0 && plain[0]&0x0F == eps.PDESM {
		return handleESM(m, ctx, ue, plain)
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
		return handleAttachRequest(m, ctx, ue, plain, integrityVerified)
	case eps.MsgIdentityResponse:
		return handleIdentityResponse(m, ctx, ue, plain)
	case eps.MsgAuthenticationResponse:
		return handleAuthenticationResponse(m, ctx, ue, plain)
	case eps.MsgAuthenticationFailure:
		return handleAuthenticationFailure(m, ctx, ue, plain)
	case eps.MsgSecurityModeComplete:
		return handleSecurityModeComplete(m, ctx, ue, plain)
	case eps.MsgSecurityModeReject:
		return handleSecurityModeReject(m, ctx, ue, plain)
	case eps.MsgAttachComplete:
		return handleAttachComplete(m, ctx, ue, plain)
	case eps.MsgDetachRequest:
		return handleDetachRequest(m, ctx, ue, plain, integrityVerified)
	case eps.MsgDetachAccept:
		return handleDetachAccept(m, ctx, ue)
	case eps.MsgTrackingAreaUpdateRequest:
		return handleTrackingAreaUpdate(m, ctx, ue, plain)
	case eps.MsgTrackingAreaUpdateComplete:
		return handleTrackingAreaUpdateComplete(m, ctx, ue)
	case eps.MsgEMMStatus:
		return handleEMMStatus(plain)
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

// dispositionForUnresolved classifies a fresh-connection EPS NAS PDU that resolved no EMM
// context, so the finalizer answers the STATUS the spec mandates (TS 24.301 §7.4 / §7.5.1)
// instead of a bare drop: a decodable EMM message whose type cannot be read → EMM STATUS #96
// (§7.5.1); a non-EMM, ciphered-without-context, or unactionable message → EMM STATUS #97
// (§7.4); a PDU too short to carry a message type → an audited silence (§7.2.1). A fresh
// connection has no secure exchange, so §4.4.4.3's silent discard does not apply — the
// message is never processed, only answered, so an unauthenticated peer gains nothing.
func dispositionForUnresolved(nas []byte) nasreply.Disposition {
	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil {
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	if pd != eps.PDEMM {
		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}

	body := nas

	switch nas[0] >> 4 {
	case uint8(eps.SHTPlain):
	case uint8(eps.SHTIntegrityProtected), uint8(eps.SHTIntegrityProtectedNewContext):
		if len(nas) < 6 {
			return nasreply.Silent(nasreply.ReasonTooShort)
		}

		body = nas[6:]
	default:
		// Ciphered/reserved: with no context the MME cannot decrypt or classify the body.
		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}

	if _, err := eps.PeekMessageType(body); err != nil {
		return nasreply.StatusMM(nasreply.CauseInvalidMandatoryInfo)
	}

	return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
}

// isInitialAttach reports whether a fresh connection's first NAS message is an
// ATTACH REQUEST — the only message warranting a new UE context (TS 24.301). A
// ciphered or non-EMM message cannot be an initial attach the network can act on,
// so only a plain or integrity-protected (peekable) body matches.
func isInitialAttach(nas []byte) bool {
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
