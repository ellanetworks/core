// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var nasTracer = otel.Tracer("ella-core/amf/nas")

// HandleNAS processes an uplink NAS PDU on a UE connection and finalizes the single outcome
// it resolves to: a REGISTRATION REQUEST mints a fresh persistent context; a message the AMF
// cannot process draws the STATUS the spec mandates (§7.4, §7.5.1) or an audited silence
// (§4.4.4.3), never a bare drop; a message that establishes no context leaves the connection
// bare for the NGAP layer to release. Every uplink NAS PDU enters here, whichever NGAP
// procedure carried it.
func HandleNAS(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeConn, nasPdu []byte) {
	if ue == nil {
		logger.From(ctx, logger.AmfLog).Error("inbound NAS on a nil UE connection")
		return
	}

	dispositionForNAS(ctx, amfInstance, ue, nasPdu).Finalize(ctx, egress{ue: ue})
}

// dispositionForNAS resolves an inbound NAS PDU to the single outcome the finalizer applies.
func dispositionForNAS(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeConn, nasPdu []byte) nasreply.Disposition {
	if nasPdu == nil {
		logger.From(ctx, logger.AmfLog).Error("inbound NAS with a nil PDU")
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	if ue.UeContext() == nil {
		amfUe, err := fetchUeContextWithMobileIdentity(ctx, amfInstance, nasPdu)
		if err != nil {
			// The first message could not be decoded to a resolvable identity. Classify the
			// raw PDU so the finalizer answers the STATUS the spec mandates (§7.4 / §7.5.1)
			// rather than dropping it. No secure exchange exists on a fresh connection, so
			// §4.4.4.3's silent discard does not apply here.
			logger.From(ctx, logger.AmfLog).Warn("failed to resolve UE context from mobile identity", zap.Error(err))

			// A SERVICE REQUEST recognizable by message type but undecodable is a protocol
			// error: §5.6.1.8 b) answers it with a SERVICE REJECT #96, not a STATUS.
			if isServiceRequest(nasPdu) {
				rejectBareServiceRequest(ctx, ue, fgs.GMMCauseInvalidMandatoryInformation)

				return nasreply.Handled()
			}

			return dispositionForUnresolved(nasPdu)
		}

		if amfUe == nil {
			// §4.4.4.3 admits a SERVICE REQUEST before secure exchange even when its MAC
			// fails, but it never mints a context. With none resolved — or none the message
			// authenticates against — the answer is a SERVICE REJECT #9, leaving the 5GMM
			// and 5G NAS security contexts unchanged.
			if isServiceRequest(nasPdu) {
				rejectBareServiceRequest(ctx, ue, fgs.GMMCauseUEIdentityCannotBeDerived)

				return nasreply.Handled()
			}

			// Mint a context only for an initial REGISTRATION REQUEST — the only message
			// that establishes a fresh context. This keeps minting reserved to registration
			// so an unauthenticated peer cannot leak a context per message. Any other message
			// resolved no context; it cannot be processed, but a message the network can still
			// classify draws a STATUS (§7.4 / §7.5.1) rather than a silent drop.
			if !isRegistrationRequest(nasPdu) {
				return dispositionForUnresolved(nasPdu)
			}

			amfUe = amf.NewUeContext()
		}

		amfInstance.AttachUeConn(amfUe, ue)
	}

	result, err := amf.DecodeNASMessage(ue.UeContext(), nasPdu)
	if err != nil {
		return amf.DispositionForDecodeError(err)
	}

	if !result.IsGMM {
		logger.From(ctx, logger.AmfLog).Warn("standalone 5GSM message on N1 discarded")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	integrityVerified := result.IntegrityVerified

	msgTypeName := amf.GmmMessageTypeName(result.MessageType)

	ctx, span := nasTracer.Start(ctx, "nas/receive",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("nas.message_type", msgTypeName),
			attribute.String("ue.supi", ue.UeContext().Supi().String()),
		),
	)
	defer span.End()

	ctx = logger.Into(ctx, ue.Log)

	logger.From(ctx, logger.AmfLog).Info(
		"Received NAS message",
		logger.MessageType(msgTypeName),
		logger.SUPI(ue.UeContext().Supi().String()),
	)

	return HandleGmmMessage(ctx, amfInstance, ue.UeContext(), result.MessageType, result.Plain, integrityVerified, result.ArrivedPlain)
}

// dispositionForUnresolved classifies a fresh-connection NAS PDU that established or resolved
// no 5GMM context, so the finalizer answers the STATUS the spec mandates instead of a bare
// drop: a decodable 5GMM message whose body is malformed → 5GMM STATUS #96 (§7.5.1); a
// non-5GMM, ciphered-without-context, or well-formed-but-unactionable message → 5GMM STATUS
// #97 (§7.4); a PDU too short to carry a message type → an audited silence (§7.2.1). A fresh
// connection has no secure exchange, so §4.4.4.3's silent discard does not apply — but the
// message is never *processed*, only answered, so an unauthenticated peer gains nothing.
func dispositionForUnresolved(nasPdu []byte) nasreply.Disposition {
	if len(nasPdu) < 3 {
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	if fgs.ProtocolDiscriminator(nasPdu[0]) != fgs.EPD5GMM {
		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}

	sht, err := fgs.PeekSecurityHeaderType(nasPdu)
	if err != nil {
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	body := nasPdu

	switch sht {
	case fgs.SHTPlain:
	case fgs.SHTIntegrityProtected:
		if len(nasPdu) < 8 {
			return nasreply.Silent(nasreply.ReasonTooShort)
		}

		body = nasPdu[7:]
	default:
		// Ciphered: with no context the AMF cannot decrypt or classify the body.
		return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
	}

	if _, _, err := amf.DecodePlainGmm(body); err != nil {
		return nasreply.StatusMM(amf.GmmDecodeFailureCause(body))
	}

	return nasreply.StatusMM(nasreply.CauseMessageTypeNotImplemented)
}

// isRegistrationRequest reports whether a fresh connection's first NAS message is a
// REGISTRATION REQUEST — the only message warranting a new UE context (TS 24.501). A
// ciphered or non-GMM message cannot be an initial registration the network can act
// on, so only a plain or integrity-protected (peekable) body matches.
func isRegistrationRequest(payload []byte) bool {
	mt, ok := peekInitialGmmType(payload)
	return ok && mt == uint8(fgs.MsgRegistrationRequest)
}

// isServiceRequest reports whether a fresh connection's first NAS message is a SERVICE
// REQUEST, so the mint gate can answer it with a SERVICE REJECT instead of minting a
// context or returning a 5GMM STATUS.
func isServiceRequest(payload []byte) bool {
	mt, ok := peekInitialGmmType(payload)
	return ok && mt == uint8(fgs.MsgServiceRequest)
}

// peekInitialGmmType returns the GMM message type of a fresh connection's first NAS PDU by
// reading the message-type octet directly (plain: octet 3; integrity-protected: octet 3 of
// the inner plain message). It deliberately does NOT fully decode the body, so a
// recognizable-but-malformed message is still classified by type — the network must answer
// such a SERVICE REQUEST with a SERVICE REJECT for the protocol error (TS 24.501 §5.6.1.8),
// not silently drop it. ok is false for a non-5GMM, ciphered, or too-short PDU. Mirrors the
// MME's raw message-type peek.
func peekInitialGmmType(payload []byte) (uint8, bool) {
	sht, err := fgs.PeekSecurityHeaderType(payload)
	if err != nil {
		return 0, false
	}

	body := payload

	switch sht {
	case fgs.SHTPlain:
	case fgs.SHTIntegrityProtected:
		// The inner plain message follows the security header (EPD, SHT, MAC[4], seq).
		spm, err := fgs.ParseSecurityProtectedMessage(payload)
		if err != nil {
			return 0, false
		}

		body = spm.UnverifiedPayload
	default:
		return 0, false
	}

	mt, err := fgs.PeekMessageType(body)
	if err != nil {
		return 0, false
	}

	return uint8(mt), true
}

// rejectBareServiceRequest answers a SERVICE REQUEST the AMF cannot accept with a SERVICE
// REJECT carrying cause, sent on the bare connection (no context is minted or mutated). The
// NGAP layer releases the connection afterwards (TS 24.501 §5.6.1.5, §5.6.1.8).
func rejectBareServiceRequest(ctx context.Context, ue *amf.UeConn, cause fgs.GMMCause) {
	pdu, err := amf.BuildServiceReject(cause)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to build service reject for uncontextualized service request", zap.Error(err))
		return
	}

	if err := ue.SendDownlinkNASTransport(ctx, pdu); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send service reject for uncontextualized service request", zap.Error(err))
	}
}

// fetchUeContextWithMobileIdentity resolves an existing UE context from the GUTI
// or 5G-S-TMSI carried by an inbound NAS message. It returns nil when the message
// must register on a fresh context.
func fetchUeContextWithMobileIdentity(ctx context.Context, amfInstance *amf.AMF, payload []byte) (*amf.UeContext, error) {
	sht, err := fgs.PeekSecurityHeaderType(payload)
	if err != nil {
		return nil, fmt.Errorf("nas payload is too short")
	}

	var body []byte

	switch sht {
	case fgs.SHTIntegrityProtected:
		if len(payload) < 7 {
			return nil, fmt.Errorf("integrity-protected nas payload is too short")
		}

		body = payload[7:]
	case fgs.SHTPlain:
		body = payload
	default:
		return nil, fmt.Errorf("unsupported security header type: 0x%0x", sht)
	}

	msgType, err := fgs.PeekMessageType(body)
	if err != nil {
		return nil, fmt.Errorf("error decoding plain nas: %w", err)
	}

	guti := etsi.InvalidGUTI5G

	additional := etsi.InvalidGUTI5G

	nativeIsAdditional := false

	switch msgType {
	case fgs.MsgRegistrationRequest:
		req, err := fgs.ParseRegistrationRequest(body)
		if !decoded(ctx, "RegistrationRequest", err) {
			return nil, fmt.Errorf("error decoding plain nas: %w", err)
		}

		if req.AdditionalGUTI != nil {
			additional, _ = etsi.NewGUTI5GFromNAS(*req.AdditionalGUTI)
		}

		nativeIsAdditional = movingFromEPC(req)

		switch {
		case req.MobileIdentity.GUTI != nil:
			guti, _ = etsi.NewGUTI5GFromNAS(req.MobileIdentity)
			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti received in Registration Request Message", logger.GUTI(guti.String()))
		case req.MobileIdentity.SUCI != nil:
			logger.WithTrace(ctx, logger.AmfLog).Debug("Suci received in Registration Request Message; using a fresh context",
				zap.Stringer("suci", req.MobileIdentity.SUCI))

			return nil, nil
		}
	case fgs.MsgServiceRequest:
		req, err := fgs.ParseServiceRequest(body)
		if !decoded(ctx, "ServiceRequest", err) {
			return nil, fmt.Errorf("error decoding plain nas: %w", err)
		}

		if req.MobileIdentity.STMSI != nil {
			guti, err = amfInstance.StmsiToGuti(ctx, *req.MobileIdentity.STMSI)
			if err != nil {
				return nil, fmt.Errorf("error converting 5G-S-TMSI to GUTI: %w", err)
			}

			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti derived from Service Request Message", logger.GUTI(guti.String()))
		}
	case fgs.MsgDeregistrationRequestUEOrig:
		req, err := fgs.ParseDeregistrationRequestUEOriginating(body)
		if err != nil {
			return nil, fmt.Errorf("error decoding plain nas: %w", err)
		}

		if req.MobileIdentity.GUTI != nil {
			guti, err = etsi.NewGUTI5GFromNAS(req.MobileIdentity)
			if err != nil {
				return nil, nil
			}

			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti received in Deregistraion Request Message", logger.GUTI(guti.String()))
		}
	}

	if guti == etsi.InvalidGUTI5G && additional == etsi.InvalidGUTI5G {
		return nil, nil
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("could not get operator info; resolving no context by GUTI", zap.Error(err))
		return nil, nil
	}

	candidates := [2]etsi.GUTI5G{guti, additional}
	if nativeIsAdditional {
		candidates = [2]etsi.GUTI5G{additional, guti}
	}

	for _, candidate := range candidates {
		ue, _ := amfInstance.LookupUeByGuti(operatorInfo.Guami, candidate)
		if ue == nil {
			continue
		}

		if !ue.ReuseForInboundNAS(payload) {
			logger.WithTrace(ctx, logger.AmfLog).Info("NAS message cites a known GUTI but is not authenticated for that context; using a fresh context", logger.GUTI(candidate.String()))
			continue
		}

		logger.From(ctx, logger.AmfLog).Info("UE Context derived from Guti", logger.GUTI(candidate.String()))

		return ue, nil
	}

	logger.WithTrace(ctx, logger.AmfLog).Warn("UE Context not found", logger.GUTI(candidates[0].String()))

	return nil, nil
}
