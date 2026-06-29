// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var nasSendTracer = otel.Tracer("ella-core/amf/nas/send")

// sendGmm builds a downlink 5GMM NAS message and hands it to the RAN over the
// signalling connection. A build failure is an invariant violation logged at
// Error (the message is dropped, failing safe); a transport failure is logged at
// Warn — delivery assurance is owned by the procedure's retransmission timer
// (TS 24.501), not the caller. It mirrors the MME's void+log NAS send leaf.
func sendGmm(ctx context.Context, ue *RanUe, spanName string, attrs []attribute.KeyValue, build func(*UeContext) ([]byte, error)) {
	if ue == nil || ue.UeContext() == nil {
		logger.AmfLog.Error("cannot send NAS message: ue or amf ue is nil", zap.String("message", spanName))
		return
	}

	amfUe := ue.UeContext()

	ctx, span := nasSendTracer.Start(ctx, spanName,
		trace.WithAttributes(append(attrs, attribute.String("supi", amfUe.supi.String()))...),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := build(amfUe)
	if err != nil {
		amfUe.Log.Error("failed to build NAS message", zap.String("message", spanName), zap.Error(err))
		return
	}

	if err := ue.SendDownlinkNasTransport(ctx, nasMsg, nil); err != nil {
		amfUe.Log.Warn("failed to send downlink NAS transport", zap.String("message", spanName), zap.Error(err))
	}
}

func SendDLNASTransport(ctx context.Context, ue *RanUe, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause uint8) {
	var causePtr *uint8
	if cause != 0 {
		causePtr = &cause
	}

	sendGmm(ctx, ue, "nas/send_downlink_nas_transport",
		[]attribute.KeyValue{
			attribute.Int("pduSessionID", int(pduSessionID)),
			attribute.Int("cause", int(cause)),
		},
		func(amfUe *UeContext) ([]byte, error) {
			return BuildDLNASTransport(amfUe, payloadContainerType, nasPdu, pduSessionID, causePtr)
		})
}

func SendIdentityRequest(ctx context.Context, ue *RanUe, typeOfIdentity uint8) {
	sendGmm(ctx, ue, "nas/send_identity_request",
		[]attribute.KeyValue{attribute.Int("typeOfIdentity", int(typeOfIdentity))},
		func(_ *UeContext) ([]byte, error) { return BuildIdentityRequest(typeOfIdentity) })
}

func SendAuthenticationRequest(ctx context.Context, amfInstance *AMF, ue *RanUe) {
	if ue == nil || ue.UeContext() == nil {
		logger.AmfLog.Error("cannot send Authentication Request: ue or amf ue is nil")
		return
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_authentication_request",
		trace.WithAttributes(
			attribute.String("supi", ue.UeContext().supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	amfUe := ue.UeContext()

	conn := amfUe.NasConn()
	if conn == nil || conn.AuthenticationCtx == nil {
		amfUe.Log.Error("cannot send Authentication Request: authentication context of UE is nil")
		return
	}

	nasMsg, err := BuildAuthenticationRequest(amfUe)
	if err != nil {
		amfUe.Log.Error("failed to build authentication request", zap.Error(err))
		return
	}

	if amfInstance.T3560Cfg.Enable {
		cfg := amfInstance.T3560Cfg
		conn.T3560.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Authentication Request", zap.Any("expireTimes", expireTimes))

			if err := ue.SendDownlinkNasTransport(context.Background(), nasMsg, nil); err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
			}
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort authentication procedure & ongoing 5GMM procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
		})
	}

	if err := ue.SendDownlinkNasTransport(ctx, nasMsg, nil); err != nil {
		amfUe.Log.Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_authentication_request"), zap.Error(err))
	}
}

func SendServiceAccept(ctx context.Context, ue *RanUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) {
	sendGmm(ctx, ue, "nas/send_service_accept",
		[]attribute.KeyValue{
			attribute.Int("pduSessionIDErrorCount", len(errPduSessionID)),
			attribute.Int("causeErrorCount", len(errCause)),
		},
		func(amfUe *UeContext) ([]byte, error) {
			return BuildServiceAccept(amfUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		})
}

func SendAuthenticationReject(ctx context.Context, ue *RanUe) {
	sendGmm(ctx, ue, "nas/send_authentication_reject", nil,
		func(_ *UeContext) ([]byte, error) { return BuildAuthenticationReject() })
}

func SendServiceReject(ctx context.Context, ue *RanUe, cause uint8) {
	sendGmm(ctx, ue, "nas/send_service_reject",
		[]attribute.KeyValue{attribute.Int("cause", int(cause))},
		func(_ *UeContext) ([]byte, error) { return BuildServiceReject(cause) })
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
func SendRegistrationReject(ctx context.Context, ue *RanUe, cause5GMM uint8) {
	sendGmm(ctx, ue, "nas/send_registration_reject",
		[]attribute.KeyValue{attribute.Int("cause", int(cause5GMM))},
		func(amfUe *UeContext) ([]byte, error) {
			return BuildRegistrationReject(int(amfUe.T3502Value.Seconds()), cause5GMM)
		})
}

func SendSecurityModeCommand(ctx context.Context, amfInstance *AMF, ue *RanUe) {
	if ue == nil || ue.UeContext() == nil {
		logger.AmfLog.Error("cannot send Security Mode Command: ue or amf ue is nil")
		return
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_security_mode_command",
		trace.WithAttributes(
			attribute.String("supi", ue.UeContext().supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	amfUe := ue.UeContext()

	nasMsg, err := BuildSecurityModeCommand(amfUe)
	if err != nil {
		amfUe.Log.Error("failed to build security mode command", zap.Error(err))
		return
	}

	if err := ue.SendDownlinkNasTransport(ctx, nasMsg, nil); err != nil {
		amfUe.Log.Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_security_mode_command"), zap.Error(err))
	}

	if amfInstance.T3560Cfg.Enable {
		cfg := amfInstance.T3560Cfg
		conn := amfUe.NasConn()
		conn.T3560.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("T3560 expires, retransmit Security Mode Command", zap.Any("expireTimes", expireTimes))

			if err := ue.SendDownlinkNasTransport(context.Background(), nasMsg, nil); err != nil {
				amfUe.Log.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}

			amfUe.Log.Info("sent security mode command")
		}, func() {
			amfUe.Log.Warn("T3560 Expires, abort security mode control procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			conn.Procedures.End(procedure.SecurityMode)
			amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
		})
	}
}

func SendDeregistrationAccept(ctx context.Context, ue *RanUe) {
	sendGmm(ctx, ue, "nas/send_deregistration_accept", nil,
		func(_ *UeContext) ([]byte, error) { return BuildDeregistrationAccept() })
}

func SendRegistrationAccept(
	ctx context.Context,
	amfInstance *AMF,
	ue *UeContext,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	pduSessionResourceSetupList *ngapType.PDUSessionResourceSetupListCxtReq,
	equivalentPlmnID models.PlmnID,
	supportedGUAMI *models.Guami,
) {
	if ue == nil {
		logger.AmfLog.Error("cannot send Registration Accept: ue is nil")
		return
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_registration_accept",
		trace.WithAttributes(
			attribute.String("supi", ue.SupiValue().String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	nasMsg, err := BuildRegistrationAccept(amfInstance, ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, equivalentPlmnID)
	if err != nil {
		ue.Log.Error("failed to build registration accept", zap.Error(err))
		return
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		ue.Log.Error("cannot send Registration Accept: ranUe is nil")
		return
	}

	if ranUe.UeContextRequest {
		ranUe.ICS = ICSPending

		if err := ranUe.SendInitialContextSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb(),
			ue.PlmnID,
			ue.UeRadioCapability,
			ue.UeRadioCapabilityForPaging,
			ue.UESecCap(),
			nasMsg,
			pduSessionResourceSetupList,
			supportedGUAMI,
		); err != nil {
			ue.Log.Warn("failed to send initial context setup request", zap.Error(err))
		} else {
			ue.Log.Info("Sent NGAP initial context setup request")
		}
	} else {
		if err := ranUe.SendDownlinkNasTransport(ctx, nasMsg, nil); err != nil {
			ue.Log.Warn("failed to send downlink NAS transport", zap.Error(err))
		} else {
			ue.Log.Info("Sent GMM registration accept")
		}
	}

	if amfInstance.T3550Cfg.Enable {
		cfg := amfInstance.T3550Cfg
		conn := ue.NasConn()
		conn.T3550.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			retryRanUe := ue.RanUe()
			if retryRanUe == nil {
				ue.Log.Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
			} else {
				if retryRanUe.UeContextRequest && retryRanUe.ICS != ICSCompleted {
					err = retryRanUe.SendInitialContextSetupRequest(
						context.Background(),
						ue.Ambr.Uplink,
						ue.Ambr.Downlink,
						ue.AllowedNssai,
						ue.Kgnb(),
						ue.PlmnID,
						ue.UeRadioCapability,
						ue.UeRadioCapabilityForPaging,
						ue.UESecCap(),
						nasMsg,
						pduSessionResourceSetupList,
						supportedGUAMI,
					)
					if err != nil {
						ue.Log.Error("could not send initial context setup request", zap.Error(err))
					}

					retryRanUe.ICS = ICSPending

					ue.Log.Info("Sent NGAP initial context setup request")
				} else {
					ue.Log.Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))

					err = retryRanUe.SendDownlinkNasTransport(context.Background(), nasMsg, nil)
					if err != nil {
						ue.Log.Error("could not send downlink NAS transport message", zap.Error(err))
					}

					ue.Log.Info("Sent GMM registration accept")
				}
			}
		}, func() {
			ue.Log.Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))

			// TS 24.501 5.5.1.2.8 case c, 5.5.1.3.8 case c
			ue.TransitionTo(Registered)
			ue.ClearRegistrationRequestData()
		})
	}
}

func SendConfigurationUpdateCommand(ctx context.Context, amfInstance *AMF, amfUe *UeContext, includeGUTI bool) {
	if amfUe == nil {
		return
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_configuration_update_command",
		trace.WithAttributes(
			attribute.String("supi", amfUe.SupiValue().String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	ranUe := amfUe.RanUe()
	if ranUe == nil {
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

	err = ranUe.SendDownlinkNasTransport(ctx, nasMsg, mobilityRestrictionList)
	if err != nil {
		amfUe.Log.Error("could not send configuration update command", zap.Error(err))
		return
	}

	if amfInstance.T3555Cfg.Enable {
		cfg := amfInstance.T3555Cfg

		amfUe.Log.Info("start T3555 timer")
		conn := amfUe.NasConn()
		conn.T3555.Arm(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.Log.Warn("timer T3555 expired, retransmit Configuration Update Command", zap.Int32("retry", expireTimes))

			retryRanUe := amfUe.RanUe()
			if retryRanUe == nil {
				amfUe.Log.Warn("UE Context released, abort retransmission of Configuration Update Command")

				return
			}

			if retryRanUe.Radio() == nil {
				amfUe.Log.Warn("Radio is nil, abort retransmission of Configuration Update Command")
				return
			}

			err = retryRanUe.SendDownlinkNasTransport(context.Background(), nasMsg, mobilityRestrictionList)
			if err != nil {
				amfUe.Log.Error("could not send configuration update command", zap.Error(err))
			}
		}, func() {
			amfUe.Log.Warn("timer T3555 expired too many times, aborting configuration update procedure", zap.Int32("maximum retries", cfg.MaxRetryTimes))
		},
		)
	}
}
