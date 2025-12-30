// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/nas/send")

func SendDLNASTransport(ctx context.Context, ue *amfContext.RanUe, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Downlink NAS Transport",
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

	nasMsg, err := BuildDLNASTransport(ue.AmfUe, payloadContainerType, nasPdu, pduSessionID, causePtr)
	if err != nil {
		return fmt.Errorf("error building downlink NAS transport message: %s", err.Error())
	}

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendIdentityRequest(ctx context.Context, ue *amfContext.RanUe, typeOfIdentity uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Identity Request",
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

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationRequest(ctx context.Context, amf *amfContext.AMF, ue *amfContext.RanUe) error {
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

	if amf.T3560Cfg.Enable {
		cfg := amf.T3560Cfg
		amfUe.T3560 = amfContext.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Authentication Request", zap.Any("expireTimes", expireTimes))
			err := ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
			if err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort authentication procedure & ongoing 5GMM procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amf.RemoveAMFUE(amfUe)
		})
	}

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendServiceAccept(ctx context.Context, ue *amfContext.RanUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Authentication Result",
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

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationReject(ctx context.Context, ue *amfContext.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Authentication Reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildAuthenticationReject()
	if err != nil {
		return fmt.Errorf("error building authentication reject: %s", err.Error())
	}

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendServiceReject(ctx context.Context, ue *amfContext.RanUe, pDUSessionStatus *[16]bool, cause uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Registration Reject",
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

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
func SendRegistrationReject(ctx context.Context, ue *amfContext.RanUe, cause5GMM uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Registration Reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
			attribute.Int("cause", int(cause5GMM)),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationReject(ue.AmfUe.T3502Value, cause5GMM)
	if err != nil {
		return fmt.Errorf("error building registration reject: %s", err.Error())
	}

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendSecurityModeCommand(ctx context.Context, amf *amfContext.AMF, ue *amfContext.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Security Mode Command",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildSecurityModeCommand(ue.AmfUe)
	if err != nil {
		return fmt.Errorf("error building security mode command: %s", err.Error())
	}

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	amfUe := ue.AmfUe

	if amf.T3560Cfg.Enable {
		cfg := amf.T3560Cfg
		amfUe.T3560 = amfContext.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Security Mode Command", zap.Any("expireTimes", expireTimes))
			err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
			if err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
			amfUe.Log.Info("sent security mode command")
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort security mode control procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			// amfUe.Remove()
			amf.RemoveAMFUE(amfUe)
		})
	}

	return nil
}

func SendDeregistrationAccept(ctx context.Context, ue *amfContext.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Deregistration Accept",
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

	err = ue.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.AmfUeNgapID, ue.RanUeNgapID, nasMsg, nil)
	if err != nil {
		ue.AmfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendRegistrationAccept(
	ctx context.Context,
	amf *amfContext.AMF,
	ue *amfContext.AmfUe,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	pduSessionResourceSetupList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedPLMN *models.PlmnSupportItem,
	supportedGUAMI *models.Guami,
) error {
	if ue == nil {
		return fmt.Errorf("ue is nil")
	}

	ctx, span := tracer.Start(ctx, "Send Registration Accept",
		trace.WithAttributes(
			attribute.String("supi", ue.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationAccept(amf, ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, supportedPLMN)
	if err != nil {
		return fmt.Errorf("error building registration accept: %s", err.Error())
	}

	if ue.RanUe.UeContextRequest {
		ue.RanUe.SentInitialContextSetupRequest = true
		err = ue.RanUe.Radio.NGAPSender.SendInitialContextSetupRequest(
			ctx,
			ue.RanUe.AmfUeNgapID,
			ue.RanUe.RanUeNgapID,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb,
			ue.PlmnID,
			ue.UeRadioCapability,
			ue.UeRadioCapabilityForPaging,
			ue.UESecurityCapability,
			nasMsg,
			pduSessionResourceSetupList,
			supportedGUAMI,
		)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %s", err.Error())
		}
		ue.Log.Info("Sent NGAP initial context setup request")
	} else {
		err = ue.RanUe.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, nasMsg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
		}
		ue.Log.Info("Sent GMM registration accept")
	}

	if amf.T3550Cfg.Enable {
		cfg := amf.T3550Cfg
		ue.T3550 = amfContext.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			if ue.RanUe == nil {
				ue.Log.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
				ue.T3550 = nil
			} else {
				if ue.RanUe.UeContextRequest && !ue.RanUe.RecvdInitialContextSetupResponse {
					err = ue.RanUe.Radio.NGAPSender.SendInitialContextSetupRequest(
						ctx,
						ue.RanUe.AmfUeNgapID,
						ue.RanUe.RanUeNgapID,
						ue.Ambr.Uplink,
						ue.Ambr.Downlink,
						ue.AllowedNssai,
						ue.Kgnb,
						ue.PlmnID,
						ue.UeRadioCapability,
						ue.UeRadioCapabilityForPaging,
						ue.UESecurityCapability,
						nasMsg,
						pduSessionResourceSetupList,
						supportedGUAMI,
					)
					if err != nil {
						ue.Log.Error("could not send initial context setup request", zap.Error(err))
					}
					ue.RanUe.SentInitialContextSetupRequest = true
					ue.Log.Info("Sent NGAP initial context setup request")
				} else {
					ue.Log.Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))
					err = ue.RanUe.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, nasMsg, nil)
					if err != nil {
						ue.Log.Error("could not send downlink NAS transport message", zap.Error(err))
					}
					ue.Log.Info("Sent GMM registration accept")
				}
			}
		}, func() {
			ue.Log.Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))
			ue.T3550 = nil // clear the timer
			// TS 24.501 5.5.1.2.8 case c, 5.5.1.3.8 case c
			ue.State = amfContext.Registered
			ue.ClearRegistrationRequestData()
		})
	}

	return nil
}

