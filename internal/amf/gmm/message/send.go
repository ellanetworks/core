// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package message

import (
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/ngap/ngapType"
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
			amfUe.GmmLog.Warnf("T3565 expires, retransmit Notification (retry: %d)", expireTimes)
			err := ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Errorf("could not send notification: %s", err.Error())
				return
			}
			amfUe.GmmLog.Infof("sent notification")
		}, func() {
			amfUe.GmmLog.Warnf("T3565 Expires %d times, abort notification procedure", cfg.MaxRetryTimes)
			amfUe.T3565 = nil // clear the timer
		})
	}

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
	amfUe.GmmLog.Infof("Sent downlink nas transport message to UE")

	if context.AMFSelf().T3560Cfg.Enable {
		cfg := context.AMFSelf().T3560Cfg
		amfUe.T3560 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warnf("T3560 expires, retransmit Authentication Request (retry: %d)", expireTimes)
			err := ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Errorf("could not send downlink NAS transport message: %s", err.Error())
				return
			}
			amfUe.GmmLog.Infof("Sent downlink nas transport message to UE")
		}, func() {
			amfUe.GmmLog.Warnf("T3560 Expires %d times, abort authentication procedure & ongoing 5GMM procedure",
				cfg.MaxRetryTimes)
			amfUe.Remove()
		})
	}

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
			amfUe.GmmLog.Warnf("T3560 expires, retransmit Security Mode Command (retry: %d)", expireTimes)
			err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Errorf("could not send downlink NAS transport message: %s", err.Error())
				return
			}
			amfUe.GmmLog.Infof("sent security mode command")
		}, func() {
			amfUe.GmmLog.Warnf("T3560 Expires %d times, abort security mode control procedure", cfg.MaxRetryTimes)
			amfUe.Remove()
		})
	}
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
	ue.AmfUe.GmmLog.Infof("sent deregistration request")

	amfUe := ue.AmfUe

	if context.AMFSelf().T3522Cfg.Enable {
		cfg := context.AMFSelf().T3522Cfg
		amfUe.T3522 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			amfUe.GmmLog.Warnf("T3522 expires, retransmit Deregistration Request (retry: %d)", expireTimes)
			err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
			if err != nil {
				amfUe.GmmLog.Errorf("could not send downlink NAS transport message: %s", err.Error())
				return
			}
			amfUe.GmmLog.Infof("sent deregistration request")
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
	return nil
}

func SendDeregistrationAccept(ue *context.RanUe) error {
	nasMsg, err := BuildDeregistrationAccept()
	if err != nil {
		return fmt.Errorf("error building deregistration accept: %s", err.Error())
	}
	err = ngap_message.SendDownlinkNasTransport(ue, nasMsg, nil)
	if err != nil {
		ue.AmfUe.GmmLog.Errorf("could not send downlink NAS transport message: %s", err.Error())
		return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
	}
	return nil
}

func SendRegistrationAccept(
	ue *context.AmfUe,
	anType models.AccessType,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID, errCause []uint8,
	pduSessionResourceSetupList *ngapType.PDUSessionResourceSetupListCxtReq,
) error {
	nasMsg, err := BuildRegistrationAccept(ue, anType, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building registration accept: %s", err.Error())
	}

	if ue.RanUe[anType].UeContextRequest {
		err = ngap_message.SendInitialContextSetupRequest(ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %s", err.Error())
		}
		ue.GmmLog.Infof("sent initial context setup request")
	} else {
		err = ngap_message.SendDownlinkNasTransport(ue.RanUe[models.AccessType3GPPAccess], nasMsg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink NAS transport message: %s", err.Error())
		}
		ue.GmmLog.Infof("sent registration accept")
	}

	if context.AMFSelf().T3550Cfg.Enable {
		cfg := context.AMFSelf().T3550Cfg
		ue.T3550 = context.NewTimer(cfg.ExpireTime, cfg.MaxRetryTimes, func(expireTimes int32) {
			if ue.RanUe[anType] == nil {
				ue.GmmLog.Warnf("[NAS] UE Context released, abort retransmission of Registration Accept")
				ue.T3550 = nil
			} else {
				if ue.RanUe[anType].UeContextRequest && !ue.RanUe[anType].RecvdInitialContextSetupResponse {
					err = ngap_message.SendInitialContextSetupRequest(ue, anType, nasMsg, pduSessionResourceSetupList, nil, nil, nil)
					if err != nil {
						ue.GmmLog.Errorf("could not send initial context setup request: %s", err.Error())
					}
					ue.GmmLog.Infof("sent initial context setup request")
				} else {
					ue.GmmLog.Warnf("T3550 expires, retransmit Registration Accept (retry: %d)", expireTimes)
					err = ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasMsg, nil)
					if err != nil {
						ue.GmmLog.Errorf("could not send downlink NAS transport message: %s", err.Error())
					}
					ue.GmmLog.Infof("sent registration accept")
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
	return nil
}
