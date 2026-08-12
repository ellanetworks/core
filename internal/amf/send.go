// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var nasSendTracer = otel.Tracer("ella-core/amf/nas/send")

func armNASGuard(conn *UeConn, ueConn *UeConn, cfg guard.TimerValue, name string, plain []byte, sht uint8, onExhausted func()) {
	ue := conn.UeContext()

	conn.armNASGuardWith(cfg, name,
		func(attempt int32) {
			logger.AmfLog.Warn("retransmitting NAS request", zap.String("timer", name), zap.Int32("attempt", attempt))

			if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
				return ueConn.SendDownlinkNASTransport(context.Background(), wire)
			}); err != nil {
				logger.AmfLog.Error("failed to retransmit NAS request", zap.String("timer", name), zap.Error(err))
			}
		},
		func() {
			logger.AmfLog.Warn("NAS guard exhausted, aborting procedure", zap.String("timer", name))
			onExhausted()
		},
	)
}

func sendGmm(ctx context.Context, ue *UeConn, spanName string, attrs []attribute.KeyValue, sht uint8, build func(*UeContext) ([]byte, error)) {
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

	plain, err := build(amfUe)
	if err != nil {
		ReportProtectFailure(ctx, amfUe, spanName, err)

		return
	}

	write := func(wire []byte) error {
		if err := ue.SendDownlinkNASTransport(ctx, wire); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", spanName), zap.Error(err))
		}

		return nil
	}

	if err := amfUe.SendDownlinkNAS(plain, sht, write); err != nil {
		ReportProtectFailure(ctx, amfUe, spanName, err)
	}
}

func SendDLNASTransport(ctx context.Context, ue *UeConn, payloadContainerType fgs.PayloadContainerType, nasPdu []byte, pduSessionID fgs.PDUSessionID, cause fgs.GMMCause) {
	var causePtr *fgs.GMMCause
	if cause != 0 {
		causePtr = &cause
	}

	sendGmm(ctx, ue, "nas/send_downlink_nas_transport",
		[]attribute.KeyValue{
			attribute.Int("pduSessionID", int(pduSessionID)),
			attribute.Int("cause", int(cause)),
		},
		uint8(fgs.SHTIntegrityProtectedCiphered),
		func(_ *UeContext) ([]byte, error) {
			return BuildDLNASTransport(payloadContainerType, nasPdu, &pduSessionID, causePtr, nil)
		})
}

func SendIdentityRequest(ctx context.Context, amfInstance *AMF, ue *UeConn, typeOfIdentity fgs.MobileIdentityType) {
	if ue == nil || ue.UeContext() == nil {
		logger.AmfLog.Error("cannot send Identity Request: ue or amf ue is nil")
		return
	}

	amfUe := ue.UeContext()

	ctx, span := nasSendTracer.Start(ctx, "nas/send_identity_request",
		trace.WithAttributes(
			attribute.String("supi", amfUe.supi.String()),
			attribute.Int("typeOfIdentity", int(typeOfIdentity)),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	conn := amfUe.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot send Identity Request: no active NAS connection")
		return
	}

	nasMsg, err := BuildIdentityRequest(typeOfIdentity)
	if err != nil {
		ReportProtectFailure(ctx, amfUe, "identity request", err)
		return
	}

	armNASGuard(conn, ue, amfInstance.NASGuardCfg, "T3570 (Identity Request)", nasMsg, uint8(fgs.SHTPlain), func() {
		amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
	})

	if err := amfUe.SendDownlinkNAS(nasMsg, uint8(fgs.SHTPlain), func(wire []byte) error {
		return ue.SendDownlinkNASTransport(ctx, wire)
	}); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_identity_request"), zap.Error(err))
	}
}

func SendAuthenticationRequest(ctx context.Context, amfInstance *AMF, ue *UeConn) {
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

	conn := amfUe.Conn()
	if conn == nil || conn.AuthenticationCtx == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot send Authentication Request: authentication context of UE is nil")
		return
	}

	nasMsg, err := BuildAuthenticationRequest(amfUe)
	if err != nil {
		ReportProtectFailure(ctx, amfUe, "authentication request", err)
		return
	}

	armNASGuard(conn, ue, amfInstance.NASGuardCfg, "T3560 (Authentication Request)", nasMsg, uint8(fgs.SHTPlain), func() {
		amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
	})

	if err := amfUe.SendDownlinkNAS(nasMsg, uint8(fgs.SHTPlain), func(wire []byte) error {
		return ue.SendDownlinkNASTransport(ctx, wire)
	}); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_authentication_request"), zap.Error(err))
	}
}

func SendAuthenticationReject(ctx context.Context, ue *UeConn) {
	sendGmm(ctx, ue, "nas/send_authentication_reject", nil, uint8(fgs.SHTPlain),
		func(_ *UeContext) ([]byte, error) { return BuildAuthenticationReject() })
}

