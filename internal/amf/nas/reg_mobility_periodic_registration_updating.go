// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
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

	// A fresh K_gNB and the {NH, NCC} anchored on it are one derivation
	// (TS 33.501 §6.9.2.1.1). The NAS SMC that would otherwise re-derive both is
	// skipped whenever the UE holds a valid security context — the normal case
	// here — so deriving the key alone leaves every later handover handing the
	// target an {NH, NCC} the UE cannot reproduce (§6.9.2.3.4).
	err := ue.UpdateSecurityContext()
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "update security context", err)
		return
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
		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseServicesNotAllowed)
		ue.Deregister(ctx)

		return
	}

	ue.AllowedNssai = subscriberProfile.AllowedNssai

	if conn.RegistrationRequest.MICOIndication != nil {
		logger.From(ctx, logger.AmfLog).Warn("Receive MICO Indication Not Supported", zap.Bool("RAAI", conn.RegistrationRequest.MICOIndication.RAAI))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.RequestedDRXParameters.Value
		if drx > fgs.DRXCycleParameterT256 {
			logger.From(ctx, logger.AmfLog).Warn("UE requested reserved DRX value, treating as not specified", zap.Stringer("drxValue", drx))
			drx = fgs.DRXValueNotSpecified
		}

		ue.DRXParameter = drx
	}

	ue.Ambr = subscriberProfile.Ambr

	var (
		reactivationResult        *[16]bool
		errPduSessionID, errCause []uint8
	)

	var (
		ctxList ngap.PDUSessionResourceSetupListCxtReq
		suList  ngap.PDUSessionResourceSetupListSUReq
	)

	if conn.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := conn.RegistrationRequest.UplinkDataStatus.PSI
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
						cause := fgs.GMMCauseProtocolErrorUnspecified
						errCause = append(errCause, uint8(cause))
					} else {
						ue.SetSmContextActive(pduSessionID)

						if ueConn.UeContextRequest {
							item, err := amf.PDUSessionSetupItem(pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
							if err != nil {
								logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", pduSessionID))
							} else {
								ctxList = append(ctxList, item)
							}
						} else {
							item, err := amf.PDUSessionSetupItemSUReq(pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
							if err != nil {
								logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", pduSessionID))
							} else {
								suList = append(suList, item)
							}
						}
					}
				}
			}
		}
	}

	var pduSessionStatus *[16]bool
	if conn.RegistrationRequest.PDUSessionStatus != nil {
		pduSessionStatus = new([16]bool)

		psiArray := conn.RegistrationRequest.PDUSessionStatus.PSI

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
		if requestData := ue.N1N2Message(); requestData != nil {
			n1Msg := requestData.BinaryDataN1Message
			n2Info := requestData.BinaryDataN2Information

			if n2Info == nil {
				if len(suList) != 0 {
					nasPdu, err := amf.BuildRegistrationAccept(amfInstance, ue, guti, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("failed to build registration accept", zap.Error(err))
						return
					}

					metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

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
					metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

					amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

					logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")
				}

				amf.SendDLNASTransport(ctx, ueConn, fgs.PayloadContainerTypeN1SMInfo, n1Msg, fgs.PDUSessionID(requestData.PduSessionID), 0)

				ue.ClearN1N2Message()

				return
			}

			_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				ue.ClearN1N2Message()
				// UE referenced a PDU session id it holds no context for; release the
				// half-updated registration to avoid leaking it.
				abortRegistration(ctx, amfInstance, ue, "UE referenced unknown PDU session id", nil)

				return
			}

			var (
				nasPdu []byte
				err    error
			)

			if n1Msg != nil {
				nasPdu, err = amf.BuildDLNASTransport(ue, fgs.PayloadContainerTypeN1SMInfo, n1Msg, new(fgs.PDUSessionID(requestData.PduSessionID)), nil, nil)
				if err != nil {
					logger.From(ctx, logger.AmfLog).Warn("failed to build DL NAS transport", zap.Error(err))
					return
				}
			}

			item, err := amf.PDUSessionSetupItemSUReq(requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
			if err != nil {
				logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", requestData.PduSessionID))
			} else {
				suList = append(suList, item)
			}
		}
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	if ueConn.UeContextRequest {
		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

		amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

		logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")

		return
	} else {
		nasPdu, err := amf.BuildRegistrationAccept(amfInstance, ue, guti, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "build registration accept", err)
			return
		}

		if len(suList) != 0 {
			metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

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
			metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

			err := ueConn.SendDownlinkNASTransport(ctx, nasPdu)
			if err != nil {
				abortRegistration(ctx, amfInstance, ue, "send downlink NAS transport", err)
				return
			}

			amf.ArmRegistrationAcceptGuard(amfInstance, ue, nasPdu)

			logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport message")
		}
	}
}

func movingFromEPC(req *fgs.RegistrationRequest) bool {
	return req != nil && req.UEStatus != nil && req.UEStatus.S1ModeReg
}
