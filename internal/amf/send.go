// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var nasSendTracer = otel.Tracer("ella-core/amf/nas/send")

// armNASGuard supervises a UE-terminated NAS request: while the guard is enabled
// it retransmits nasMsg as a downlink NAS transport on each timer expiry, up to
// cfg.MaxRetryTimes, then runs onExhausted on the final expiry
// (TS 24.501 §10.2 T3550/T3560/T3570 — 6 s ×4).
func armNASGuard(conn *UeConn, ueConn *UeConn, cfg guard.TimerValue, name string, nasMsg []byte, onExhausted func()) {
	conn.armNASGuardWith(cfg, name,
		func(attempt int32) {
			logger.AmfLog.Warn("retransmitting NAS request", zap.String("timer", name), zap.Int32("attempt", attempt))

			if err := ueConn.SendDownlinkNASTransport(context.Background(), nasMsg); err != nil {
				logger.AmfLog.Error("failed to retransmit NAS request", zap.String("timer", name), zap.Error(err))
			}
		},
		func() {
			logger.AmfLog.Warn("NAS guard exhausted, aborting procedure", zap.String("timer", name))
			onExhausted()
		},
	)
}

// sendGmm builds a downlink 5GMM NAS message and hands it to the RAN over the
// signalling connection. A build failure is an invariant violation logged at
// Error (the message is dropped, failing safe); a transport failure is logged at
// Warn — delivery assurance is owned by the procedure's retransmission timer
// (TS 24.501), not the caller.
func sendGmm(ctx context.Context, ue *UeConn, spanName string, attrs []attribute.KeyValue, build func(*UeContext) ([]byte, error)) {
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
		logger.From(ctx, logger.AmfLog).Error("failed to build NAS message", zap.String("message", spanName), zap.Error(err))
		return
	}

	if err := ue.SendDownlinkNASTransport(ctx, nasMsg); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", spanName), zap.Error(err))
	}
}

func SendDLNASTransport(ctx context.Context, ue *UeConn, payloadContainerType uint8, nasPdu []byte, pduSessionID uint8, cause uint8) {
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

// SendIdentityRequest builds and sends the IDENTITY REQUEST and arms its T3570
// retransmission timer. On each expiry the request is retransmitted; on
// exhaustion the identification procedure and any ongoing 5GMM procedure are
// aborted and the UE is released (TS 24.501 §5.4.3.2).
func SendIdentityRequest(ctx context.Context, amfInstance *AMF, ue *UeConn, typeOfIdentity uint8) {
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
		logger.From(ctx, logger.AmfLog).Error("failed to build identity request", zap.Error(err))
		return
	}

	armNASGuard(conn, ue, amfInstance.NASGuardCfg, "T3570 (Identity Request)", nasMsg, func() {
		amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
	})

	if err := ue.SendDownlinkNASTransport(ctx, nasMsg); err != nil {
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
		logger.From(ctx, logger.AmfLog).Error("failed to build authentication request", zap.Error(err))
		return
	}

	armNASGuard(conn, ue, amfInstance.NASGuardCfg, "T3560 (Authentication Request)", nasMsg, func() {
		amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
	})

	if err := ue.SendDownlinkNASTransport(ctx, nasMsg); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_authentication_request"), zap.Error(err))
	}
}