func SendServiceReject(ctx context.Context, ue *UeConn, cause fgs.GMMCause) {
	sendGmm(ctx, ue, "nas/send_service_reject",
		[]attribute.KeyValue{attribute.Int("cause", int(cause))}, uint8(fgs.SHTPlain),
		func(_ *UeContext) ([]byte, error) { return BuildServiceReject(cause) })
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
func SendRegistrationReject(ctx context.Context, ue *UeConn, cause5GMM fgs.GMMCause) {
	sendGmm(ctx, ue, "nas/send_registration_reject",
		[]attribute.KeyValue{attribute.Int("cause", int(cause5GMM))},
		registrationRejectSHT(ue),
		func(_ *UeContext) ([]byte, error) {
			return BuildRegistrationReject(int(ue.amf.T3502Value.Seconds()), cause5GMM)
		})
}

func SendSecurityModeCommand(ctx context.Context, amfInstance *AMF, ue *UeConn) error {
	return sendSecurityModeCommand(ctx, amfInstance, ue, BuildSecurityModeCommand, true)
}

func SendEPSNASAlgorithmsSecurityModeCommand(ctx context.Context, amfInstance *AMF, ue *UeConn) error {
	return sendSecurityModeCommand(ctx, amfInstance, ue, BuildEPSNASAlgorithmsSecurityModeCommand, false)
}

func sendSecurityModeCommand(ctx context.Context, amfInstance *AMF, ue *UeConn, build func(*UeContext) ([]byte, error), newContext bool) error {
	if ue == nil || ue.UeContext() == nil {
		return fmt.Errorf("cannot send Security Mode Command: ue or amf ue is nil")
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_security_mode_command",
		trace.WithAttributes(
			attribute.String("supi", ue.UeContext().supi.String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	amfUe := ue.UeContext()

	plain, err := build(amfUe)
	if err != nil {
		return fmt.Errorf("failed to build security mode command: %w", err)
	}

	sht := uint8(fgs.SHTIntegrityProtectedNewContext)

	if err := amfUe.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		if err := ue.SendDownlinkNASTransport(ctx, wire); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_security_mode_command"), zap.Error(err))
		}

		return nil
	}); err != nil {
		if newContext {
			amfUe.ClearSecured()
		}

		return fmt.Errorf("failed to protect security mode command: %w", err)
	}

	conn := amfUe.Conn()
	armNASGuard(conn, ue, amfInstance.NASGuardCfg, "T3560 (Security Mode Command)", plain, sht, func() {
		amfUe.EndKeyChainProc(procedure.SecurityMode)
		amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
	})

	return nil
}

func registrationRejectSHT(ue *UeConn) uint8 {
	if ue.SecureExchangeEstablished() {
		return uint8(fgs.SHTIntegrityProtectedCiphered)
	}

	return uint8(fgs.SHTPlain)
}

func SendDeregistrationAccept(ctx context.Context, ue *UeConn) {
	sendGmm(ctx, ue, "nas/send_deregistration_accept", nil, uint8(fgs.SHTPlain),
		func(_ *UeContext) ([]byte, error) { return BuildDeregistrationAccept() })
}

func SendRegistrationAccept(
	ctx context.Context,
	amfInstance *AMF,
	ue *UeContext,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	pduSessionResourceSetupList ngap.PDUSessionResourceSetupListCxtReq,
	equivalentPlmnID models.PlmnID,
	supportedGUAMI *models.Guami,
) {
	if ue == nil {
		logger.AmfLog.Error("cannot send Registration Accept: ue is nil")
		return
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_registration_accept",
		trace.WithAttributes(
			attribute.String("supi", ue.Supi().String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	guti, err := amfInstance.Guti(supportedGUAMI, ue)
	if err != nil {
		ReportProtectFailure(ctx, ue, "5G-GUTI for registration accept", err)
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot send Registration Accept: ueConn is nil")
		return
	}

	plain, err := BuildRegistrationAccept(amfInstance, ue, guti, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, equivalentPlmnID)
	if err != nil {
		ReportProtectFailure(ctx, ue, "registration accept", err)

		return
	}

	sht := uint8(fgs.SHTIntegrityProtectedCiphered)

	if conn := ue.Conn(); conn != nil {
		conn.RegistrationAcceptPlain = plain
	}

	kgnb, ueSecCap := ue.Kgnb(), ue.UESecCap()

	if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		if ueConn.UeContextRequest {
			ueConn.MarkICSPending()

			if err := ueConn.SendInitialContextSetup(
				ctx,
				ue.Ambr.Uplink,
				ue.Ambr.Downlink,
				ue.AllowedNssai,
				kgnb,
				ue.RadioCapability,
				ue.RadioCapabilityForPaging,
				ueSecCap,
				wire,
				pduSessionResourceSetupList,
				supportedGUAMI,
			); err != nil {
				logger.From(ctx, logger.AmfLog).Warn("failed to send initial context setup request", zap.Error(err))
			} else {
				logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request")
			}

			return nil
		}

		if err := ueConn.SendDownlinkNASTransport(ctx, wire); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.Error(err))
		} else {
			logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")
		}

		return nil
	}); err != nil {
		ReportProtectFailure(ctx, ue, "registration accept", err)

		return
	}

	if amfInstance.NASGuardCfg.Enable {
		cfg := amfInstance.NASGuardCfg
		conn := ue.Conn()
		conn.armNASGuardWith(cfg, "T3550 (Registration Accept)", func(expireTimes int32) {
			retryUeConn := ue.Conn()
			if retryUeConn == nil {
				logger.From(ctx, logger.AmfLog).Warn("[NAS] UE Context released, abort retransmission of Registration Accept")

				return
			}

			if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
				if retryUeConn.UeContextRequest && retryUeConn.ICS() != ICSCompleted {
					if err := retryUeConn.SendInitialContextSetup(
						context.Background(),
						ue.Ambr.Uplink,
						ue.Ambr.Downlink,
						ue.AllowedNssai,
						kgnb,
						ue.RadioCapability,
						ue.RadioCapabilityForPaging,
						ueSecCap,
						wire,
						pduSessionResourceSetupList,
						supportedGUAMI,
					); err != nil {
						logger.From(ctx, logger.AmfLog).Error("could not send initial context setup request", zap.Error(err))
					}

					retryUeConn.MarkICSPending()

					logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request")

					return nil
				}

				logger.From(ctx, logger.AmfLog).Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))

				if err := retryUeConn.SendDownlinkNASTransport(context.Background(), wire); err != nil {
					logger.From(ctx, logger.AmfLog).Error("could not send downlink NAS transport message", zap.Error(err))
				}

				logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")

				return nil
			}); err != nil {
				logger.From(ctx, logger.AmfLog).Error("could not retransmit Registration Accept", zap.Error(err))
			}
		}, func() {
			logger.From(ctx, logger.AmfLog).Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))

			ue.TransitionTo(Registered)
			ue.ClearRegistrationRequestData()
		})
	}
}

