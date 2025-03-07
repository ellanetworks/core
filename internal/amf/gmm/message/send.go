// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/ngap/ngapType"
)

// backOffTimerUint = 7 means backoffTimer is null
func SendDLNASTransport(ue *context.RanUe, payloadContainerType uint8, nasPdu []byte,
	pduSessionId int32, cause uint8, backOffTimerUint *uint8, backOffTimer uint8,
) {
	ue.AmfUe.GmmLog.Info("Send DL NAS Transport")
	var causePtr *uint8
	if cause != 0 {
		causePtr = &cause
	}
	nasMsg, err := BuildDLNASTransport(ue.AmfUe, payloadContainerType, nasPdu,
		uint8(pduSessionId), causePtr, backOffTimerUint, backOffTimer)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

func SendNotification(ue *context.RanUe, nasMsg []byte) {
	ue.AmfUe.GmmLog.Info("Send Notification")

	amfUe := ue.AmfUe
	if amfUe == nil {
		ue.AmfUe.GmmLog.Error("AmfUe is nil")
		return
	}

	if context.AMF_Self().T3565Cfg.Enable {
		cfg := context.AMF_Self().T3565Cfg
		amfUe.T3565 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warnf("T3565 expires, retransmit Notification (retry: %d)", expireTimes)
			ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
		}, func() {
			amfUe.GmmLog.Warnf("T3565 Expires %d times, abort notification procedure", cfg.MaxRetryTimes)
			amfUe.T3565 = nil // clear the timer
		})
	}
}

func SendIdentityRequest(ue *context.RanUe, typeOfIdentity uint8) {
	ue.AmfUe.GmmLog.Info("Send Identity Request")

	nasMsg, err := BuildIdentityRequest(typeOfIdentity)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

func SendAuthenticationRequest(ue *context.RanUe) {
	amfUe := ue.AmfUe
	if amfUe == nil {
		logger.AmfLog.Error("AmfUe is nil")
		return
	}

	amfUe.GmmLog.Infof("Send Authentication Request")

	if amfUe.AuthenticationCtx == nil {
		amfUe.GmmLog.Error("Authentication Context of UE is nil")
		return
	}

	nasMsg, err := BuildAuthenticationRequest(amfUe)
	if err != nil {
		amfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)

	if context.AMF_Self().T3560Cfg.Enable {
		cfg := context.AMF_Self().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warnf("T3560 expires, retransmit Authentication Request (retry: %d)", expireTimes)
			ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
		}, func() {
			amfUe.GmmLog.Warnf("T3560 Expires %d times, abort authentication procedure & ongoing 5GMM procedure",
				cfg.MaxRetryTimes)
			amfUe.Remove()
		})
	}
}

