// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleMobilityAndPeriodicRegistrationUpdating(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext) error {
	ue.Log.Debug("Handle MobilityAndPeriodicRegistrationUpdating")

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	err := ue.DerivateAnKey()
	if err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	if conn.RegistrationRequest.UpdateType5GS != nil {
		if conn.RegistrationRequest.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.UeRadioCapability = nil
			ue.UeRadioCapabilityForPaging = nil
		}
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	subscriberProfile, err := amfInstance.GetSubscriberProfile(ctx, ue.SupiValue())
	if err != nil {
		return fmt.Errorf("error getting subscriber profile: %v", err)
	}

	if len(subscriberProfile.AllowedNssai) == 0 {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMM5GSServicesNotAllowed)

		return fmt.Errorf("registration Reject [No allowed S-NSSAI in subscription]")
	}

	ue.AllowedNssai = subscriberProfile.AllowedNssai

	// The 5GMM capability IE is optional (TS 24.501), re-sent only
	// when it changes; a missing optional IE is treated as not present,
	// not an error.

	if conn.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", conn.RegistrationRequest.GetRAAI()))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.GetDRXValue()
		if drx > nasMessage.DRXcycleParameterT256 {
			ue.Log.Warn("UE requested reserved DRX value, treating as not specified", zap.Uint8("drxValue", drx))
			drx = nasMessage.DRXValueNotSpecified
		}

		ue.UESpecificDRX = drx
	}

	if len(ue.Pei) == 0 {
		ue.Log.Debug("The UE did not provide PEI")

		amf.SendIdentityRequest(ctx, ranUe, nasMessage.MobileIdentity5GSTypeImei)

		ue.Log.Info("sent identity request to UE")

		return nil
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
					binaryDataN2SmInformation, err := amfInstance.Smf.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						ue.Log.Warn("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else {
						if ranUe.UeContextRequest {
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
					err := amfInstance.Smf.ReleaseSmContext(ctx, smContext.Ref)
					if err != nil {
						return fmt.Errorf("failed to release sm context: %s", err)
					} else {
						pduSessionStatus[psi] = false
					}
				} else {
					pduSessionStatus[psi] = true
				}
			}
		}
	}

	err = amfInstance.ReAllocateGuti(ctx, ue, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	if conn.RegistrationRequest.AllowedPDUSessionStatus != nil {
		if conn.N1N2Message != nil {
			requestData := conn.N1N2Message
			n1Msg := conn.N1N2Message.BinaryDataN1Message
			n2Info := conn.N1N2Message.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := amf.BuildRegistrationAccept(amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
					if err != nil {
						return err
					}

					metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

					err = ranUe.SendPDUSessionResourceSetupRequest(
						ctx,
						ue.Ambr.Uplink,
						ue.Ambr.Downlink,
						nasPdu,
						suList,
					)
					if err != nil {
						return fmt.Errorf("error sending pdu session resource setup request: %v", err)
					}

					ue.Log.Info("Sent NGAP pdu session resource setup request")
				} else {
					metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

					amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

					ue.Log.Info("Sent GMM registration accept")
				}

				amf.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)

				conn.N1N2Message = nil

				return nil
			}

			_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				conn.N1N2Message = nil
				return fmt.Errorf("pdu Session Id does not Exists")
			}

			var (
				nasPdu []byte
				err    error
			)

			if n1Msg != nil {
				nasPdu, err = amf.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
				if err != nil {
					return err
				}
			}

			send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
		}
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	if ranUe.UeContextRequest {
		metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

		amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

		ue.Log.Info("Sent GMM registration accept")

		return nil
	} else {
		nasPdu, err := amf.BuildRegistrationAccept(amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
		if err != nil {
			return fmt.Errorf("error building registration accept: %v", err)
		}

		if len(suList.List) != 0 {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

			err := ranUe.SendPDUSessionResourceSetupRequest(
				ctx,
				ue.Ambr.Uplink,
				ue.Ambr.Downlink,
				nasPdu,
				suList,
			)
			if err != nil {
				return fmt.Errorf("error sending pdu session resource setup request: %v", err)
			}

			ue.Log.Info("Sent NGAP pdu session resource setup request")
		} else {
			metrics.RegistrationAttempt(metrics.RAT5G, getRegistrationType5GSName(conn.RegistrationType5GS), metrics.ResultAccept)

			err := ranUe.SendDownlinkNasTransport(ctx, nasPdu, nil)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}

			ue.Log.Info("sent downlink nas transport message")
		}

		return nil
	}
}
