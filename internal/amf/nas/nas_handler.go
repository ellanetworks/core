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
// message binds a fresh persistent context here; a message that establishes none leaves
// the connection bare for the NGAP layer to release. A REGISTRATION REQUEST mints a fresh
// context; any other unresolved message is dropped without a STATUS (a SERVICE REQUEST is
// routed to HandleServiceRequest at the NGAP layer, before HandleNAS). An unimplemented
// message type draws a 5GMM STATUS from HandleGmmMessage (§7.4).
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
			// before HandleNAS by HandleServiceRequest, routed at the NGAP layer; any other
			// message cites a context that was not found and cannot proceed. This keeps
			// minting reserved to registration so an unauthenticated peer cannot leak a
			// context per message. A connection left bare here is released by the NGAP layer.
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
// on, so only a plain or integrity-protected (peekable) body matches.
func isRegistrationRequest(payload []byte) bool {
	mt, ok := peekInitialGmmType(payload)
	return ok && mt == nas.MsgTypeRegistrationRequest
}

// IsServiceRequest reports whether a fresh connection's first NAS message is a SERVICE
// REQUEST, so the NGAP layer can route it to HandleServiceRequest before the mint gate.
func IsServiceRequest(payload []byte) bool {
	mt, ok := peekInitialGmmType(payload)
	return ok && mt == nas.MsgTypeServiceRequest
}

// HandleServiceRequest answers an initial SERVICE REQUEST, routed here from the NGAP layer
// before the HandleNAS mint gate. It resolves the UE by the request's 5G-S-TMSI —
// integrity-verified against the held context — and either dispatches the accept, or
// answers a SERVICE REJECT (#96 for a protocol error per §5.6.1.8, else #9 when no context
// can be derived per §5.6.1.5/§4.4.4.3). It never mints a context and leaves the
// 5GMM/security context unchanged on rejection.
func HandleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeConn, nasPdu []byte) {
	amfUe, err := fetchUeContextWithMobileIdentity(ctx, amfInstance, nasPdu)
	if err != nil {
		// The SERVICE REQUEST is recognizable but could not be decoded (a protocol error,
		// e.g. a missing mandatory IE). TS 24.501 §5.6.1.8 b): the AMF shall return a
		// SERVICE REJECT with cause #96 "invalid mandatory information", not a silent drop.
		logger.From(ctx, logger.AmfLog).Warn("malformed service request; rejecting", zap.Error(err))
		rejectBareServiceRequest(ctx, ue, nasMessage.Cause5GMMInvalidMandatoryInformation)

		return
	}

	if amfUe == nil {
		// The request decoded, but no 5GMM context exists for the cited 5G-S-TMSI (or it
		// failed the integrity check): it cannot be accepted. SERVICE REJECT #9 without
		// binding or mutating any context; the NGAP layer releases the bare connection.
		rejectBareServiceRequest(ctx, ue, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
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

// peekInitialGmmType returns the GMM message type of a fresh connection's first NAS PDU by
// reading the message-type octet directly (plain: octet 3; integrity-protected: octet 3 of
// the inner plain message). It deliberately does NOT fully decode the body, so a
// recognizable-but-malformed message is still classified by type — the network must answer
// such a SERVICE REQUEST with a SERVICE REJECT for the protocol error (TS 24.501 §5.6.1.8),
// not silently drop it. ok is false for a non-5GMM, ciphered, or too-short PDU. Mirrors the
// MME's raw message-type peek.
func peekInitialGmmType(payload []byte) (uint8, bool) {
	if len(payload) < 3 || payload[0] != nasMessage.Epd5GSMobilityManagementMessage {
		return 0, false
	}

	switch nas.GetSecurityHeaderType(payload) & 0x0f {
	case nas.SecurityHeaderTypePlainNas:
		return payload[2], true
	case nas.SecurityHeaderTypeIntegrityProtected:
		// Security header (EPD, SHT, MAC[4], sequence number) then the inner plain message
		// (EPD, SHT, message type), so the message type is octet 10 (index 9).
		if len(payload) < 10 {
			return 0, false
		}

		return payload[9], true
	default:
		return 0, false
	}
}

// rejectBareServiceRequest answers a SERVICE REQUEST the AMF cannot accept with a SERVICE
// REJECT carrying cause, sent on the bare connection (no context is minted or mutated). The
// NGAP layer releases the connection afterwards (TS 24.501 §5.6.1.5, §5.6.1.8).
func rejectBareServiceRequest(ctx context.Context, ue *amf.UeConn, cause uint8) {
	pdu, err := amf.BuildServiceReject(cause)
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
