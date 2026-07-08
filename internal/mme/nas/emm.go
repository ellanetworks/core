// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// HandleNAS is the MME's EMM entry point for an inbound NAS message on a UE
// connection. A bare connection's first message binds a fresh persistent context
// here, at the NAS layer (mirrors the AMF's HandleNAS); it then unwraps NAS
// security when the message is protected and routes the plain message to its
// procedure handler.
func HandleNAS(m *mme.MME, ctx context.Context, conn *mme.UeConn, nas []byte) {
	ue := conn.UeContext()
	if ue == nil {
		// A bare connection binds a persistent context only for an ATTACH REQUEST —
		// the only message warranting one (TS 24.301) — so an unauthenticated peer
		// cannot exhaust UE contexts (mirrors the AMF's registration-only mint gate).
		// A connection left bare here is released by the S1AP layer.
		if !isInitialAttach(nas) {
			return
		}

		ue = mme.NewUeContext()
		m.AttachUeConn(ue, conn)
	}

	// Resolve-first: for an as-yet-unsecured context (a fresh Attach), a native GUTI
	// that verifies against a held EPS security context adopts it before decode, so
	// everything below runs on the right context (mirrors the AMF). An established
	// (secured) connection skips this and decodes normally.
	if !ue.Secured() {
		resolved, drop := resolveAttachContext(m, ctx, ue, nas)
		if drop {
			return
		}

		ue = resolved
	}

	if conn := ue.Conn(); conn != nil && conn.Log != nil {
		ctx = logger.Into(ctx, conn.Log)
	}

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read NAS protocol discriminator", zap.Error(err))
		return
	}

	if pd != eps.PDEMM {
		logger.From(ctx, logger.MmeLog).Debug("ignoring standalone ESM NAS message")
		return
	}

	result, err := mme.DecodeNASMessage(ue, nas)
	if err != nil {
		// DecodeNASMessage has logged the reason; the PDU is dropped.
		return
	}

	HandleEmmMessage(m, ctx, ue, result.Plain, result.IntegrityVerified)
}

// HandleEmmMessage routes a plain NAS message to its procedure handler, splitting ESM
// session-management messages from EMM mobility messages by protocol
// discriminator.
func HandleEmmMessage(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) {
	if len(plain) > 0 && plain[0]&0x0F == eps.PDESM {
		handleESM(m, ctx, ue, plain)
		return
	}

	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to read EMM message type", zap.Error(err))
		sendEMMStatus(ctx, ue, mme.EmmCauseProtocolErrorUnspec)

		return
	}

	ctx, span := mme.Tracer.Start(ctx, "nas/receive",
		trace.WithAttributes(attribute.String("nas.message_type", mme.EmmMessageTypeName(mt))))
	defer span.End()

	switch mt {
	case eps.MsgAttachRequest:
		handleAttachRequest(m, ctx, ue, plain, integrityVerified)
	case eps.MsgIdentityResponse:
		handleIdentityResponse(m, ctx, ue, plain)
	case eps.MsgAuthenticationResponse:
		handleAuthenticationResponse(m, ctx, ue, plain)
	case eps.MsgAuthenticationFailure:
		handleAuthenticationFailure(m, ctx, ue, plain)
	case eps.MsgSecurityModeComplete:
		handleSecurityModeComplete(m, ctx, ue, plain)
	case eps.MsgSecurityModeReject:
		handleSecurityModeReject(m, ctx, ue, plain)
	case eps.MsgAttachComplete:
		handleAttachComplete(m, ctx, ue, plain)
	case eps.MsgDetachRequest:
		handleDetachRequest(m, ctx, ue, plain, integrityVerified)
	case eps.MsgDetachAccept:
		handleDetachAccept(m, ctx, ue)
	case eps.MsgTrackingAreaUpdateRequest:
		handleTrackingAreaUpdate(m, ctx, ue, plain)
	case eps.MsgTrackingAreaUpdateComplete:
		handleTrackingAreaUpdateComplete(m, ctx, ue)
	case eps.MsgEMMStatus:
		handleEMMStatus(plain)
	default:
		// TS 24.301 §7.4: a message type not implemented by the receiver is ignored, but an
		// EMM STATUS with cause #97 "message type non-existent or not implemented" should be
		// returned (mirrors the AMF's HandleGmmMessage default).
		logger.From(ctx, logger.MmeLog).Warn("unhandled EMM message",
			zap.String("message-type", mme.EmmMessageTypeName(mt)),
			zap.Int("message-type-value", int(mt)))
		sendEMMStatus(ctx, ue, mme.EmmCauseMessageTypeNonExistent)
	}
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