func SendServiceAccept(ue *context.RanUe, pDUSessionStatus *[16]bool, reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
) {
	ue.AmfUe.GmmLog.Info("Send Service Accept")

	nasMsg, err := BuildServiceAccept(ue.AmfUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

func SendAuthenticationReject(ue *context.RanUe, eapMsg string) {
	ue.AmfUe.GmmLog.Info("Send Authentication Reject")

	nasMsg, err := BuildAuthenticationReject(ue.AmfUe, eapMsg)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

func SendAuthenticationResult(ue *context.RanUe, eapSuccess bool, eapMsg string) {
	if ue.AmfUe == nil {
		logger.AmfLog.Errorf("AmfUe is nil")
		return
	}

	ue.AmfUe.GmmLog.Info("Send Authentication Result")

	nasMsg, err := BuildAuthenticationResult(ue.AmfUe, eapSuccess, eapMsg)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

func SendServiceReject(ue *context.RanUe, pDUSessionStatus *[16]bool, cause uint8) {
	ue.AmfUe.GmmLog.Info("Send Service Reject")

	nasMsg, err := BuildServiceReject(pDUSessionStatus, cause)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

// T3502: This IE may be included to indicate a value for timer T3502 during the initial registration
// eapMessage: if the REGISTRATION REJECT message is used to convey EAP-failure message
func SendRegistrationReject(ue *context.RanUe, cause5GMM uint8, eapMessage string) {
	ue.AmfUe.GmmLog.Info("Send Registration Reject")

	nasMsg, err := BuildRegistrationReject(ue.AmfUe, cause5GMM, eapMessage)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

// eapSuccess: only used when authType is EAP-AKA', set the value to false if authType is not EAP-AKA'
// eapMessage: only used when authType is EAP-AKA', set the value to "" if authType is not EAP-AKA'
func SendSecurityModeCommand(ue *context.RanUe, eapSuccess bool, eapMessage string) {
	ue.AmfUe.GmmLog.Info("Send Security Mode Command")

	nasMsg, err := BuildSecurityModeCommand(ue.AmfUe, eapSuccess, eapMessage)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)

	amfUe := ue.AmfUe

	if context.AMF_Self().T3560Cfg.Enable {
		cfg := context.AMF_Self().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warnf("T3560 expires, retransmit Security Mode Command (retry: %d)", expireTimes)
			ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
		}, func() {
			amfUe.GmmLog.Warnf("T3560 Expires %d times, abort security mode control procedure", cfg.MaxRetryTimes)
			amfUe.Remove()
		})
	}
}

func SendDeregistrationRequest(ue *context.RanUe, accessType uint8, reRegistrationRequired bool, cause5GMM uint8) {
	ue.AmfUe.GmmLog.Info("Send Deregistration Request")

	// setting accesstype
	ue.AmfUe.DeregistrationTargetAccessType = accessType

	nasMsg, err := BuildDeregistrationRequest(ue, accessType, reRegistrationRequired, cause5GMM)
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)

	amfUe := ue.AmfUe

	if context.AMF_Self().T3522Cfg.Enable {
		cfg := context.AMF_Self().T3522Cfg
		amfUe.T3522 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warnf("T3522 expires, retransmit Deregistration Request (retry: %d)", expireTimes)
			ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
		}, func() {
			amfUe.GmmLog.Warnf("T3522 Expires %d times, abort deregistration procedure", cfg.MaxRetryTimes)
			amfUe.T3522 = nil // clear the timer
			if accessType == nasMessage.AccessType3GPP {
				amfUe.GmmLog.Warnln("UE accessType[3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessType3GPPAccess].Set(context.Deregistered)
				amfUe.Remove()
			} else if accessType == nasMessage.AccessTypeNon3GPP {
				amfUe.GmmLog.Warnln("UE accessType[Non3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessTypeNon3GPPAccess].Set(context.Deregistered)
				amfUe.Remove()
			} else {
				amfUe.GmmLog.Warnln("UE accessType[3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessType3GPPAccess].Set(context.Deregistered)
				amfUe.GmmLog.Warnln("UE accessType[Non3GPP] transfer to Deregistered state")
				amfUe.State[models.AccessTypeNon3GPPAccess].Set(context.Deregistered)
				amfUe.Remove()
			}
		})
	}
}

func SendDeregistrationAccept(ue *context.RanUe) {
	ue.AmfUe.GmmLog.Info("Send Deregistration Accept")

	nasMsg, err := BuildDeregistrationAccept()
	if err != nil {
		ue.AmfUe.GmmLog.Error(err.Error())
		return
	}
	ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
}

func SendRegistrationAccept(
	ue *context.AmfUe,
	anType models.AccessType,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	pduSessionResourceSetupList *ngapType.PDUSessionResourceSetupListCxtReq,
) {
	nasMsg, err := BuildRegistrationAccept(ue, anType, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		ue.GmmLog.Error(err.Error())
		return
	}

	if ue.RanUe[anType].UeContextRequest {
		ngap_message.SendInitialContextSetupRequest(ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil)
	} else {
		ngap_message.SendDownlinkNasTransport(ue.RanUe[models.AccessType3GPPAccess], nasMsg, nil)
	}

	if context.AMF_Self().T3550Cfg.Enable {
		cfg := context.AMF_Self().T3550Cfg
		ue.T3550 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			if ue.RanUe[anType] == nil {
				ue.GmmLog.Warnf("[NAS] UE Context released, abort retransmission of Registration Accept")
				ue.T3550 = nil
			} else {
				if ue.RanUe[anType].UeContextRequest && !ue.RanUe[anType].RecvdInitialContextSetupResponse {
					ngap_message.SendInitialContextSetupRequest(ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil)
				} else {
					ue.GmmLog.Warnf("T3550 expires, retransmit Registration Accept (retry: %d)", expireTimes)
					ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasMsg, nil)
				}
			}
		}, func() {
			ue.GmmLog.Warnf("T3550 Expires %d times, abort retransmission of Registration Accept", cfg.MaxRetryTimes)
			ue.T3550 = nil // clear the timer
			// TS 24.501 5.5.1.2.8 case c, 5.5.1.3.8 case c
			ue.State[anType].Set(context.Registered)
			ue.ClearRegistrationRequestData(anType)
		})
	}
}
