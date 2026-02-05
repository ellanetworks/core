// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package nas

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/nas")

// HandleNAS processes an uplink NAS PDU and emits a span around the entire operation.
func HandleNAS(ctx context.Context, amf *amfContext.AMF, ue *amfContext.RanUe, nasPdu []byte) error {
	if ue == nil {
		return fmt.Errorf("ue is nil")
	}

	if nasPdu == nil {
		return fmt.Errorf("nas pdu is nil")
	}

	// First-time UE attach: fetch or create AMF context
	if ue.AmfUe == nil {
		amfUe, err := fetchUeContextWithMobileIdentity(ctx, amf, nasPdu)
		if err != nil {
			return fmt.Errorf("error fetching UE context with mobile identity: %v", err)
		}

		ue.AmfUe = amfUe
		if ue.AmfUe == nil {
			ue.AmfUe = amfContext.NewAmfUe()
		}

		ue.AmfUe.AttachRanUe(ue)
	}

	ue.AmfUe.Mutex.Lock()
	defer ue.AmfUe.Mutex.Unlock()

	msg, err := ue.AmfUe.DecodeNASMessage(nasPdu)
	if err != nil {
		return fmt.Errorf("error decoding NAS message: %v", err)
	}

	if msg.GmmMessage == nil {
		return errors.New("gmm message is nil")
	}

	if msg.GsmMessage != nil {
		return errors.New("gsm message is not nil")
	}

	msgTypeName := messageName(msg.GmmHeader.GetMessageType())

	ctx, span := tracer.Start(ctx, "NAS receive",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("nas.message_type", msgTypeName),
			attribute.String("ue.supi", ue.AmfUe.Supi),
		),
	)
	defer span.End()

	logger.AmfLog.Info(
		"Received NAS message",
		zap.String("MessageType", msgTypeName),
		zap.String("SUPI", ue.AmfUe.Supi),
	)

	err = gmm.HandleGmmMessage(ctx, amf, ue.AmfUe, msg.GmmMessage)
	if err != nil {
		return fmt.Errorf("error handling NAS message for supi %s: %v", ue.AmfUe.Supi, err)
	}

	return nil
}

/*
fetch Guti if present incase of integrity protected Nas Message
*/
func fetchUeContextWithMobileIdentity(ctx context.Context, amf *amfContext.AMF, payload []byte) (*amfContext.AmfUe, error) {
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
		p := payload[7:]
		if err := msg.PlainNasDecode(&p); err != nil {
			return nil, fmt.Errorf("error decoding plain nas: %+v", err)
		}
	case nas.SecurityHeaderTypePlainNas:
		if err := msg.PlainNasDecode(&payload); err != nil {
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
			logger.AmfLog.Debug("Guti received in Registration Request Message", zap.String("guti", guti.String()))
		} else if nasMessage.MobileIdentity5GSTypeSuci == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			suci, _ := nasConvert.SuciToString(mobileIdentity5GSContents)
			/* UeContext found based on SUCI which means context is exist in Network(AMF) but not
			   present in UE. Hence, AMF clear the existing context
			*/
			ue, _ := amf.FindAMFUEBySuci(suci)
			if ue != nil {
				ue.Log.Info("UE Context derived from Suci", zap.String("suci", suci))
				ue.SecurityContextAvailable = false
			}

			return ue, nil
		}
	case nas.MsgTypeServiceRequest:
		mobileIdentity5GSContents := msg.TMSI5GS.Octet
		if len(mobileIdentity5GSContents) == 0 {
			return nil, fmt.Errorf("mobile identity 5GS is empty")
		}

		if nasMessage.MobileIdentity5GSType5gSTmsi == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			guti, err := amf.StmsiToGuti(ctx, mobileIdentity5GSContents)
			if err != nil {
				return nil, fmt.Errorf("error converting 5G-S-TMSI to GUTI: %+v", err)
			}

			logger.AmfLog.Debug("Guti derived from Service Request Message", zap.String("guti", guti.String()))
		}
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		mobileIdentity5GSContents := msg.DeregistrationRequestUEOriginatingDeregistration.GetMobileIdentity5GSContents()
		if nasMessage.MobileIdentity5GSType5gGuti == nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0]) {
			guti, err := etsi.NewGUTIFromBytes(mobileIdentity5GSContents)
			if err != nil {
				return nil, nil
			}

			logger.AmfLog.Debug("Guti received in Deregistraion Request Message", zap.String("guti", guti.String()))
		}
	}

	if guti == etsi.InvalidGUTI {
		return nil, nil
	}

	ue, _ := amf.FindAmfUeByGuti(guti)
	if ue == nil {
		logger.AmfLog.Warn("UE Context not found", zap.String("guti", guti.String()))
		return nil, nil
	}

	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		ue.Log.Info("UE identified by GUTI but NAS is plain; treating as no security context", zap.String("guti", guti.String()))

		// UE likely lost keys; force fresh security setup
		ue.SecurityContextAvailable = false

		return ue, nil
	}

	ue.Log.Info("UE Context derived from Guti", zap.String("guti", guti.String()))

	return ue, nil
}