func SendConfigurationUpdateCommand(ctx context.Context, amf *amfContext.AMF, amfUe *amfContext.AmfUe, flags *amfContext.ConfigurationUpdateCommandFlags) {
	if amfUe == nil {
		return
	}

	ctx, span := tracer.Start(ctx, "Send Configuration Update Command",
		trace.WithAttributes(
			attribute.String("supi", amfUe.Supi),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	if amfUe.RanUe == nil {
		amfUe.Log.Error("cannot SendConfigurationUpdateCommand: RanUe is nil")
		return
	}

	nasMsg, err, startT3555 := BuildConfigurationUpdateCommand(amf, amfUe, flags)
	if err != nil {
		amfUe.Log.Error("error building ConfigurationUpdateCommand", zap.Error(err))
		return
	}
	amfUe.Log.Info("Send Configuration Update Command")

	mobilityRestrictionList, err := send.BuildIEMobilityRestrictionList(amfUe.PlmnID)
	if err != nil {
		amfUe.Log.Error("could not build Mobility Restriction List IE", zap.Error(err))
		return
	}

	err = amfUe.RanUe.Radio.NGAPSender.SendDownlinkNasTransport(ctx, amfUe.RanUe.AmfUeNgapID, amfUe.RanUe.RanUeNgapID, nasMsg, mobilityRestrictionList)
	if err != nil {
		amfUe.Log.Error("could not send configuration update command", zap.Error(err))
		return
	}

	if startT3555 && amf.T3555Cfg.Enable {
		cfg := amf.T3555Cfg
		amfUe.Log.Info("start T3555 timer")
		amfUe.T3555 = amfContext.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("timer T3555 expired, retransmit Configuration Update Command",
				zap.Int32("retry", expireTimes))
			err = amfUe.RanUe.Radio.NGAPSender.SendDownlinkNasTransport(ctx, amfUe.RanUe.AmfUeNgapID, amfUe.RanUe.RanUeNgapID, nasMsg, mobilityRestrictionList)
			if err != nil {
				amfUe.Log.Error("could not send configuration update command", zap.Error(err))
			}
		}, func() {
			amfUe.Log.Warn("timer T3555 expired too many times, aborting configuration update procedure",
				zap.Int32("maximum retries", cfg.MaxRetryTimes))
		},
		)
	}
}
