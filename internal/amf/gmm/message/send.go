// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/nas/send")

func SendDLNASTransport(ctx ctxt.Context, ue *context.RanUe, payloadContainerType uint8, nasPdu []byte, pduSessionID int32, cause uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Downlink NAS Transport",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("pduSessionID", int(pduSessionID)),
			attribute.Int("cause", int(cause)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	var causePtr *uint8
	if cause != 0 {
		causePtr = &cause
	}

	nasMsg, err := BuildDLNASTransport(ue.AmfUe, payloadContainerType, nasPdu, uint8(pduSessionID), causePtr)
	if err != nil {
		return fmt.Errorf("error building downlink NAS transport message: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendNotification(ctx ctxt.Context, ue *context.RanUe, nasMsg []byte) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Notification",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	amfUe := ue.AmfUe

	if context.AMFSelf().T3565Cfg.Enable {
		cfg := context.AMFSelf().T3565Cfg
		amfUe.T3565 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3565 expires, retransmit Notification", zap.Any("expireTimes", expireTimes))
			err := ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Error("could not send notification", zap.Error(err))
				return
			}
			amfUe.GmmLog.Info("sent notification")
		}, func() {
			amfUe.GmmLog.Warn("abort notification procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfUe.T3565 = nil // clear the timer
		})
	}

	return nil
}

func SendIdentityRequest(ctx ctxt.Context, ue *context.RanUe, typeOfIdentity uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Identity Request",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("typeOfIdentity", int(typeOfIdentity)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildIdentityRequest(typeOfIdentity)
	if err != nil {
		return fmt.Errorf("error building identity request: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationRequest(ctx ctxt.Context, ue *context.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Authentication Request",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	amfUe := ue.AmfUe

	if amfUe.AuthenticationCtx == nil {
		return fmt.Errorf("authentication context of UE is nil")
	}

	nasMsg, err := BuildAuthenticationRequest(amfUe)
	if err != nil {
		return fmt.Errorf("error building authentication request: %s", err.Error())
	}

	if context.AMFSelf().T3560Cfg.Enable {
		cfg := context.AMFSelf().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3560 expires, retransmit Authentication Request", zap.Any("expireTimes", expireTimes))
			err := ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
		}, func() {
			amfUe.GmmLog.Warn("T3560 Expires, abort authentication procedure & ongoing 5GMM procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfUe.Remove()
		})
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendServiceAccept(ctx ctxt.Context, ue *context.RanUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Authentication Result",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("pduSessionIDErrorCount", len(errPduSessionID)),
			attribute.Int("causeErrorCount", len(errCause)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildServiceAccept(ue.AmfUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building service accept: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationReject(ctx ctxt.Context, ue *context.RanUe, eapMsg string) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Authentication Reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildAuthenticationReject(ue.AmfUe, eapMsg)
	if err != nil {
		return fmt.Errorf("error building authentication reject: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationResult(ctx ctxt.Context, ue *context.RanUe, eapSuccess bool, eapMsg string) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Authentication Result",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildAuthenticationResult(ue.AmfUe, eapSuccess, eapMsg)
	if err != nil {
		return fmt.Errorf("error building authentication result: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendServiceReject(ctx ctxt.Context, ue *context.RanUe, pDUSessionStatus *[16]bool, cause uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Registration Reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("cause", int(cause)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildServiceReject(pDUSessionStatus, cause)
	if err != nil {
		return fmt.Errorf("error building service reject: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
// eapMessage: if the REGISTRATION REJECT message is used to convey EAP-failure message
func SendRegistrationReject(ctx ctxt.Context, ue *context.RanUe, cause5GMM uint8, eapMessage string) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Registration Reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("cause", int(cause5GMM)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationReject(ue.AmfUe, cause5GMM, eapMessage)
	if err != nil {
		return fmt.Errorf("error building registration reject: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

// eapSuccess: only used when authType is EAP-AKA', set the value to false if authType is not EAP-AKA'
// eapMessage: only used when authType is EAP-AKA', set the value to "" if authType is not EAP-AKA'
func SendSecurityModeCommand(ctx ctxt.Context, ue *context.RanUe, eapSuccess bool, eapMessage string) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Security Mode Command",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildSecurityModeCommand(ue.AmfUe, eapSuccess, eapMessage)
	if err != nil {
		return fmt.Errorf("error building security mode command: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	amfUe := ue.AmfUe

	if context.AMFSelf().T3560Cfg.Enable {
		cfg := context.AMFSelf().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3560 expires, retransmit Security Mode Command", zap.Any("expireTimes", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
			amfUe.GmmLog.Info("sent security mode command")
		}, func() {
			amfUe.GmmLog.Warn("T3560 Expires, abort security mode control procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfUe.Remove()
		})
	}

	return nil
}

func SendDeregistrationRequest(ctx ctxt.Context, ue *context.RanUe, accessType uint8, reRegistrationRequired bool, cause5GMM uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Deregistration Request",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("accessType", int(accessType)),
			attribute.Int("cause", int(cause5GMM)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	ue.AmfUe.DeregistrationTargetAccessType = accessType

	nasMsg, err := BuildDeregistrationRequest(ue, accessType, reRegistrationRequired, cause5GMM)
	if err != nil {
		return fmt.Errorf("error building deregistration request: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}
	ue.AmfUe.GmmLog.Info("sent deregistration request")

	amfUe := ue.AmfUe

	if context.AMFSelf().T3522Cfg.Enable {
		cfg := context.AMFSelf().T3522Cfg
		amfUe.T3522 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3522 expires, retransmit Deregistration Request", zap.Any("expireTimes", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
			amfUe.GmmLog.Info("sent deregistration request")
		}, func() {
			amfUe.GmmLog.Warn("T3522 Expires, abort deregistration procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfUe.T3522 = nil // clear the timer
			if accessType == nasMessage.AccessType3GPP {
				amfUe.GmmLog.Warn("UE accessType[3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessType3GPPAccess].Set(context.Deregistered)
				amfUe.Remove()
			} else if accessType == nasMessage.AccessTypeNon3GPP {
				amfUe.GmmLog.Warn("UE accessType[Non3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessTypeNon3GPPAccess].Set(context.Deregistered)
				amfUe.Remove()
			} else {
				amfUe.GmmLog.Warn("UE accessType[3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessType3GPPAccess].Set(context.Deregistered)
				amfUe.GmmLog.Warn("UE accessType[Non3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessTypeNon3GPPAccess].Set(context.Deregistered)
				amfUe.Remove()
			}
		})
	}

	return nil
}

func SendDeregistrationAccept(ctx ctxt.Context, ue *context.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	_, span := tracer.Start(ctx, "Send Deregistration Accept",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildDeregistrationAccept()
	if err != nil {
		return fmt.Errorf("error building deregistration accept: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		ue.AmfUe.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendRegistrationAccept(
	ctx ctxt.Context,
	ue *context.AmfUe,
	anType models.AccessType,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	pduSessionResourceSetupList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedPLMN *context.PlmnSupportItem,
	supportedGUAMI *models.Guami,
) error {
	if ue == nil {
		return fmt.Errorf("ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Registration Accept",
		trace.WithAttributes(
			attribute.String("supi", ue.Supi),
			attribute.String("accessType", string(anType)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationAccept(ctx, ue, anType, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, supportedPLMN)
	if err != nil {
		return fmt.Errorf("error building registration accept: %s", err.Error())
	}

	if ue.RanUe[anType].UeContextRequest {
		err = ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil, supportedGUAMI)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %s", err.Error())
		}
		ue.GmmLog.Info("Sent NGAP initial context setup request")
	} else {
		err = ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe[models.AccessType3GPPAccess], nasMsg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
		}
		ue.GmmLog.Info("Sent GMM registration accept")
	}

	if context.AMFSelf().T3550Cfg.Enable {
		cfg := context.AMFSelf().T3550Cfg
		ue.T3550 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			if ue.RanUe[anType] == nil {
				ue.GmmLog.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
				ue.T3550 = nil
			} else {
				if ue.RanUe[anType].UeContextRequest && !ue.RanUe[anType].RecvdInitialContextSetupResponse {
					err = ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil, supportedGUAMI)
					if err != nil {
						ue.GmmLog.Error("could not send initial context setup request", zap.Error(err))
					}
					ue.GmmLog.Info("Sent NGAP initial context setup request")
				} else {
					ue.GmmLog.Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))
					err = ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe[anType], nasMsg, nil)
					if err != nil {
						ue.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
					}
					ue.GmmLog.Info("Sent GMM registration accept")
				}
			}
		}, func() {
			ue.GmmLog.Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))
			ue.T3550 = nil // clear the timer
			// TS 24.501 5.5.1.2.8 case c, 5.5.1.3.8 case c
			ue.State[anType].Set(context.Registered)
			ue.ClearRegistrationRequestData(anType)
		})
	}

	return nil
}

func SendConfigurationUpdateCommand(ctx ctxt.Context, amfUe *context.AmfUe, accessType models.AccessType) {
	if amfUe == nil {
		return
	}

	_, span := tracer.Start(ctx, "Send Configuration Update Command",
		trace.WithAttributes(
			attribute.String("supi", amfUe.Supi),
			attribute.String("accessType", string(accessType)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	flags := amfUe.ConfigurationUpdateCommandFlags

	if amfUe.RanUe[accessType] == nil {
		amfUe.GmmLog.Error("cannot SendConfigurationUpdateCommand: RanUe is nil")
		return
	}

	nasMsg, err, startT3555 := BuildConfigurationUpdateCommand(amfUe, accessType, flags)
	if err != nil {
		amfUe.GmmLog.Error("error building ConfigurationUpdateCommand", zap.Error(err))
		return
	}
	amfUe.GmmLog.Info("Send Configuration Update Command")

	mobilityRestrictionList, err := ngap_message.BuildIEMobilityRestrictionList(amfUe)
	if err != nil {
		amfUe.GmmLog.Error("could not build Mobility Restriction List IE", zap.Error(err))
		return
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, amfUe.RanUe[accessType], nasMsg, mobilityRestrictionList)
	if err != nil {
		amfUe.GmmLog.Error("could not send configuration update command", zap.Error(err))
		return
	}

	if startT3555 && context.AMFSelf().T3555Cfg.Enable {
		cfg := context.AMFSelf().T3555Cfg
		amfUe.GmmLog.Info("start T3555 timer")
		amfUe.T3555 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("timer T3555 expired, retransmit Configuration Update Command",
				zap.Int32("retry", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ctx, amfUe.RanUe[accessType], nasMsg, mobilityRestrictionList)
			if err != nil {
				amfUe.GmmLog.Error("could not send configuration update command", zap.Error(err))
			}
		}, func() {
			amfUe.GmmLog.Warn("timer T3555 expired too many times, aborting configuration update procedure",
				zap.Int32("maximum retries", cfg.MaxRetryTimes))
		},
		)
	}
}
