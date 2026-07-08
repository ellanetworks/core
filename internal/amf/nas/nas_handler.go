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
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var nasTracer = otel.Tracer("ella-core/amf/nas")

// HandleNAS processes an uplink NAS PDU on a UE connection. A bare connection's first
// message binds a fresh persistent context here (mirrors the MME's HandleNAS); a
// message that establishes none leaves the connection bare for the NGAP layer to
// release. A REGISTRATION REQUEST mints a fresh context; any other unresolved message is
// dropped without a STATUS (a SERVICE REQUEST is routed to HandleServiceRequest at the
// NGAP layer, before HandleNAS). An unimplemented message type draws a 5GMM STATUS from
// HandleGmmMessage (§7.4).
func HandleNAS(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeConn, nasPdu []byte) {
	if ue == nil {
		logger.From(ctx, logger.AmfLog).Error("inbound NAS on a nil UE connection")
		return
	}

	if nasPdu == nil {
		logger.From(ctx, logger.AmfLog).Error("inbound NAS with a nil PDU")
		return
	}

	if ue.UeContext() == nil {
		amfUe, err := fetchUeContextWithMobileIdentity(ctx, amfInstance, nasPdu)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to resolve UE context from mobile identity", zap.Error(err))
			return
		}

		if amfUe == nil {
			// Mint a context only for an initial REGISTRATION REQUEST — the only message
			// that establishes a fresh context. A SERVICE REQUEST is resolved-or-rejected
			// before HandleNAS by HandleServiceRequest (routed at the NGAP layer, mirroring
			// the MME); any other message cites a context that was not found and cannot
			// proceed. This keeps minting reserved to registration so an unauthenticated
			// peer cannot leak a context per message (mirrors the MME's ATTACH-only gate).
			// A connection left bare here is released by the NGAP layer.
			if !isRegistrationRequest(nasPdu) {
				logger.From(ctx, logger.AmfLog).Debug("initial NAS message is not a registration request; leaving the connection bare")
				return
			}

			amfUe = amf.NewUeContext()
		}

		amfInstance.AttachUeConn(amfUe, ue)
	}

	result, err := amf.DecodeNASMessage(ue.UeContext(), nasPdu)
	if err != nil {
		// DecodeNASMessage logged the reason; the PDU is dropped. On a secured
		// connection a message that fails integrity or arrives plain is discarded
		// without answer (TS 24.501 §4.4.4.3).
		return
	}

	msg := result.Message

	if msg.GmmMessage == nil {
		logger.From(ctx, logger.AmfLog).Warn("decoded NAS message carries no GMM body")
		return
	}

	if msg.GsmMessage != nil {
		logger.From(ctx, logger.AmfLog).Warn("standalone 5GSM message on N1 discarded")
		return
	}

	integrityVerified := result.IntegrityVerified

	msgTypeName := amf.GmmMessageTypeName(msg.GmmHeader.GetMessageType())

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

	HandleGmmMessage(ctx, amfInstance, ue.UeContext(), msg.GmmMessage, integrityVerified)
}

// isRegistrationRequest reports whether a fresh connection's first NAS message is a
// REGISTRATION REQUEST — the only message warranting a new UE context (TS 24.501). A
// ciphered or non-GMM message cannot be an initial registration the network can act
// on, so only a plain or integrity-protected (peekable) body matches (mirrors the
// MME's isInitialAttach).
func isRegistrationRequest(payload []byte) bool {
	mt, ok := peekInitialGmmType(payload)
	return ok && mt == nas.MsgTypeRegistrationRequest
}

// IsServiceRequest reports whether a fresh connection's first NAS message is a SERVICE
// REQUEST, so the NGAP layer can route it to HandleServiceRequest before the mint gate
// (mirrors the MME's S1AP peek).
func IsServiceRequest(payload []byte) bool {
	mt, ok := peekInitialGmmType(payload)
	return ok && mt == nas.MsgTypeServiceRequest
}

// HandleServiceRequest answers an initial SERVICE REQUEST, routed here from the NGAP layer
// before the HandleNAS mint gate (mirrors the MME's dedicated S1AP handler). It resolves
// the UE by the request's 5G-S-TMSI — integrity-verified against the held context — and
// either dispatches the accept/reactivation or answers SERVICE REJECT #9. It never mints
// a context and leaves the 5GMM/security context unchanged on rejection
// (TS 24.501 §5.6.1.5, §4.4.4.3).
func HandleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeConn, nasPdu []byte) {
	amfUe, err := fetchUeContextWithMobileIdentity(ctx, amfInstance, nasPdu)
	if err != nil || amfUe == nil {
		// No context for the cited 5G-S-TMSI, or the request failed the integrity check:
		// it cannot be accepted. Answer SERVICE REJECT #9 without binding or mutating any
		// context; the NGAP layer releases the bare connection.
		rejectBareServiceRequest(ctx, ue)
		return
	}

	amfInstance.AttachUeConn(amfUe, ue)
	amfInstance.StopIdleTimers(amfUe)

	result, err := amf.DecodeNASMessage(amfUe, nasPdu)
	if err != nil {
		return
	}

	if result.Message.GmmMessage == nil || result.Message.ServiceRequest == nil {
		logger.From(ctx, logger.AmfLog).Warn("service request routed but decoded body is not a service request")
		return
	}

	handleServiceRequest(ctx, amfInstance, amfUe, result.Message.ServiceRequest, result.IntegrityVerified)
}

