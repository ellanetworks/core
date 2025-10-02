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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

func SendDLNASTransport(ue *context.RanUe, payloadContainerType uint8, nasPdu []byte, pduSessionID int32, cause uint8) error {
	var causePtr *uint8
	if cause != 0 {
		causePtr = &cause
	}

	nasMsg, err := BuildDLNASTransport(ue.AmfUe, payloadContainerType, nasPdu, uint8(pduSessionID), causePtr)
	if err != nil {
		return fmt.Errorf("error building downlink NAS transport message: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberDownlinkNasTransport,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.Int32("pduSessionID", pduSessionID),
		zap.String("cause", nasMessage.Cause5GMMToString(cause)),
	)

	return nil
}

func SendNotification(ue *context.RanUe, nasMsg []byte) error {
	amfUe := ue.AmfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if context.AMFSelf().T3565Cfg.Enable {
		cfg := context.AMFSelf().T3565Cfg
		amfUe.T3565 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3565 expires, retransmit Notification", zap.Any("expireTimes", expireTimes))
			err := ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
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

	logger.LogSubscriberEvent(
		logger.SubscriberNotification,
		logger.DirectionOutbound,
		nasMsg,
		amfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", amfUe.Suci),
		zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
	)

	return nil
}

func SendIdentityRequest(ue *context.RanUe, typeOfIdentity uint8) error {
	nasMsg, err := BuildIdentityRequest(typeOfIdentity)
	if err != nil {
		return fmt.Errorf("error building identity request: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberIdentityRequest,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

	return nil
}

func SendAuthenticationRequest(ue *context.RanUe) error {
	amfUe := ue.AmfUe
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if amfUe.AuthenticationCtx == nil {
		return fmt.Errorf("authentication context of UE is nil")
	}

	nasMsg, err := BuildAuthenticationRequest(amfUe)
	if err != nil {
		return fmt.Errorf("error building authentication request: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	amfUe.GmmLog.Info("Sent GMM downlink nas transport message to UE")

	if context.AMFSelf().T3560Cfg.Enable {
		cfg := context.AMFSelf().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3560 expires, retransmit Authentication Request", zap.Any("expireTimes", expireTimes))
			err := ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
				return
			}
			amfUe.GmmLog.Info("Sent GMM downlink nas transport message to UE")
		}, func() {
			amfUe.GmmLog.Warn("T3560 Expires, abort authentication procedure & ongoing 5GMM procedure", zap.Any("expireTimes", cfg.MaxRetryTimes))
			amfUe.Remove()
		})
	}

	logger.LogSubscriberEvent(
		logger.SubscriberAuthenticationRequest,
		logger.DirectionOutbound,
		nasMsg,
		amfUe.Supi,
		zap.String("suci", amfUe.Suci),
		zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
	)

	return nil
}

func SendServiceAccept(ue *context.RanUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool, errPduSessionID, errCause []uint8) error {
	nasMsg, err := BuildServiceAccept(ue.AmfUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building service accept: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberServiceAccept,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

	return nil
}

func SendAuthenticationReject(ue *context.RanUe, eapMsg string) error {
	nasMsg, err := BuildAuthenticationReject(ue.AmfUe, eapMsg)
	if err != nil {
		return fmt.Errorf("error building authentication reject: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberAuthenticationReject,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

	return nil
}

func SendAuthenticationResult(ue *context.RanUe, eapSuccess bool, eapMsg string) error {
	if ue.AmfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	nasMsg, err := BuildAuthenticationResult(ue.AmfUe, eapSuccess, eapMsg)
	if err != nil {
		return fmt.Errorf("error building authentication result: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberAuthenticationResult,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

	return nil
}

func SendServiceReject(ue *context.RanUe, pDUSessionStatus *[16]bool, cause uint8) error {
	nasMsg, err := BuildServiceReject(pDUSessionStatus, cause)
	if err != nil {
		return fmt.Errorf("error building service reject: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberServiceReject,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
		zap.String("cause", nasMessage.Cause5GMMToString(cause)),
	)

	return nil
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
// eapMessage: if the REGISTRATION REJECT message is used to convey EAP-failure message
func SendRegistrationReject(ue *context.RanUe, cause5GMM uint8, eapMessage string) error {
	nasMsg, err := BuildRegistrationReject(ue.AmfUe, cause5GMM, eapMessage)
	if err != nil {
		return fmt.Errorf("error building registration reject: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberRegistrationReject,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
		zap.String("cause", nasMessage.Cause5GMMToString(cause5GMM)),
	)

	return nil
}

// eapSuccess: only used when authType is EAP-AKA', set the value to false if authType is not EAP-AKA'
// eapMessage: only used when authType is EAP-AKA', set the value to "" if authType is not EAP-AKA'
func SendSecurityModeCommand(ue *context.RanUe, eapSuccess bool, eapMessage string) error {
	nasMsg, err := BuildSecurityModeCommand(ue.AmfUe, eapSuccess, eapMessage)
	if err != nil {
		return fmt.Errorf("error building security mode command: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	amfUe := ue.AmfUe

	if context.AMFSelf().T3560Cfg.Enable {
		cfg := context.AMFSelf().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3560 expires, retransmit Security Mode Command", zap.Any("expireTimes", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
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

	logger.LogSubscriberEvent(
		logger.SubscriberSecurityModeCommand,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

	return nil
}

func SendDeregistrationRequest(ue *context.RanUe, accessType uint8, reRegistrationRequired bool, cause5GMM uint8) error {
	ue.AmfUe.DeregistrationTargetAccessType = accessType

	nasMsg, err := BuildDeregistrationRequest(ue, accessType, reRegistrationRequired, cause5GMM)
	if err != nil {
		return fmt.Errorf("error building deregistration request: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}
	ue.AmfUe.GmmLog.Info("sent deregistration request")

	amfUe := ue.AmfUe

	if context.AMFSelf().T3522Cfg.Enable {
		cfg := context.AMFSelf().T3522Cfg
		amfUe.T3522 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warn("T3522 expires, retransmit Deregistration Request", zap.Any("expireTimes", expireTimes))
			err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
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

	logger.LogSubscriberEvent(
		logger.SubscriberDeregistrationRequest,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

	return nil
}

func SendDeregistrationAccept(ue *context.RanUe) error {
	nasMsg, err := BuildDeregistrationAccept()
	if err != nil {
		return fmt.Errorf("error building deregistration accept: %s", err.Error())
	}

	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		ue.AmfUe.GmmLog.Error("could not send downlink NAS transport message", zap.Error(err))
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}

	logger.LogSubscriberEvent(
		logger.SubscriberDeregistrationAccept,
		logger.DirectionOutbound,
		nasMsg,
		ue.AmfUe.Supi,
		zap.String("ran", ue.Ran.Name),
		zap.String("suci", ue.AmfUe.Suci),
		zap.String("plmnID", ue.AmfUe.PlmnID.Mcc+ue.AmfUe.PlmnID.Mnc),
	)

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
) error {
	nasMsg, err := BuildRegistrationAccept(ctx, ue, anType, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building registration accept: %s", err.Error())
	}

	if ue.RanUe[anType].UeContextRequest {
		err = ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %s", err.Error())
		}
		ue.GmmLog.Info("Sent NGAP initial context setup request")
	} else {
		err = ngap_message.SendDownlinkNasTransport(ue.RanUe[models.AccessType3GPPAccess], nasMsg, nil)
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
					err = ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil)
					if err != nil {
						ue.GmmLog.Error("could not send initial context setup request", zap.Error(err))
					}
					ue.GmmLog.Info("Sent NGAP initial context setup request")
				} else {
					ue.GmmLog.Warn("T3550 expires, retransmit Registration Accept", zap.Any("expireTimes", expireTimes))
					err = ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasMsg, nil)
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

	logger.LogSubscriberEvent(
		logger.SubscriberRegistrationAccept,
		logger.DirectionOutbound,
		nasMsg,
		ue.Supi,
		zap.String("ran", ue.RanUe[anType].Ran.Name),
		zap.String("suci", ue.Suci),
		zap.String("plmnID", ue.PlmnID.Mcc+ue.PlmnID.Mnc),
	)

	return nil
}