func ArmRegistrationAcceptGuard(amfInstance *AMF, ue *UeContext, plain []byte) {
	if !amfInstance.NASGuardCfg.Enable {
		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	cfg := amfInstance.NASGuardCfg
	conn.armNASGuardWith(cfg, "T3550 (Registration Accept)", func(expireTimes int32) {
		retryUeConn := ue.Conn()
		if retryUeConn == nil {
			logger.AmfLog.Warn("UE context released, abort retransmission of Registration Accept")
			return
		}

		logger.AmfLog.Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))

		if err := ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
			return retryUeConn.SendDownlinkNASTransport(context.Background(), wire)
		}); err != nil {
			logger.AmfLog.Error("could not retransmit Registration Accept", zap.Error(err))
		}
	}, func() {
		logger.AmfLog.Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))
		ue.TransitionTo(Registered)
		ue.ClearRegistrationRequestData()
	})
}

func ResendRegistrationAccept(ctx context.Context, amfInstance *AMF, ue *UeContext) {
	conn := ue.Conn()
	if conn == nil || len(conn.RegistrationAcceptPlain) == 0 {
		return
	}

	plain := conn.RegistrationAcceptPlain

	if err := ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
		return conn.SendDownlinkNASTransport(ctx, wire)
	}); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to resend Registration Accept", zap.Error(err))
	}

	ArmRegistrationAcceptGuard(amfInstance, ue, plain)
}

func SendConfigurationUpdateCommand(ctx context.Context, amfInstance *AMF, amfUe *UeContext, includeGUTI bool) {
	if amfUe == nil {
		return
	}

	ctx, span := nasSendTracer.Start(ctx, "nas/send_configuration_update_command",
		trace.WithAttributes(
			attribute.String("supi", amfUe.Supi().String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	ueConn := amfUe.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot SendConfigurationUpdateCommand: UeConn is nil")
		return
	}

	operator, err := amfInstance.DBInstance.GetOperator(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("cannot SendConfigurationUpdateCommand: failed to get operator", zap.Error(err))
		return
	}

	operatorInfo, err := amfInstance.operatorInfoFrom(operator)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("cannot SendConfigurationUpdateCommand: failed to get operator info", zap.Error(err))
		return
	}

	guti, err := amfInstance.Guti(operatorInfo.Guami, amfUe)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("cannot SendConfigurationUpdateCommand: failed to build 5G-GUTI", zap.Error(err))
		return
	}

	plain, err := BuildConfigurationUpdateCommand(guti, operator.SpnFullName, operator.SpnShortName, includeGUTI)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("error building ConfigurationUpdateCommand", zap.Error(err))

		return
	}

	sht := uint8(fgs.SHTIntegrityProtectedCiphered)

	logger.From(ctx, logger.AmfLog).Info("nas/send_configuration_update_command")

	if err := amfUe.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		return ueConn.SendDownlinkNASTransport(ctx, wire)
	}); err != nil {
		logger.From(ctx, logger.AmfLog).Error("could not send configuration update command", zap.Error(err))

		return
	}

	if amfInstance.NASGuardCfg.Enable {
		cfg := amfInstance.NASGuardCfg

		logger.From(ctx, logger.AmfLog).Info("start T3555 timer")

		conn := amfUe.Conn()
		if conn == nil {
			return
		}

		conn.armNASGuardWith(cfg, "T3555 (Configuration Update)", func(expireTimes int32) {
			logger.From(ctx, logger.AmfLog).Warn("timer T3555 expired, retransmit Configuration Update Command", zap.Int32("retry", expireTimes))

			retryUeConn := amfUe.Conn()
			if retryUeConn == nil {
				logger.From(ctx, logger.AmfLog).Warn("UE Context released, abort retransmission of Configuration Update Command")

				return
			}

			if retryUeConn.Radio() == nil {
				logger.From(ctx, logger.AmfLog).Warn("Radio is nil, abort retransmission of Configuration Update Command")
				return
			}

			if err := amfUe.SendDownlinkNAS(plain, sht, func(wire []byte) error {
				return retryUeConn.SendDownlinkNASTransport(context.Background(), wire)
			}); err != nil {
				logger.From(ctx, logger.AmfLog).Error("could not send configuration update command", zap.Error(err))
			}
		}, func() {
			logger.From(ctx, logger.AmfLog).Warn("timer T3555 expired too many times, aborting configuration update procedure", zap.Int32("maximum retries", cfg.MaxRetryTimes))
		},
		)
	}
}