func SendServiceAccept(ctx context.Context, ue *UeConn, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) {
	sendGmm(ctx, ue, "nas/send_service_accept",
		[]attribute.KeyValue{
			attribute.Int("pduSessionIDErrorCount", len(errPduSessionID)),
			attribute.Int("causeErrorCount", len(errCause)),
		},
		func(amfUe *UeContext) ([]byte, error) {
			return BuildServiceAccept(amfUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		})
}

func SendAuthenticationReject(ctx context.Context, ue *UeConn) {
	sendGmm(ctx, ue, "nas/send_authentication_reject", nil,
		func(_ *UeContext) ([]byte, error) { return BuildAuthenticationReject() })
}

func SendServiceReject(ctx context.Context, ue *UeConn, cause uint8) {
	sendGmm(ctx, ue, "nas/send_service_reject",
		[]attribute.KeyValue{attribute.Int("cause", int(cause))},
		func(_ *UeContext) ([]byte, error) { return BuildServiceReject(cause) })
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
func SendRegistrationReject(ctx context.Context, ue *UeConn, cause5GMM uint8) {
	sendGmm(ctx, ue, "nas/send_registration_reject",
		[]attribute.KeyValue{attribute.Int("cause", int(cause5GMM))},
		func(amfUe *UeContext) ([]byte, error) {
			return BuildRegistrationReject(int(ue.amf.T3502Value.Seconds()), cause5GMM)
		})
}

// SendSecurityModeCommand builds and sends the SECURITY MODE COMMAND and arms
// its T3560 retransmission timer. It returns an error only when the message
// cannot be built (nothing goes in flight, so the caller releases the procedure);
// a transport send failure is covered by T3560 and is not fatal.
func SendSecurityModeCommand(ctx context.Context, amfInstance *AMF, ue *UeConn) error {
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

	nasMsg, err := BuildSecurityModeCommand(amfUe)
	if err != nil {
		return fmt.Errorf("failed to build security mode command: %w", err)
	}

	if err := ue.SendDownlinkNASTransport(ctx, nasMsg); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.String("message", "nas/send_security_mode_command"), zap.Error(err))
	}

	conn := amfUe.Conn()
	armNASGuard(conn, ue, amfInstance.NASGuardCfg, "T3560 (Security Mode Command)", nasMsg, func() {
		conn.Parent().EndKeyChainProc(procedure.SecurityMode)
		amfInstance.DeregisterAndRemoveUeContext(context.Background(), amfUe)
	})

	return nil
}

func SendDeregistrationAccept(ctx context.Context, ue *UeConn) {
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
			attribute.String("supi", ue.Supi().String()),
		),
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	guti, err := amfInstance.Guti(supportedGUAMI, ue)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to build 5G-GUTI for registration accept", zap.Error(err))
		return
	}

	nasMsg, err := BuildRegistrationAccept(amfInstance, ue, guti, pDUSessionStatus, reactivationResult, errPduSessionID, errCause, equivalentPlmnID)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to build registration accept", zap.Error(err))
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Error("cannot send Registration Accept: ueConn is nil")
		return
	}

	if conn := ue.Conn(); conn != nil {
		// Keep the accept so a duplicate REGISTRATION REQUEST with identical IEs can be
		// answered by resending it (TS 24.501 §5.5.1.2.8 case d).
		conn.RegistrationAcceptPdu = nasMsg
	}

	if ueConn.UeContextRequest {
		ueConn.MarkICSPending()

		if err := ueConn.SendInitialContextSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb(),
			ue.RadioCapability,
			ue.RadioCapabilityForPaging,
			ue.UESecCap(),
			nasMsg,
			pduSessionResourceSetupList,
			supportedGUAMI,
		); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to send initial context setup request", zap.Error(err))
		} else {
			logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request")
		}
	} else {
		if err := ueConn.SendDownlinkNASTransport(ctx, nasMsg); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to send downlink NAS transport", zap.Error(err))
		} else {
			logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")
		}
	}

	if amfInstance.NASGuardCfg.Enable {
		cfg := amfInstance.NASGuardCfg
		conn := ue.Conn()
		conn.armNASGuardWith(cfg, "T3550 (Registration Accept)", func(expireTimes int32) {
			retryUeConn := ue.Conn()
			if retryUeConn == nil {
				logger.From(ctx, logger.AmfLog).Warn("[NAS] UE Context released, abort retransmission of Registration Accept")
			} else {
				if retryUeConn.UeContextRequest && retryUeConn.ICS() != ICSCompleted {
					err = retryUeConn.SendInitialContextSetupRequest(
						context.Background(),
						ue.Ambr.Uplink,
						ue.Ambr.Downlink,
						ue.AllowedNssai,
						ue.Kgnb(),
						ue.RadioCapability,
						ue.RadioCapabilityForPaging,
						ue.UESecCap(),
						nasMsg,
						pduSessionResourceSetupList,
						supportedGUAMI,
					)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Error("could not send initial context setup request", zap.Error(err))
					}

					retryUeConn.MarkICSPending()

					logger.From(ctx, logger.AmfLog).Info("Sent NGAP initial context setup request")
				} else {
					logger.From(ctx, logger.AmfLog).Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))

					err = retryUeConn.SendDownlinkNASTransport(context.Background(), nasMsg)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Error("could not send downlink NAS transport message", zap.Error(err))
					}

					logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")
				}
			}
		}, func() {
			logger.From(ctx, logger.AmfLog).Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))

			ue.TransitionTo(Registered)
			ue.ClearRegistrationRequestData()
		})
	}
}

