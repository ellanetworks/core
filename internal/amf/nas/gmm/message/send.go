// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/nas/send")

func SendDLNASTransport(ctx context.Context, ue *amf.RanUe, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_downlink_nas_transport",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
			attribute.Int("pduSessionID", int(pduSessionID)),
			attribute.Int("cause", int(cause)),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
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

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendIdentityRequest(ctx context.Context, ue *amf.RanUe, typeOfIdentity uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_identity_request",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
			attribute.Int("typeOfIdentity", int(typeOfIdentity)),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildIdentityRequest(typeOfIdentity)
	if err != nil {
		return fmt.Errorf("error building identity request: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_authentication_request",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
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

	if amfInstance.T3560Cfg.Enable {
		cfg := amfInstance.T3560Cfg
		amfUe.T3560 = amf.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Authentication Request", zap.Any("expireTimes", expireTimes))

			err := ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
			if err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort authentication procedure & ongoing 5GMM procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfInstance.DeregisterAndRemoveAMFUE(context.Background(), amfUe)
		})
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendServiceAccept(ctx context.Context, ue *amf.RanUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_service_accept",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
			attribute.Int("pduSessionIDErrorCount", len(errPduSessionID)),
			attribute.Int("causeErrorCount", len(errCause)),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildServiceAccept(ue.AmfUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building service accept: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendAuthenticationReject(ctx context.Context, ue *amf.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_authentication_reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildAuthenticationReject()
	if err != nil {
		return fmt.Errorf("error building authentication reject: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendServiceReject(ctx context.Context, ue *amf.RanUe, cause uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_service_reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
			attribute.Int("cause", int(cause)),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildServiceReject(cause)
	if err != nil {
		return fmt.Errorf("error building service reject: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
func SendRegistrationReject(ctx context.Context, ue *amf.RanUe, cause5GMM uint8) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_registration_reject",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
			attribute.Int("cause", int(cause5GMM)),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationReject(int(ue.AmfUe.T3502Value.Seconds()), cause5GMM)
	if err != nil {
		return fmt.Errorf("error building registration reject: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendSecurityModeCommand(ctx context.Context, amfInstance *amf.AMF, ue *amf.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_security_mode_command",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildSecurityModeCommand(ue.AmfUe)
	if err != nil {
		return fmt.Errorf("error building security mode command: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	amfUe := ue.AmfUe

	if amfInstance.T3560Cfg.Enable {
		cfg := amfInstance.T3560Cfg
		amfUe.T3560 = amf.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Security Mode Command", zap.Any("expireTimes", expireTimes))

			err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
			if err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}

			amfUe.Log.Info("sent security mode command")
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort security mode control procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			// amfUe.Remove()
			amfInstance.DeregisterAndRemoveAMFUE(context.Background(), amfUe)
		})
	}

	return nil
}

func SendDeregistrationAccept(ctx context.Context, ue *amf.RanUe) error {
	if ue == nil || ue.AmfUe == nil {
		return fmt.Errorf("ue or amf ue is nil")
	}

	ctx, span := tracer.Start(ctx, "nas/send_deregistration_accept",
		trace.WithAttributes(
			attribute.String("supi", ue.AmfUe.Supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildDeregistrationAccept()
	if err != nil {
		return fmt.Errorf("error building deregistration accept: %s", err.Error())
	}

	err = ue.SendDownlinkNasTransport(ctx, nasMsg, nil)
	if err != nil {
		ue.AmfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	return nil
}

func SendRegistrationAccept(
	ctx context.Context,
	amfInstance *amf.AMF,
	ue *amf.AmfUe,
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

	ctx, span := tracer.Start(ctx, "nas/send_registration_accept",
		trace.WithAttributes(
			attribute.String("supi", ue.Supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationAccept(amfInstance, ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, supportedPLMN)
	if err != nil {
		return fmt.Errorf("error building registration accept: %s", err.Error())
	}

	if ue.RanUe.UeContextRequest {
		ue.RanUe.SentInitialContextSetupRequest = true

		err = ue.RanUe.SendInitialContextSetupRequest(
			ctx,
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
		err = ue.RanUe.SendDownlinkNasTransport(ctx, nasMsg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
		}

		ue.Log.Info("Sent GMM registration accept")
	}

	if amfInstance.T3550Cfg.Enable {
		cfg := amfInstance.T3550Cfg
		ue.T3550 = amf.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			if ue.RanUe == nil {
				ue.Log.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
				ue.T3550 = nil
			} else {
				if ue.RanUe.UeContextRequest && !ue.RanUe.RecvdInitialContextSetupResponse {
					err = ue.RanUe.SendInitialContextSetupRequest(
						ctx,
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

					err = ue.RanUe.SendDownlinkNasTransport(ctx, nasMsg, nil)
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
			ue.TransitionTo(amf.Registered)
			ue.ClearRegistrationRequestData()
		})
	}

	return nil
}

func SendConfigurationUpdateCommand(ctx context.Context, amfInstance *amf.AMF, amfUe *amf.AmfUe, includeGUTI bool) {
	if amfUe == nil {
		return
	}

	ctx, span := tracer.Start(ctx, "nas/send_configuration_update_command",
		trace.WithAttributes(
			attribute.String("supi", amfUe.Supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	if amfUe.RanUe == nil {
		amfUe.Log.Error("cannot SendConfigurationUpdateCommand: RanUe is nil")
		return
	}

	operator, err := amfInstance.DBInstance.GetOperator(ctx)
	if err != nil {
		amfUe.Log.Error("cannot SendConfigurationUpdateCommand: failed to get operator", zap.Error(err))
		return
	}

	nasMsg, err := BuildConfigurationUpdateCommand(amfUe, operator.SpnFullName, operator.SpnShortName, includeGUTI)
	if err != nil {
		amfUe.Log.Error("error building ConfigurationUpdateCommand", zap.Error(err))
		return
	}

	amfUe.Log.Info("nas/send_configuration_update_command")

	mobilityRestrictionList, err := send.BuildIEMobilityRestrictionList(amfUe.PlmnID)
	if err != nil {
		amfUe.Log.Error("could not build Mobility Restriction List IE", zap.Error(err))
		return
	}

	err = amfUe.RanUe.SendDownlinkNasTransport(ctx, nasMsg, mobilityRestrictionList)
	if err != nil {
		amfUe.Log.Error("could not send configuration update command", zap.Error(err))
		return
	}

	if amfInstance.T3555Cfg.Enable {
		cfg := amfInstance.T3555Cfg

		amfUe.Log.Info("start T3555 timer")
		amfUe.T3555 = amf.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("timer T3555 expired, retransmit Configuration Update Command", zap.Int32("retry", expireTimes))

			if amfUe.RanUe == nil {
				amfUe.Log.Warn("UE Context released, abort retransmission of Configuration Update Command")
				amfUe.T3555 = nil

				return
			}

			if amfUe.RanUe.Radio == nil {
				amfUe.Log.Warn("Radio is nil, abort retransmission of Configuration Update Command")
				return
			}

			err = amfUe.RanUe.SendDownlinkNasTransport(ctx, nasMsg, mobilityRestrictionList)
			if err != nil {
				amfUe.Log.Error("could not send configuration update command", zap.Error(err))
			}
		}, func() {
			amfUe.Log.Warn("timer T3555 expired too many times, aborting configuration update procedure", zap.Int32("maximum retries", cfg.MaxRetryTimes))
		},
		)
	}
}
