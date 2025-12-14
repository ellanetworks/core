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

func SendIdentityRequest(ctx ctxt.Context, ue *context.RanUe, typeOfIdentity uint8) error {
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
			amfUe.Log.Warn("T3560 expires, retransmit Authentication Request", zap.Any("expireTimes", expireTimes))
			err := ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
			if err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort authentication procedure & ongoing 5GMM procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
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

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationReject(ctx ctxt.Context, ue *context.RanUe) error {
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

	nasMsg, err := BuildAuthenticationReject(ue.AmfUe)
	if err != nil {
		return fmt.Errorf("error building authentication reject: %s", err.Error())
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

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
func SendRegistrationReject(ctx ctxt.Context, ue *context.RanUe, cause5GMM uint8) error {
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

	nasMsg, err := BuildRegistrationReject(ue.AmfUe, cause5GMM)
	if err != nil {
		return fmt.Errorf("error building registration reject: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendSecurityModeCommand(ctx ctxt.Context, ue *context.RanUe) error {
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

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	amfUe := ue.AmfUe

	if context.AMFSelf().T3560Cfg.Enable {
		cfg := context.AMFSelf().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Security Mode Command", zap.Any("expireTimes", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
			if err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
			amfUe.Log.Info("sent security mode command")
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort security mode control procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfUe.Remove()
		})
	}

	return nil
}

func SendDeregistrationAccept(ctx ctxt.Context, ue *context.RanUe) error {
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

	err = ngap_message.SendDownlinkNasTransport(ctx, ue, nasMsg, nil)
	if err != nil {
		ue.AmfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendRegistrationAccept(
	ctx ctxt.Context,
	ue *context.AmfUe,
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
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationAccept(ctx, ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, supportedPLMN)
	if err != nil {
		return fmt.Errorf("error building registration accept: %s", err.Error())
	}

	if ue.RanUe.UeContextRequest {
		err = ngap_message.SendInitialContextSetupRequest(ctx, ue, nasMsg, pduSessionResourceSetupList, nil, nil, nil, supportedGUAMI)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %s", err.Error())
		}
		ue.Log.Info("Sent NGAP initial context setup request")
	} else {
		err = ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe, nasMsg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
		}
		ue.Log.Info("Sent GMM registration accept")
	}

	if context.AMFSelf().T3550Cfg.Enable {
		cfg := context.AMFSelf().T3550Cfg
		ue.T3550 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			if ue.RanUe == nil {
				ue.Log.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
				ue.T3550 = nil
			} else {
				if ue.RanUe.UeContextRequest && !ue.RanUe.RecvdInitialContextSetupResponse {
					err = ngap_message.SendInitialContextSetupRequest(ctx, ue, nasMsg, pduSessionResourceSetupList, nil, nil, nil, supportedGUAMI)
					if err != nil {
						ue.Log.Error("could not send initial context setup request", zap.Error(err))
					}
					ue.Log.Info("Sent NGAP initial context setup request")
				} else {
					ue.Log.Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))
					err = ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe, nasMsg, nil)
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
			ue.State.Set(context.Registered)
			ue.ClearRegistrationRequestData()
		})
	}

	return nil
}

func SendConfigurationUpdateCommand(ctx ctxt.Context, amfUe *context.AmfUe) {
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

	flags := amfUe.ConfigurationUpdateCommandFlags

	if amfUe.RanUe == nil {
		amfUe.Log.Error("cannot SendConfigurationUpdateCommand: RanUe is nil")
		return
	}

	nasMsg, err, startT3555 := BuildConfigurationUpdateCommand(amfUe, flags)
	if err != nil {
		amfUe.Log.Error("error building ConfigurationUpdateCommand", zap.Error(err))
		return
	}
	amfUe.Log.Info("Send Configuration Update Command")

	mobilityRestrictionList, err := ngap_message.BuildIEMobilityRestrictionList(amfUe)
	if err != nil {
		amfUe.Log.Error("could not build Mobility Restriction List IE", zap.Error(err))
		return
	}

	err = ngap_message.SendDownlinkNasTransport(ctx, amfUe.RanUe, nasMsg, mobilityRestrictionList)
	if err != nil {
		amfUe.Log.Error("could not send configuration update command", zap.Error(err))
		return
	}

	if startT3555 && context.AMFSelf().T3555Cfg.Enable {
		cfg := context.AMFSelf().T3555Cfg
		amfUe.Log.Info("start T3555 timer")
		amfUe.T3555 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("timer T3555 expired, retransmit Configuration Update Command",
				zap.Int32("retry", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ctx, amfUe.RanUe, nasMsg, mobilityRestrictionList)
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