// SendNGAP writes a complete NGAP PDU to this UE's gNB association (via its conn).
func (ueConn *UeConn) SendNGAP(ctx context.Context, msgType NGAPProcedure, pkt []byte) {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to resolve NGAP send target", zap.String("procedure", string(msgType)), zap.Error(err))
		return
	}

	// The send failure, if any, is logged at the chokepoint.
	_ = amfInstance.SendToRadio(ctx, conn, msgType, pkt)
}

// downlinkNASTransportBytes builds a Downlink NAS Transport PDU carrying nas for
// the given NGAP identities (TS 38.413 §9.2.5.2).
func downlinkNASTransportBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, nas []byte) ([]byte, error) {
	msg := &ngap.DownlinkNASTransport{
		AMFUENGAPID: amfID,
		RANUENGAPID: ranID,
		NASPDU:      ngap.NASPDU(nas),
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendDownlinkNASTransport(ctx context.Context, nasPdu []byte) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := downlinkNASTransportBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), nasPdu)
	if err != nil {
		return err
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedureDownlinkNASTransport, pkt)
}

func downlinkUEAssociatedNRPPaTransportBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, routingID int64, nrppaPdu []byte) ([]byte, error) {
	msg := &ngap.DownlinkUEAssociatedNRPPaTransport{
		AMFUENGAPID: amfID,
		RANUENGAPID: ranID,
		RoutingID: ngap.RoutingID{
			byte(routingID >> 24), byte(routingID >> 16), byte(routingID >> 8), byte(routingID),
		},
		NRPPaPDU: ngap.NRPPaPDU(nrppaPdu),
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendDownlinkNRPPaTransport(ctx context.Context, routingID int64, nrppaPdu []byte) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := downlinkUEAssociatedNRPPaTransportBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), routingID, nrppaPdu)
	if err != nil {
		return fmt.Errorf("build downlink NRPPa transport: %w", err)
	}

	if ue := ueConn.UeContext(); ue != nil {
		ue.RecordNRPPaRoutingID(routingID)
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedureDownlinkNRPPaTransport, pkt)
}

func ueContextReleaseCommandBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, cause ngap.Cause) ([]byte, error) {
	msg := &ngap.UEContextReleaseCommand{
		UENGAPIDs: ngap.UENGAPIDs{
			AMFUENGAPID: amfID,
			RANUENGAPID: ranID,
			Pair:        ranID != ngap.RANUENGAPID(models.RanUeNgapIDUnspecified),
		},
		Cause: &cause,
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendUEContextReleaseCommand(ctx context.Context, cause ngap.Cause) {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to resolve send target for UE Context Release Command", zap.Error(err))
		return
	}

	// Idempotent: a release already in flight for this RAN UE (an eNB-initiated release
	// racing a NAS-guard timeout, or two handover-abort paths) must not send a second
	// UE Context Release Command.
	if !amfInstance.claimRelease(ueConn) {
		logger.WithTrace(ctx, ueConn.Log).Debug("UE Context Release already in progress; suppressing duplicate")
		return
	}

	pkt, err := ueContextReleaseCommandBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), cause)
	if err != nil {
		// The command cannot be sent, so no Release Complete will arrive; release
		// locally now to avoid leaking the UeConn and its claim.
		logger.From(ctx, ueConn.Log).Error("failed to build UE Context Release Command", zap.Error(err))
		amfInstance.ReleaseUeConn(ctx, ueConn)

		return
	}

	if err := amfInstance.SendToRadio(ctx, conn, NGAPProcedureUEContextReleaseCommand, pkt); err != nil {
		// The chokepoint logs the send failure; release locally so the UeConn cannot leak.
		amfInstance.ReleaseUeConn(ctx, ueConn)

		return
	}

	ueConn.releaseGuard.Arm(releaseGuardTimeout, 0, nil, func() {
		amfInstance.ReleaseUeConn(context.Background(), ueConn)
	})
}

func PDUSessionSetupItemSUReq(pduSessionID uint8, snssai *models.Snssai, nasPDU, transfer []byte) (ngap.PDUSessionResourceSetupItemSUReq, error) {
	if snssai == nil {
		return ngap.PDUSessionResourceSetupItemSUReq{}, fmt.Errorf("S-NSSAI is required")
	}

	s, err := util.SNSSAIToNGAP(*snssai)
	if err != nil {
		return ngap.PDUSessionResourceSetupItemSUReq{}, fmt.Errorf("could not convert S-NSSAI: %w", err)
	}

	item := ngap.PDUSessionResourceSetupItemSUReq{
		PDUSessionID: ngap.PDUSessionID(pduSessionID),
		SNSSAI:       s,
		Transfer:     ngap.TransferContainer(transfer),
	}

	if nasPDU != nil {
		item.NASPDU = ngap.Ptr(ngap.NASPDU(nasPDU))
	}

	return item, nil
}

