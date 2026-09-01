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

	if ueConn.ICS() == amf.ICSNotStarted {
		if err := ue.UpdateSecurityContext(); err != nil {
			abortRegistration(ctx, amfInstance, ue, "update security context", err)
			return
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

	if !subscriberProfile.Allow5G {
		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultReject)

		logger.From(ctx, logger.AmfLog).Info("registration update rejected: 5G not allowed for subscriber")

		amf.SendRegistrationReject(ctx, ueConn, fgs.GMMCauseServicesNotAllowed)
		ue.Deregister(ctx)

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

	ue.SetAmbr(subscriberProfile.Ambr)
	ue.SetAllow4G(subscriberProfile.Allow4G)

	if !adoptArrivingSessions(ctx, amfInstance, ue, conn) {
		return
	}

	releaseLocallyDeactivatedEPSBearers(ctx, amfInstance, ue, conn)

	var (
		reactivationResult        *[16]bool
		errPduSessionID, errCause []uint8
	)

	var (
		ctxList ngap.PDUSessionResourceSetupListCxtReq
		suList  ngap.PDUSessionResourceSetupListSUReq
	)

	appendPendingN1 := func(uint8) error { return nil }

	if conn.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := conn.RegistrationRequest.UplinkDataStatus.PSI
		reactivationResult = new([16]bool)

		for idx, hasUplinkData := range uplinkDataPsi {
			pduSessionID := uint8(idx)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if hasUplinkData {
					if !ueConn.ClaimN2SetupSession(n2SetupProcedure(ueConn.UeContextRequest), pduSessionID) {
						logger.From(ctx, logger.AmfLog).Debug("skipping PDU session already set up on the NG-RAN node",
							zap.Uint8("pdu_session_id", pduSessionID))

						continue
					}

					binaryDataN2SmInformation, err := amfInstance.Session.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := fgs.GMMCauseProtocolErrorUnspecified
						errCause = append(errCause, uint8(cause))
					} else {
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

	pduSessionStatus, err := syncPDUSessionStatus(ctx, amfInstance, ue, conn.RegistrationRequest)
	if err != nil {
		abortRegistration(ctx, amfInstance, ue, "synchronise PDU session status", err)
		return
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

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

			if n2Info == nil || requestData.Standalone() {
				if len(suList) != 0 {
					plain, err := amf.BuildRegistrationAccept(amfInstance, ue, guti, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("failed to build registration accept", zap.Error(err))

						return
					}

					metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

					if err := ue.SendDownlinkNAS(plain, uint8(fgs.SHTIntegrityProtectedCiphered), func(wire []byte) error {
						if err := ueConn.SendPDUSessionResourceSetupRequest(
							ctx,
							ue.Ambr.Uplink,
							ue.Ambr.Downlink,
							wire,
							suList,
						); err != nil {
							ueConn.EndN2Setup(amf.N2SetupPDUSession)

							return err
						}

						ueConn.ArmN2Setup(amf.N2SetupPDUSession, amfInstance.N2SetupGuardCfg)

						return nil
					}); err != nil {
						abortRegistration(ctx, amfInstance, ue, "send PDU session resource setup request", err)

						return
					}

					amf.ArmRegistrationAcceptGuard(amfInstance, ue, plain)

					logger.From(ctx, logger.AmfLog).Info("Sent NGAP pdu session resource setup request")
				} else {
					ueConn.EndN2Setup(amf.N2SetupPDUSession)

					metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

					amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

					logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")
				}

				if requestData.Standalone() {
					if err := amf.DeliverStandaloneN1N2(ctx, ue, ueConn, requestData); err != nil {
						logger.From(ctx, logger.AmfLog).Warn("failed to deliver buffered downlink message", zap.Error(err))
					}
				} else {
					amf.SendDLNASTransport(ctx, ueConn, fgs.PayloadContainerTypeN1SMInfo, n1Msg, fgs.PDUSessionID(requestData.PduSessionID), 0)
				}

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

			appendPendingN1 = func(sht uint8) error {
				stage := func(nasPdu []byte) error {
					item, err := amf.PDUSessionSetupItemSUReq(requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", requestData.PduSessionID))

						return nil
					}

					suList = append(suList, item)

					return nil
				}

				if n1Msg == nil {
					return stage(nil)
				}

				plain, err := amf.BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, n1Msg, new(fgs.PDUSessionID(requestData.PduSessionID)), nil, nil)
				if err != nil {
					return err
				}

				return ue.SendDownlinkNAS(plain, sht, stage)
			}
		}
	}

	if ueConn.UeContextRequest {
		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

		amf.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)

		logger.From(ctx, logger.AmfLog).Info("Sent GMM registration accept")

		return
	} else {
		sht := uint8(fgs.SHTIntegrityProtectedCiphered)

		if err := appendPendingN1(sht); err != nil {
			abortRegistration(ctx, amfInstance, ue, "send buffered N1 SM message", err)

			return
		}

		plain, err := amf.BuildRegistrationAccept(amfInstance, ue, guti, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
		if err != nil {
			abortRegistration(ctx, amfInstance, ue, "build registration accept", err)

			return
		}

		metrics.RegistrationAttempt(metrics.RAT5G, registrationTypeName(conn.RegistrationType5GS), metrics.ResultAccept)

		if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
			if len(suList) != 0 {
				if err := ueConn.SendPDUSessionResourceSetupRequest(
					ctx,
					ue.Ambr.Uplink,
					ue.Ambr.Downlink,
					wire,
					suList,
				); err != nil {
					ueConn.EndN2Setup(amf.N2SetupPDUSession)

					return err
				}

				ueConn.ArmN2Setup(amf.N2SetupPDUSession, amfInstance.N2SetupGuardCfg)

				return nil
			}

			ueConn.EndN2Setup(amf.N2SetupPDUSession)

			return ueConn.SendDownlinkNASTransport(ctx, wire)
		}); err != nil {
			abortRegistration(ctx, amfInstance, ue, "send registration accept", err)

			return
		}

		amf.ArmRegistrationAcceptGuard(amfInstance, ue, plain)

		logger.From(ctx, logger.AmfLog).Info("sent registration accept")
	}
}

func movingFromEPC(req *fgs.RegistrationRequest) bool {
	return req != nil && req.UEStatus != nil && req.UEStatus.S1ModeReg
}

func releaseLocallyDeactivatedEPSBearers(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, conn *amf.UeConn) {
	status := conn.RegistrationRequest.EPSBearerContextStatus
	if status == nil || !conn.ArrivedFromEPS() || amfInstance.EPS == nil {
		return
	}

	for pduSessionID, ebi := range ue.EPSBearerIdentities() {
		if int(ebi) < len(status.Active) && status.Active[ebi] {
			continue
		}

		smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			continue
		}

		if err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to release a PDU session the UE deactivated in EPS",
				zap.Error(err), zap.Uint8("pdu_session_id", pduSessionID), zap.Uint8("ebi", ebi))

			continue
		}

		ue.DeleteSmContext(pduSessionID)
	}
}