// ArmRegistrationAcceptGuard supervises with T3550 a GUTI-bearing REGISTRATION
// ACCEPT delivered outside SendRegistrationAccept — embedded in a PDU Session
// Resource Setup Request, or as a plain DL NAS Transport during a mobility/periodic
// registration update. The AMF always reallocates the 5G-GUTI, so every such accept
// carries one and must be supervised (TS 24.501 §5.5.1.3.4). Registration Complete
// stops the timer.
func ArmRegistrationAcceptGuard(amfInstance *AMF, ue *UeContext, nasMsg []byte) {
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

		if err := retryUeConn.SendDownlinkNASTransport(context.Background(), nasMsg); err != nil {
			logger.AmfLog.Error("could not retransmit Registration Accept", zap.Error(err))
		}
	}, func() {
		logger.AmfLog.Warn("T3550 Expires, abort retransmission of Registration Accept", zap.Any("expireTimes", cfg.MaxRetryTimes))
		ue.TransitionTo(Registered)
		ue.ClearRegistrationRequestData()
	})
}

// ResendRegistrationAccept resends the REGISTRATION ACCEPT last sent and restarts
// T3550 without re-authenticating, for a duplicate REGISTRATION REQUEST whose IEs
// match the one being served (TS 24.501 §5.5.1.2.8 case d). Re-arming resets the
// guard, so this retransmission is not charged against the T3550 count. At this
// stage the Initial Context Setup is complete, so the accept rides a plain DL NAS
// Transport.
func ResendRegistrationAccept(ctx context.Context, amfInstance *AMF, ue *UeContext) {
	conn := ue.Conn()
	if conn == nil || len(conn.RegistrationAcceptPdu) == 0 {
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		return
	}

	if err := ueConn.SendDownlinkNASTransport(ctx, conn.RegistrationAcceptPdu); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to resend Registration Accept", zap.Error(err))
	}

	ArmRegistrationAcceptGuard(amfInstance, ue, conn.RegistrationAcceptPdu)
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

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("cannot SendConfigurationUpdateCommand: failed to get operator info", zap.Error(err))
		return
	}

	guti, err := amfInstance.Guti(operatorInfo.Guami, amfUe)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("cannot SendConfigurationUpdateCommand: failed to build 5G-GUTI", zap.Error(err))
		return
	}

	nasMsg, err := BuildConfigurationUpdateCommand(amfInstance, amfUe, guti, operator.SpnFullName, operator.SpnShortName, includeGUTI)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("error building ConfigurationUpdateCommand", zap.Error(err))
		return
	}

	logger.From(ctx, logger.AmfLog).Info("nas/send_configuration_update_command")

	err = ueConn.SendDownlinkNASTransport(ctx, nasMsg)
	if err != nil {
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

			err = retryUeConn.SendDownlinkNASTransport(context.Background(), nasMsg)
			if err != nil {
				logger.From(ctx, logger.AmfLog).Error("could not send configuration update command", zap.Error(err))
			}
		}, func() {
			logger.From(ctx, logger.AmfLog).Warn("timer T3555 expired too many times, aborting configuration update procedure", zap.Int32("maximum retries", cfg.MaxRetryTimes))
		},
		)
	}
}

// SendNGAP writes a complete NGAP PDU to this UE's gNB association (via its conn).
func (ueConn *UeConn) SendNGAP(ctx context.Context, msgType send.NGAPProcedure, pkt []byte) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, msgType, pkt)
}

func (ueConn *UeConn) SendDownlinkNASTransport(ctx context.Context, nasPdu []byte) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildDownlinkNasTransport(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), nasPdu)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedureDownlinkNasTransport, pkt)
}