// pduSessionResourceSetupBytes builds a PDU SESSION RESOURCE SETUP REQUEST
// (TS 38.413 §9.2.1.1).
func pduSessionResourceSetupBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, ambrUp, ambrDown models.BitRate, nasPdu []byte, sessions ngap.PDUSessionResourceSetupListSUReq) ([]byte, error) {
	msg := &ngap.PDUSessionResourceSetupRequest{
		AMFUENGAPID:             amfID,
		RANUENGAPID:             ranID,
		PDUSessionResourceSetup: sessions,
		UEAggregateMaximumBitRate: &ngap.UEAggregateMaximumBitRate{
			DL: ngap.BitRate(ambrDown.Bps()),
			UL: ngap.BitRate(ambrUp.Bps()),
		},
	}

	if nasPdu != nil {
		msg.NASPDU = ngap.Ptr(ngap.NASPDU(nasPdu))
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendPDUSessionResourceSetupRequest(ctx context.Context, ambrUp models.BitRate, ambrDown models.BitRate, nasPdu []byte, list ngap.PDUSessionResourceSetupListSUReq) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := pduSessionResourceSetupBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), ambrUp, ambrDown, nasPdu, list)
	if err != nil {
		return err
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedurePDUSessionResourceSetupRequest, pkt)
}

func pduSessionResourceReleaseBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, nasPdu []byte, sessions ngap.PDUSessionResourceToReleaseListRelCmd) ([]byte, error) {
	msg := &ngap.PDUSessionResourceReleaseCommand{
		AMFUENGAPID:               amfID,
		RANUENGAPID:               ranID,
		PDUSessionResourceRelease: sessions,
	}

	if nasPdu != nil {
		msg.NASPDU = ngap.Ptr(ngap.NASPDU(nasPdu))
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendPDUSessionResourceReleaseCommand(ctx context.Context, nasPdu []byte, list ngap.PDUSessionResourceToReleaseListRelCmd) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := pduSessionResourceReleaseBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), nasPdu, list)
	if err != nil {
		return err
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedurePDUSessionResourceReleaseCommand, pkt)
}

func PDUSessionSetupItem(pduSessionID uint8, snssai *models.Snssai, nasPDU, transfer []byte) (ngap.PDUSessionResourceSetupItemCxtReq, error) {
	if snssai == nil {
		return ngap.PDUSessionResourceSetupItemCxtReq{}, fmt.Errorf("S-NSSAI is required")
	}

	s, err := util.SNSSAIToNGAP(*snssai)
	if err != nil {
		return ngap.PDUSessionResourceSetupItemCxtReq{}, fmt.Errorf("could not convert S-NSSAI: %w", err)
	}

	item := ngap.PDUSessionResourceSetupItemCxtReq{
		PDUSessionID: ngap.PDUSessionID(pduSessionID),
		SNSSAI:       s,
		Transfer:     ngap.TransferContainer(transfer),
	}

	if nasPDU != nil {
		item.NASPDU = ngap.Ptr(ngap.NASPDU(nasPDU))
	}

	return item, nil
}

// initialContextSetupBytes builds an INITIAL CONTEXT SETUP REQUEST
// (TS 38.413 §9.2.2.1).
func initialContextSetupBytes(
	amfID ngap.AMFUENGAPID,
	ranID ngap.RANUENGAPID,
	ambrUp, ambrDown models.BitRate,
	allowedNssai []models.Snssai,
	kgnb []byte,
	ueRadioCapability []byte,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueSecurityCapability *fgs.UESecurityCapability,
	nasPdu []byte,
	sessions ngap.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) ([]byte, error) {
	if ueSecurityCapability == nil {
		return nil, fmt.Errorf("UE security capability is required")
	}

	if supportedGUAMI == nil || supportedGUAMI.PlmnID == nil {
		return nil, fmt.Errorf("GUAMI is required")
	}

	guami, err := util.GUAMIToNGAP(*supportedGUAMI)
	if err != nil {
		return nil, fmt.Errorf("could not convert GUAMI: %w", err)
	}

	allowed, err := util.AllowedNSSAIToNGAP(allowedNssai)
	if err != nil {
		return nil, err
	}

	if len(kgnb) != 32 {
		return nil, fmt.Errorf("K_gNB is %d octets, want 32", len(kgnb))
	}

	msg := &ngap.InitialContextSetupRequest{
		AMFUENGAPID:            amfID,
		RANUENGAPID:            ranID,
		GUAMI:                  guami,
		AllowedNSSAI:           allowed,
		UESecurityCapabilities: util.SecurityCapabilitiesToNGAP(ueSecurityCapability),
		UERadioCapability:      ngap.UERadioCapability(ueRadioCapability),
	}

	copy(msg.SecurityKey[:], kgnb)

	// §9.2.2.1 makes the UE Aggregate Maximum Bit Rate conditional on the PDU
	// session list, so the two travel together.
	if len(sessions) > 0 {
		msg.PDUSessionResourceSetup = sessions
		msg.UEAggregateMaximumBitRate = &ngap.UEAggregateMaximumBitRate{
			DL: ngap.BitRate(ambrDown.Bps()),
			UL: ngap.BitRate(ambrUp.Bps()),
		}
	}

	if nasPdu != nil {
		msg.NASPDU = ngap.Ptr(ngap.NASPDU(nasPdu))
	}

	msg.UERadioCapabilityForPaging = util.RadioCapabilityForPagingToNGAP(ueRadioCapabilityForPaging)

	return msg.Marshal()
}

