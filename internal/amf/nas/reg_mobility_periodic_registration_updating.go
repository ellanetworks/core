// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleMobilityAndPeriodicRegistrationUpdating(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) {
	logger.From(ctx, logger.AmfLog).Debug("Handle MobilityAndPeriodicRegistrationUpdating")

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	err := ue.DeriveAnKey()
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "derive AnKey", err)
		return
	}

	if conn.RegistrationRequest.UpdateType5GS != nil {
		if conn.RegistrationRequest.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.RadioCapability = nil
			ue.RadioCapabilityForPaging = nil
		}
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "get operator info", err)
		return
	}

	subscriberProfile, err := amfInstance.SubscriberProfile(ctx, ue.Supi())
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "get subscriber profile", err)
		return
	}

	if len(subscriberProfile.AllowedNssai) == 0 {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, nasMessage.Cause5GMM5GSServicesNotAllowed)
		ue.Deregister(ctx)

		return
	}

	ue.AllowedNssai = subscriberProfile.AllowedNssai

	if conn.RegistrationRequest.MICOIndication != nil {
		logger.From(ctx, logger.AmfLog).Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", conn.RegistrationRequest.GetRAAI()))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.GetDRXValue()
		if drx > nasMessage.DRXcycleParameterT256 {
			logger.From(ctx, logger.AmfLog).Warn("UE requested reserved DRX value, treating as not specified", zap.Uint8("drxValue", drx))
			drx = nasMessage.DRXValueNotSpecified
		}

		ue.DRXParameter = drx
	}

	if !ue.Imei.IsSet() {
		logger.From(ctx, logger.AmfLog).Debug("The UE did not provide PEI")

		amf.SendIdentityRequest(ctx, amfInstance, ueConn, nasMessage.MobileIdentity5GSTypeImei)

		logger.From(ctx, logger.AmfLog).Info("sent identity request to UE")

		return
	}

	ue.Ambr = subscriberProfile.Ambr

	var (
		reactivationResult        *[16]bool
		errPduSessionID, errCause []uint8
	)

	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}
	suList := ngapType.PDUSessionResourceSetupListSUReq{}

	if conn.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(conn.RegistrationRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)

		for idx, hasUplinkData := range uplinkDataPsi {
			pduSessionID := uint8(idx)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if hasUplinkData {
					binaryDataN2SmInformation, err := amfInstance.Session.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else {
						if ueConn.UeContextRequest {
							send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionID,
								smContext.Snssai, nil, binaryDataN2SmInformation)
						} else {
							send.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionID,
								smContext.Snssai, nil, binaryDataN2SmInformation)
						}
					}
				}
			}
		}
	}

	var pduSessionStatus *[16]bool
	if conn.RegistrationRequest.PDUSessionStatus != nil {
		pduSessionStatus = new([16]bool)
		psiArray := nasConvert.PSIToBooleanArray(conn.RegistrationRequest.PDUSessionStatus.Buffer)

		for psi := 1; psi <= 15; psi++ {
			pduSessionID := uint8(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
					err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("failed to release sm context", zap.Error(err))
						return
					} else {
						pduSessionStatus[psi] = false
					}
				} else {
					pduSessionStatus[psi] = true
				}
			}
		}
	}

	err = amfInstance.ReallocateGUTI(ctx, ue)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "reallocate GUTI", err)
		return
	}

	guti, err := amfInstance.Guti(operatorInfo.Guami, ue)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "build 5G-GUTI", err)
		return
	}

	if conn.RegistrationRequest.AllowedPDUSessionStatus != nil {
		if requestData := conn.N1N2Message(); requestData != nil {
			n1Msg := requestData.BinaryDataN1Message
			n2Info := requestData.BinaryDataN2Information

			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := amf.BuildRegistrationAccept(amfInstance, ue, guti, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("failed to build registration accept", zap.Error(err))
						return
					}

					metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

					err = ueConn.SendPDUSessionResourceSetupRequest(
						ctx,
						ue.Ambr.Uplink,
						ue.Ambr.Downlink,
						nasPdu,
						suList,
					)
					if err != nil {
						abortRegistration(ctx, amfInstance, ue, "send PDU session resource setup request", err)
						return
					}

					amf.ArmRegistrationAcceptGuard(amfInstance, ue, nasPdu)

					logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request")
				} else {
					metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

					amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

					logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")
				}

				amf.SendDLNASTransport(ctx, ueConn, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)

				conn.ClearN1N2Message()

				return
			}

			_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				conn.ClearN1N2Message()
				// UE referenced a PDU session id it holds no context for; release the
				// half-updated registration rather than leak it.
				abortRegistration(ctx, amfInstance, ue, "UE referenced unknown PDU session id", nil)

				return
			}

			var (
				nasPdu []byte
				err    error
			)

			if n1Msg != nil {
				nasPdu, err = amf.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
				if err != nil {
					logger.From(ctx, logger.AmfLog).Warn("failed to build DL NAS transport", zap.Error(err))
					return
				}
			}

			send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
		}
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	if ueConn.UeContextRequest {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

		amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

		logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")

		return
	} else {
		nasPdu, err := amf.BuildRegistrationAccept(amfInstance, ue, guti, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "build registration accept", err)
			return
		}

		if len(suList.List) != 0 {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

			err := ueConn.SendPDUSessionResourceSetupRequest(
				ctx,
				ue.Ambr.Uplink,
				ue.Ambr.Downlink,
				nasPdu,
				suList,
			)
			if err != nil {
				abortRegistration(ctx, amfInstance, ue, "send PDU session resource setup request", err)
				return
			}

			amf.ArmRegistrationAcceptGuard(amfInstance, ue, nasPdu)

			logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request")
		} else {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

			err := ueConn.SendDownlinkNASTransport(ctx, nasPdu, nil)
			if err != nil {
				abortRegistration(ctx, amfInstance, ue, "send downlink NAS transport", err)
				return
			}

			amf.ArmRegistrationAcceptGuard(amfInstance, ue, nasPdu)

			logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport message")
		}
	}
}