// peekInitialGmmType returns the GMM message type of a fresh connection's first NAS PDU,
// peeking the plain or integrity-protected body. ok is false for a ciphered or
// undecodable message the network cannot classify without a security context.
func peekInitialGmmType(payload []byte) (uint8, bool) {
	if len(payload) < 2 {
		return 0, false
	}

	body := payload

	switch nas.GetSecurityHeaderType(payload) & 0x0f {
	case nas.SecurityHeaderTypePlainNas:
	case nas.SecurityHeaderTypeIntegrityProtected:
		if len(payload) < 7 {
			return 0, false
		}

		body = payload[7:]
	default:
		return 0, false
	}

	msg := new(nas.Message)
	if err := msg.PlainNasDecode(&body); err != nil {
		return 0, false
	}

	return msg.GmmHeader.GetMessageType(), true
}

// rejectBareServiceRequest answers a SERVICE REQUEST that resolved no 5GMM context with
// SERVICE REJECT #9, sent on the bare connection (no context is minted). The NGAP layer
// releases the connection afterwards.
func rejectBareServiceRequest(ctx context.Context, ue *amf.UeConn) {
	pdu, err := amf.BuildServiceReject(nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to build service reject for uncontextualized service request", zap.Error(err))
		return
	}

	if err := ue.SendDownlinkNASTransport(ctx, pdu, nil); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send service reject for uncontextualized service request", zap.Error(err))
	}
}

// fetchUeContextWithMobileIdentity resolves an existing UE context from the GUTI
// or 5G-S-TMSI carried by an inbound NAS message. It returns nil when the message
// must register on a fresh context.
func fetchUeContextWithMobileIdentity(ctx context.Context, amfInstance *amf.AMF, payload []byte) (*amf.UeContext, error) {
	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	if len(payload) < 2 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	msg := new(nas.Message)

	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f
	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
		if len(payload) < 7 {
			return nil, fmt.Errorf("integrity-protected nas payload is too short")
		}

		p := payload[7:]

		if err := msg.PlainNasDecode(&p); err != nil {
			return nil, fmt.Errorf("error decoding plain nas: %+v", err)
		}
	case nas.SecurityHeaderTypePlainNas:
		// Decode a copy so the original payload stays intact for the later integrity check.
		p := payload

		if err := msg.PlainNasDecode(&p); err != nil {
			return nil, fmt.Errorf("error decoding plain nas: %+v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported security header type: 0x%0x", msg.SecurityHeaderType)
	}

	guti := etsi.InvalidGUTI5G

	switch msg.GmmHeader.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		mobileIdentity5GSContents := msg.RegistrationRequest.GetMobileIdentity5GSContents()
		if len(mobileIdentity5GSContents) == 0 {
			return nil, fmt.Errorf("mobile identity 5GS is empty")
		}

		if nasMessage.MobileIdentity5GSType5gGuti == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			guti, _ = etsi.NewGUTI5GFromBytes(mobileIdentity5GSContents)
			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti received in Registration Request Message", logger.GUTI(guti.String()))
		} else if nasMessage.MobileIdentity5GSTypeSuci == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			// A SUCI is a one-time concealed identity, not a handle to an existing
			// context. Always register on a fresh context; any prior context for
			// the same subscriber is superseded only once this registration is
			// authenticated (TS 24.501, reconciled by SUPI on accept).
			suci, _ := nasConvert.SuciToString(mobileIdentity5GSContents)
			logger.WithTrace(ctx, logger.AmfLog).Debug("Suci received in Registration Request Message; using a fresh context", zap.String("suci", suci))

			return nil, nil
		}
	case nas.MsgTypeServiceRequest:
		mobileIdentity5GSContents := msg.TMSI5GS.Octet
		if len(mobileIdentity5GSContents) == 0 {
			return nil, fmt.Errorf("mobile identity 5GS is empty")
		}

		if nasMessage.MobileIdentity5GSType5gSTmsi == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			var err error

			guti, err = amfInstance.StmsiToGuti(ctx, mobileIdentity5GSContents)
			if err != nil {
				return nil, fmt.Errorf("error converting 5G-S-TMSI to GUTI: %+v", err)
			}

			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti derived from Service Request Message", logger.GUTI(guti.String()))
		}
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		mobileIdentity5GSContents := msg.DeregistrationRequestUEOriginatingDeregistration.GetMobileIdentity5GSContents()
		if len(mobileIdentity5GSContents) == 0 {
			return nil, fmt.Errorf("mobile identity 5GS is empty")
		}

		if nasMessage.MobileIdentity5GSType5gGuti == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			var err error

			guti, err = etsi.NewGUTI5GFromBytes(mobileIdentity5GSContents)
			if err != nil {
				return nil, nil
			}

			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti received in Deregistraion Request Message", logger.GUTI(guti.String()))
		}
	}

	if guti == etsi.InvalidGUTI5G {
		return nil, nil
	}

	ue, _ := amfInstance.LookupUeByGuti(guti)
	if ue == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("UE Context not found", logger.GUTI(guti.String()))
		return nil, nil
	}

	if !ue.ReuseForInboundNAS(payload) {
		// TS 24.501: this message cites an existing context but is not
		// integrity-verified for it. Register on a fresh context; the committed
		// context (its NAS security context and PDU sessions) is left unchanged.
		logger.WithTrace(ctx, logger.AmfLog).Info("NAS message cites a known GUTI but is not authenticated for that context; using a fresh context", logger.GUTI(guti.String()))
		return nil, nil
	}

	logger.From(ctx, logger.AmfLog).Info("UE Context derived from Guti", logger.GUTI(guti.String()))

	return ue, nil
}