func (ueConn *UeConn) SendInitialContextSetup(
	ctx context.Context,
	ambrUp models.BitRate,
	ambrDown models.BitRate,
	allowedNssai []models.Snssai,
	kgnb []byte,
	ueRadioCapability []byte,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueSecurityCapability *fgs.UESecurityCapability,
	nasPdu []byte,
	sessions ngap.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := initialContextSetupBytes(
		ngap.AMFUENGAPID(ueConn.AmfUeNgapID),
		ngap.RANUENGAPID(ueConn.RanUeNgapID),
		ambrUp,
		ambrDown,
		allowedNssai,
		kgnb,
		ueRadioCapability,
		ueRadioCapabilityForPaging,
		ueSecurityCapability,
		nasPdu,
		sessions,
		supportedGUAMI,
	)
	if err != nil {
		return err
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedureInitialContextSetupRequest, pkt)
}

// pduSessionResourceModifyBytes builds a PDU SESSION RESOURCE MODIFY REQUEST
// (TS 38.413 §9.2.1.5).
func pduSessionResourceModifyBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, sessions ngap.PDUSessionResourceModifyListModReq) ([]byte, error) {
	msg := &ngap.PDUSessionResourceModifyRequest{
		AMFUENGAPID:              amfID,
		RANUENGAPID:              ranID,
		PDUSessionResourceModify: sessions,
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendPDUSessionResourceModifyRequest(
	ctx context.Context,
	pduSessionResourceModifyList ngap.PDUSessionResourceModifyListModReq,
) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := pduSessionResourceModifyBytes(
		ngap.AMFUENGAPID(ueConn.AmfUeNgapID),
		ngap.RANUENGAPID(ueConn.RanUeNgapID),
		pduSessionResourceModifyList,
	)
	if err != nil {
		return err
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedurePDUSessionResourceModifyRequest, pkt)
}

// handoverPreparationFailureBytes builds a Handover Preparation Failure for the
// given NGAP identities (TS 38.413 §9.2.3.3).
func handoverPreparationFailureBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, cause ngap.Cause, diagnostics *ngap.CriticalityDiagnostics, targetFailure ngap.TargettoSourceFailureTransparentContainer) ([]byte, error) {
	msg := &ngap.HandoverPreparationFailure{
		AMFUENGAPID:            &amfID,
		RANUENGAPID:            &ranID,
		Cause:                  &cause,
		CriticalityDiagnostics: diagnostics,
		TargettoSourceFailureTransparentContainer: targetFailure,
	}

	return msg.Marshal()
}

// targetFailure is the container the handover target supplied in HANDOVER
// FAILURE, which §8.4.1.3 has the AMF pass on to the source NG-RAN node; it is
// nil where preparation failed before any target answered.
func (ueConn *UeConn) SendHandoverPreparationFailure(ctx context.Context, cause ngap.Cause, criticalityDiagnostics *ngap.CriticalityDiagnostics, targetFailure ngap.TargettoSourceFailureTransparentContainer) {
	pkt, err := handoverPreparationFailureBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), cause, criticalityDiagnostics, targetFailure)
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to build Handover Preparation Failure", zap.Error(err))
		return
	}

	ueConn.SendNGAP(ctx, NGAPProcedureHandoverPreparationFailure, pkt)
}

// handoverCancelAcknowledgeBytes builds a Handover Cancel Acknowledge for the
// given NGAP identities (TS 38.413 §9.2.3.12).
func handoverCancelAcknowledgeBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID) ([]byte, error) {
	msg := &ngap.HandoverCancelAcknowledge{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendHandoverCancelAcknowledge(ctx context.Context) {
	pkt, err := handoverCancelAcknowledgeBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID))
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to build Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	ueConn.SendNGAP(ctx, NGAPProcedureHandoverCancelAcknowledge, pkt)
}

// PDUSessionSetupItemHOReq builds one item for a HANDOVER REQUEST
// (TS 38.413 §9.2.3.4). The transfer is the SMF's, carried opaquely.
func PDUSessionSetupItemHOReq(pduSessionID uint8, snssai *models.Snssai, transfer []byte) (ngap.PDUSessionResourceSetupItemHOReq, error) {
	if snssai == nil {
		return ngap.PDUSessionResourceSetupItemHOReq{}, fmt.Errorf("S-NSSAI is required")
	}

	s, err := util.SNSSAIToNGAP(*snssai)
	if err != nil {
		return ngap.PDUSessionResourceSetupItemHOReq{}, fmt.Errorf("could not convert S-NSSAI: %w", err)
	}

	return ngap.PDUSessionResourceSetupItemHOReq{
		PDUSessionID: ngap.PDUSessionID(pduSessionID),
		SNSSAI:       s,
		Transfer:     ngap.TransferContainer(transfer),
	}, nil
}