func (ueConn *UeConn) SendUEContextReleaseCommand(ctx context.Context, causePresent int, cause aper.Enumerated) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	// Idempotent: a release already in flight for this RAN UE (an eNB-initiated release
	// racing a NAS-guard timeout, or two handover-abort paths) must not send a second
	// UE Context Release Command.
	if !amfInstance.claimRelease(ueConn) {
		logger.WithTrace(ctx, ueConn.Log).Debug("UE Context Release already in progress; suppressing duplicate")
		return nil
	}

	pkt, err := send.BuildUEContextReleaseCommand(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), causePresent, cause)
	if err != nil {
		// The command cannot be sent, so no Release Complete will arrive; release
		// locally now to avoid leaking the UeConn and its claim.
		amfInstance.ReleaseUeConn(ctx, ueConn)
		return err
	}

	if err := amfInstance.SendToRan(ctx, conn, send.NGAPProcedureUEContextReleaseCommand, pkt); err != nil {
		amfInstance.ReleaseUeConn(ctx, ueConn)
		return err
	}

	// Supervise the Release Complete: if it is lost, fire once and run the same
	// action-keyed cleanup so the UeConn + AMF-UE-NGAP-ID cannot leak (TS 38.413 §8.3
	// mandates no CN-side timer, so this is a local robustness guard).
	ueConn.releaseGuard.Arm(releaseGuardTimeout, 0, nil, func() {
		amfInstance.ReleaseUeConn(context.Background(), ueConn)
	})

	return nil
}

func (ueConn *UeConn) SendPDUSessionResourceSetupRequest(ctx context.Context, ambrUp string, ambrDown string, nasPdu []byte, list ngapType.PDUSessionResourceSetupListSUReq) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildPDUSessionResourceSetupRequest(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), ambrUp, ambrDown, nasPdu, list)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedurePDUSessionResourceSetupRequest, pkt)
}

func (ueConn *UeConn) SendPDUSessionResourceReleaseCommand(ctx context.Context, nasPdu []byte, list ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildPDUSessionResourceReleaseCommand(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), nasPdu, list)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedurePDUSessionResourceReleaseCommand, pkt)
}

func (ueConn *UeConn) SendInitialContextSetupRequest(
	ctx context.Context,
	ambrUp string,
	ambrDown string,
	allowedNssai []models.Snssai,
	kgnb []byte,
	ueRadioCapability []byte,
	ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging,
	ueSecurityCapability *nasType.UESecurityCapability,
	nasPdu []byte,
	pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq,
	supportedGUAMI *models.Guami,
) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildInitialContextSetupRequest(
		int64(ueConn.AmfUeNgapID),
		int64(ueConn.RanUeNgapID),
		ambrUp,
		ambrDown,
		allowedNssai,
		kgnb,
		ueRadioCapability,
		ueRadioCapabilityForPaging,
		ueSecurityCapability,
		nasPdu,
		pduSessionResourceSetupRequestList,
		supportedGUAMI,
	)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedureInitialContextSetupRequest, pkt)
}

func (ueConn *UeConn) SendPDUSessionResourceModifyConfirm(
	ctx context.Context,
	pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm,
	pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm,
) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildPDUSessionResourceModifyConfirm(
		int64(ueConn.AmfUeNgapID),
		int64(ueConn.RanUeNgapID),
		pduSessionResourceModifyConfirmList,
		pduSessionResourceFailedToModifyList,
	)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedurePDUSessionResourceModifyConfirm, pkt)
}

func (ueConn *UeConn) SendPDUSessionResourceModifyRequest(
	ctx context.Context,
	pduSessionResourceModifyList ngapType.PDUSessionResourceModifyListModReq,
) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildPDUSessionResourceModifyRequest(
		int64(ueConn.AmfUeNgapID),
		int64(ueConn.RanUeNgapID),
		pduSessionResourceModifyList,
	)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedurePDUSessionResourceModifyRequest, pkt)
}

func (ueConn *UeConn) SendHandoverPreparationFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildHandoverPreparationFailure(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), cause, criticalityDiagnostics)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedureHandoverPreparationFailure, pkt)
}

func (ueConn *UeConn) SendHandoverCancelAcknowledge(ctx context.Context) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildHandoverCancelAcknowledge(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID))
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedureHandoverCancelAcknowledge, pkt)
}

func (ueConn *UeConn) SendHandoverRequest(
	ctx context.Context,
	handOverType ngapType.HandoverType,
	uplinkAmbr string,
	downlinkAmbr string,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	snssaiList []models.Snssai,
	supportedGUAMI *models.Guami,
) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := send.BuildHandoverRequest(
		int64(ueConn.AmfUeNgapID),
		handOverType,
		uplinkAmbr,
		downlinkAmbr,
		ueSecurityCapability,
		ncc,
		nh,
		cause,
		pduSessionResourceSetupListHOReq,
		sourceToTargetTransparentContainer,
		snssaiList,
		supportedGUAMI,
	)
	if err != nil {
		return err
	}

	return amfInstance.SendToRan(ctx, conn, send.NGAPProcedureHandoverRequest, pkt)
}
