// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
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

// HandleNAS processes an uplink NAS PDU.
func HandleNAS(ctx context.Context, amfInstance *amf.AMF, ue *amf.RanUe, nasPdu []byte) error {
	if ue == nil {
		return fmt.Errorf("ue is nil")
	}

	if nasPdu == nil {
		return fmt.Errorf("nas pdu is nil")
	}

	// First-time UE attach: fetch or create amf.AMF context
	if ue.UeContext() == nil {
		amfUe, err := fetchUeContextWithMobileIdentity(ctx, amfInstance, nasPdu)
		if err != nil {
			return fmt.Errorf("error fetching UE context with mobile identity: %v", err)
		}

		if amfUe == nil {
			amfUe = amf.NewUeContext()
		}

		amfUe.AttachRanUe(ue)
	}

	result, err := amf.DecodeNASMessage(ue.UeContext(), nasPdu)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}

	msg := result.Message

	if msg.GmmMessage == nil {
		return errors.New("gmm message is nil")
	}

	if msg.GsmMessage != nil {
		return errors.New("gsm message is not nil")
	}

	var integrityVerified bool

	switch result.Verdict {
	case amf.VerdictIntegrityVerified:
		integrityVerified = true
	case amf.VerdictPlainAllowed, amf.VerdictMacFailedAllowed:
		integrityVerified = false
	case amf.VerdictReject:
		return fmt.Errorf("nas pdu rejected by classifier")
	}

	msgTypeName := amf.MessageName(msg.GmmHeader.GetMessageType())

	ctx, span := nasTracer.Start(ctx, "nas/receive",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("nas.message_type", msgTypeName),
			attribute.String("ue.supi", ue.UeContext().SupiValue().String()),
		),
	)
	defer span.End()

	logger.WithTrace(ctx, logger.AmfLog).Info(
		"Received NAS message",
		logger.MessageType(msgTypeName),
		logger.SUPI(ue.UeContext().SupiValue().String()),
	)

	err = HandleGmmMessage(ctx, amfInstance, ue.UeContext(), msg.GmmMessage, integrityVerified)
	if err != nil {
		return fmt.Errorf("error handling NAS message for supi %s: %v", ue.UeContext().SupiValue().String(), err)
	}

	return nil
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
		// Decode a copy so the original payload remains intact for the
		// integrity check used in the reuse decision below.
		p := payload

		if err := msg.PlainNasDecode(&p); err != nil {
			return nil, fmt.Errorf("error decoding plain nas: %+v", err)
		}
	default:
		return nil, fmt.Errorf("unsupported security header type: 0x%0x", msg.SecurityHeaderType)
	}

	guti := etsi.InvalidGUTI

	switch msg.GmmHeader.GetMessageType() {
	case nas.MsgTypeRegistrationRequest:
		mobileIdentity5GSContents := msg.RegistrationRequest.GetMobileIdentity5GSContents()
		if len(mobileIdentity5GSContents) == 0 {
			return nil, fmt.Errorf("mobile identity 5GS is empty")
		}

		if nasMessage.MobileIdentity5GSType5gGuti == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			guti, _ = etsi.NewGUTIFromBytes(mobileIdentity5GSContents)
			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti received in Registration Request Message", logger.GUTI(guti.String()))
		} else if nasMessage.MobileIdentity5GSTypeSuci == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			// A SUCI is a one-time concealed identity, not a handle to an existing
			// context. Always register on a fresh context; any prior context for
			// the same subscriber is superseded only once this registration is
			// authenticated (TS 24.501 §5.5.1.2.8 f, reconciled by SUPI on accept).
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
			guti, err := amfInstance.StmsiToGuti(ctx, mobileIdentity5GSContents)
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
			guti, err := etsi.NewGUTIFromBytes(mobileIdentity5GSContents)
			if err != nil {
				return nil, nil
			}

			logger.WithTrace(ctx, logger.AmfLog).Debug("Guti received in Deregistraion Request Message", logger.GUTI(guti.String()))
		}
	}

	if guti == etsi.InvalidGUTI {
		return nil, nil
	}

	ue, _ := amfInstance.FindUeContextByGuti(guti)
	if ue == nil {
		logger.WithTrace(ctx, logger.AmfLog).Warn("UE Context not found", logger.GUTI(guti.String()))
		return nil, nil
	}

	if !ue.ReuseForInboundNAS(payload) {
		// TS 24.501 §4.4.4.3: this message cites an existing context but is not
		// integrity-verified for it. Register on a fresh context; the committed
		// context (its NAS security context and PDU sessions) is left unchanged.
		logger.WithTrace(ctx, logger.AmfLog).Info("NAS message cites a known GUTI but is not authenticated for that context; using a fresh context", logger.GUTI(guti.String()))
		return nil, nil
	}

	ue.Log.Info("UE Context derived from Guti", logger.GUTI(guti.String()))

	return ue, nil
}