// handoverRequestBytes builds a HANDOVER REQUEST (TS 38.413 §9.2.3.4). The
// {NH, NCC} pair is the AS key chain the target derives its keys from; it is
// staged at preparation and committed only when the UE arrives (TS 33.501).
type HandoverRequestOpts struct {
	HandoverType         ngap.HandoverType
	UplinkAmbr           models.BitRate
	DownlinkAmbr         models.BitRate
	UESecurityCapability *fgs.UESecurityCapability
	NCC                  uint8
	NH                   []byte
	Cause                ngap.Cause
	Sessions             ngap.PDUSessionResourceSetupListHOReq
	SourceToTarget       ngap.SourceToTargetTransparentContainer
	SnssaiList           []models.Snssai
	GUAMI                *models.Guami
	NASC                 []byte
	NewSecurityContext   bool
	ServingPLMN          *models.PlmnID
}

func handoverRequestBytes(amfID ngap.AMFUENGAPID, opts HandoverRequestOpts, allowedNSSAI ngap.AllowedNSSAI, guami ngap.GUAMI) ([]byte, error) {
	var nextHop ngap.SecurityKey

	if len(opts.NH) != len(nextHop) {
		return nil, fmt.Errorf("next hop is %d octets, want %d", len(opts.NH), len(nextHop))
	}

	copy(nextHop[:], opts.NH)

	msg := &ngap.HandoverRequest{
		AMFUENGAPID:  amfID,
		HandoverType: opts.HandoverType,
		Cause:        &opts.Cause,
		UEAggregateMaximumBitRate: ngap.UEAggregateMaximumBitRate{
			DL: ngap.BitRate(opts.DownlinkAmbr.Bps()),
			UL: ngap.BitRate(opts.UplinkAmbr.Bps()),
		},
		UESecurityCapabilities:             util.SecurityCapabilitiesToNGAP(opts.UESecurityCapability),
		SecurityContext:                    ngap.SecurityContext{NextHopChainingCount: opts.NCC, NextHopNH: nextHop},
		NASC:                               opts.NASC,
		PDUSessionResourceSetupListHOReq:   opts.Sessions,
		AllowedNSSAI:                       allowedNSSAI,
		SourceToTargetTransparentContainer: opts.SourceToTarget,
		GUAMI:                              guami,
	}

	if opts.NewSecurityContext {
		msg.NewSecurityContextInd = ngap.Ptr(ngap.NewSecurityContextIndTrue)
	}

	if opts.ServingPLMN != nil {
		plmn, err := util.PLMNToNGAP(*opts.ServingPLMN)
		if err != nil {
			return nil, fmt.Errorf("could not convert the serving PLMN: %w", err)
		}

		msg.MobilityRestrictionList = &ngap.MobilityRestrictionList{ServingPLMN: plmn}
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendHandoverRequest(ctx context.Context, opts HandoverRequestOpts) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	if opts.GUAMI == nil {
		return fmt.Errorf("no GUAMI to name this AMF with")
	}

	guami, err := util.GUAMIToNGAP(*opts.GUAMI)
	if err != nil {
		return fmt.Errorf("could not convert GUAMI: %w", err)
	}

	allowed, err := util.AllowedNSSAIToNGAP(opts.SnssaiList)
	if err != nil {
		return fmt.Errorf("could not convert Allowed NSSAI: %w", err)
	}

	pkt, err := handoverRequestBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), opts, allowed, guami)
	if err != nil {
		return err
	}

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedureHandoverRequest, pkt)
}

// handoverCommandBytes builds a HANDOVER COMMAND (TS 38.413 §9.2.3.2).
func handoverCommandBytes(
	amfID ngap.AMFUENGAPID,
	ranID ngap.RANUENGAPID,
	handoverType ngap.HandoverType,
	admitted ngap.PDUSessionResourceHandoverList,
	toRelease ngap.PDUSessionResourceToReleaseListHOCmd,
	targetToSource ngap.TargetToSourceTransparentContainer,
	nasSecurityParameters ngap.NASSecurityParametersFromNGRAN,
) ([]byte, error) {
	msg := &ngap.HandoverCommand{
		AMFUENGAPID:                        amfID,
		RANUENGAPID:                        ranID,
		HandoverType:                       handoverType,
		NASSecurityParametersFromNGRAN:     nasSecurityParameters,
		PDUSessionResourceHandoverList:     admitted,
		PDUSessionResourceToReleaseList:    toRelease,
		TargetToSourceTransparentContainer: targetToSource,
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendHandoverCommand(
	ctx context.Context,
	admitted ngap.PDUSessionResourceHandoverList,
	toRelease ngap.PDUSessionResourceToReleaseListHOCmd,
	targetToSource ngap.TargetToSourceTransparentContainer,
) {
	pkt, err := handoverCommandBytes(
		ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID),
		ueConn.HandOverType, admitted, toRelease, targetToSource, nil,
	)
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to build Handover Command", zap.Error(err))
		return
	}

	ueConn.SendNGAP(ctx, NGAPProcedureHandoverCommand, pkt)
}

func (ueConn *UeConn) SendHandoverCommandToEPS(
	ctx context.Context,
	toRelease ngap.PDUSessionResourceToReleaseListHOCmd,
	targetToSource ngap.TargetToSourceTransparentContainer,
	nasSecurityParameters ngap.NASSecurityParametersFromNGRAN,
) {
	pkt, err := handoverCommandBytes(
		ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID),
		ngap.HandoverTypeFiveGSToEPS, nil, toRelease, targetToSource, nasSecurityParameters,
	)
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to build Handover Command to EPS", zap.Error(err))
		return
	}

	ueConn.SendNGAP(ctx, NGAPProcedureHandoverCommand, pkt)
}

