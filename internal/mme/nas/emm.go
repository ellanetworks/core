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
// context. It unwraps NAS security when the message is protected, then routes
// the plain message to its procedure handler.
func HandleNAS(m *mme.MME, ctx context.Context, ue *mme.UeContext, nas []byte) {
	ue.TouchLastSeen()

	pd, err := eps.ProtocolDiscriminator(nas)
	if err != nil {
		logger.MmeLog.Warn("failed to read NAS protocol discriminator", zap.Error(err))
		return
	}

	if pd != eps.PDEMM {
		logger.MmeLog.Debug("ignoring standalone ESM NAS message")
		return
	}

	result, err := mme.DecodeNASMessage(ue, nas)
	if err != nil {
		// DecodeNASMessage has logged the reason; the PDU is dropped.
		return
	}

	// A returning UE's ATTACH REQUEST can fail the fresh context's MAC yet carry a
	// native GUTI whose held EPS security context verifies it; that context is then
	// reused and authentication skipped (TS 23.401). The check verifies the held
	// MAC against the raw PDU, so it is a no-op for any other message or a plain PDU.
	if !result.IntegrityVerified && reuseContextForGUTIAttach(m, ctx, ue, nas, result.Plain) {
		return
	}

	DispatchEMM(m, ctx, ue, result.Plain, result.IntegrityVerified)
}

// DispatchEMM routes a plain NAS message to its procedure handler, splitting ESM
// session-management messages from EMM mobility messages by protocol
// discriminator.
func DispatchEMM(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) {
	if len(plain) > 0 && plain[0]&0x0F == eps.PDESM {
		handleESM(m, ctx, ue, plain)
		return
	}

	mt, err := eps.PeekMessageType(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to read EMM message type", zap.Error(err))
		return
	}

	ctx, span := mme.Tracer.Start(ctx, "mme/emm",
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
		handleDetachRequest(m, ctx, ue, plain)
	case eps.MsgDetachAccept:
		handleDetachAccept(m, ctx, ue)
	case eps.MsgTrackingAreaUpdateRequest:
		handleTrackingAreaUpdate(m, ctx, ue, plain)
	case eps.MsgTrackingAreaUpdateComplete:
		handleTrackingAreaUpdateComplete(m, ctx, ue)
	default:
		logger.MmeLog.Warn("unhandled EMM message",
			zap.String("message-type", mme.EmmMessageTypeName(mt)),
			zap.Int("message-type-value", int(mt)))
	}
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