func ReportProtectFailure(ctx context.Context, ue *UeContext, what string, err error) {
	log := logger.From(ctx, logger.AmfLog)

	if !errors.Is(err, nas.ErrCountExhausted) {
		log.Error("failed to build "+what, zap.Error(err))
		return
	}

	log.Error("downlink NAS COUNT exhausted, releasing the connection",
		zap.String("message", what), zap.Error(err))

	if conn := ue.Conn(); conn != nil {
		conn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
	}
}

// downlinkRANStatusTransferBytes builds a DOWNLINK RAN STATUS TRANSFER
// (TS 38.413 §9.2.3.14).
func downlinkRANStatusTransferBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, container ngap.StatusTransferContainer) ([]byte, error) {
	msg := &ngap.DownlinkRANStatusTransfer{
		AMFUENGAPID: amfID,
		RANUENGAPID: ranID,
		Container:   container,
	}

	return msg.Marshal()
}

// SendDownlinkRANStatusTransfer relays the source node's PDCP SN/HFN status to
// the handover target. The container is opaque to the AMF (TS 38.413 §9.3.1.108).
func (ueConn *UeConn) SendDownlinkRANStatusTransfer(ctx context.Context, container ngap.StatusTransferContainer) {
	pkt, err := downlinkRANStatusTransferBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), container)
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to build Downlink RAN Status Transfer", zap.Error(err))
		return
	}

	ueConn.SendNGAP(ctx, NGAPProcedureDownlinkRANStatusTransfer, pkt)
}

// pathSwitchRequestAcknowledgeBytes builds a PATH SWITCH REQUEST ACKNOWLEDGE
// (TS 38.413 §9.2.3.9). The {NH, NCC} pair is the AS key chain the target
// derives its next keys from (TS 33.501 §6.9.5).
func pathSwitchRequestAcknowledgeBytes(
	amfID ngap.AMFUENGAPID,
	ranID ngap.RANUENGAPID,
	ueSecurityCapability *fgs.UESecurityCapability,
	ncc uint8,
	nh []byte,
	switched ngap.PDUSessionResourceSwitchedList,
	released ngap.PDUSessionResourceReleasedListPSAck,
	allowedNSSAI ngap.AllowedNSSAI,
) ([]byte, error) {
	var nextHop ngap.SecurityKey

	if len(nh) != len(nextHop) {
		return nil, fmt.Errorf("next hop is %d octets, want %d", len(nh), len(nextHop))
	}

	copy(nextHop[:], nh)

	msg := &ngap.PathSwitchRequestAcknowledge{
		AMFUENGAPID:                    &amfID,
		RANUENGAPID:                    &ranID,
		SecurityContext:                ngap.SecurityContext{NextHopChainingCount: ncc, NextHopNH: nextHop},
		PDUSessionResourceSwitchedList: switched,
		PDUSessionResourceReleased:     released,
		AllowedNSSAI:                   allowedNSSAI,
	}

	if ueSecurityCapability != nil {
		msg.UESecurityCapabilities = ngap.Ptr(util.SecurityCapabilitiesToNGAP(ueSecurityCapability))
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendPathSwitchRequestAcknowledge(
	ctx context.Context,
	ueSecurityCapability *fgs.UESecurityCapability,
	ncc uint8,
	nh []byte,
	switched ngap.PDUSessionResourceSwitchedList,
	released ngap.PDUSessionResourceReleasedListPSAck,
	snssaiList []models.Snssai,
) {
	allowed, err := util.AllowedNSSAIToNGAP(snssaiList)
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("could not convert Allowed NSSAI", zap.Error(err))
		return
	}

	pkt, err := pathSwitchRequestAcknowledgeBytes(
		ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID),
		ueSecurityCapability, ncc, nh, switched, released, allowed,
	)
	if err != nil {
		logger.From(ctx, ueConn.Log).Error("failed to build Path Switch Request Acknowledge", zap.Error(err))
		return
	}

	ueConn.SendNGAP(ctx, NGAPProcedurePathSwitchRequestAcknowledge, pkt)
}

// locationReportingControlBytes builds a LOCATION REPORTING CONTROL
// (TS 38.413 §9.2.11.1).
func locationReportingControlBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, eventType ngap.EventType) ([]byte, error) {
	msg := &ngap.LocationReportingControl{
		AMFUENGAPID: amfID,
		RANUENGAPID: ranID,
		LocationReportingRequestType: &ngap.LocationReportingRequestType{
			EventType:  eventType,
			ReportArea: ngap.ReportAreaCell,
		},
	}

	return msg.Marshal()
}

// SendLocationReportingControl asks the NG-RAN node to start, change or stop
// reporting this UE's location (TS 38.413 §8.12.1).
func (ueConn *UeConn) SendLocationReportingControl(ctx context.Context, eventType ngap.EventType) error {
	pkt, err := locationReportingControlBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID), eventType)
	if err != nil {
		return fmt.Errorf("build LocationReportingControl: %w", err)
	}

	ueConn.SendNGAP(ctx, NGAPProcedureLocationReportingControl, pkt)

	return nil
}
