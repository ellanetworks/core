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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type pendingN1 struct {
	pduSessionID uint8
	snssai       *models.Snssai
	n1Msg        []byte
	n2Info       []byte
}

func sendServiceAccept(
	ctx context.Context,
	ue *amf.UeContext,
	ueConn *amf.UeConn,
	ctxList ngap.PDUSessionResourceSetupListCxtReq,
	suList ngap.PDUSessionResourceSetupListSUReq,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID []uint8,
	errCause []uint8,
	supportedGUAMI *models.Guami,
	pending *pendingN1,
) error {
	if ueConn.UeContextRequest {
		if err := ue.UpdateSecurityContext(); err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}
	}

	sht := uint8(fgs.SHTIntegrityProtectedCiphered)

	// The buffered N1 SM message and the SERVICE ACCEPT are two protected messages
	// riding one RAN message, so the gNB, not the AMF, decides the order they reach
	// the UE in. The two sends are kept adjacent so no third message can take a NAS
	// COUNT between them (TS 24.501 §4.4.3.1).
	if pending != nil {
		if err := stagePendingN1(ctx, ue, ueConn, pending, sht, &ctxList, &suList); err != nil {
			amf.ReportProtectFailure(ctx, ue, "buffered N1 SM message", err)

			return err
		}
	}

	plain, err := amf.BuildServiceAccept(pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
	if err != nil {
		return fmt.Errorf("error building service accept message: %v", err)
	}

	kgnb, ueSecCap := ue.Kgnb(), ue.UESecCap()

	if err := ue.SendDownlinkNAS(plain, sht, func(wire []byte) error {
		switch {
		case ueConn.UeContextRequest:
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
				ctxList,
				supportedGUAMI,
			); err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}

			logger.From(ctx, logger.AmfLog).Info("sent service accept with initial context setup request")
		case len(suList) != 0:
			if err := ueConn.SendPDUSessionResourceSetupRequest(
				ctx,
				ue.Ambr.Uplink,
				ue.Ambr.Downlink,
				wire,
				suList,
			); err != nil {
				return fmt.Errorf("error sending pdu session resource setup request: %v", err)
			}

			logger.From(ctx, logger.AmfLog).Info("sent service accept")
		default:
			if err := ueConn.SendDownlinkNASTransport(ctx, wire); err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}

			logger.From(ctx, logger.AmfLog).Info("sent service accept")
		}

		return nil
	}); err != nil {
		amf.ReportProtectFailure(ctx, ue, "service accept", err)

		return err
	}

	return nil
}

// stagePendingN1 protects the buffered N1 SM message and stages it on the PDU
// session setup item the SERVICE ACCEPT's RAN message carries. The write does not
// reach the RAN itself: the item is handed to the gNB by the send that follows.
func stagePendingN1(
	ctx context.Context,
	ue *amf.UeContext,
	ueConn *amf.UeConn,
	pending *pendingN1,
	sht uint8,
	ctxList *ngap.PDUSessionResourceSetupListCxtReq,
	suList *ngap.PDUSessionResourceSetupListSUReq,
) error {
	stage := func(nasPdu []byte) error {
		if ueConn.UeContextRequest {
			item, err := amf.PDUSessionSetupItem(pending.pduSessionID, pending.snssai, nasPdu, pending.n2Info)
			if err != nil {
				logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", pending.pduSessionID))

				return nil
			}

			*ctxList = append(*ctxList, item)

			return nil
		}

		item, err := amf.PDUSessionSetupItemSUReq(pending.pduSessionID, pending.snssai, nasPdu, pending.n2Info)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("could not build PDU session setup item", zap.Error(err), zap.Uint8("pdu_session_id", pending.pduSessionID))

			return nil
		}

		*suList = append(*suList, item)

		return nil
	}

	if pending.n1Msg == nil {
		return stage(nil)
	}

	plain, err := amf.BuildDLNASTransport(fgs.PayloadContainerTypeN1SMInfo, pending.n1Msg, new(fgs.PDUSessionID(pending.pduSessionID)), nil, nil)
	if err != nil {
		return fmt.Errorf("error building DL NAS transport message: %v", err)
	}

	return ue.SendDownlinkNAS(plain, sht, stage)
}

// TS 24501 5.6.1
func handleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, plain []byte, integrityVerified bool) {
	msg, err := fgs.ParseServiceRequest(plain)
	if !decoded(ctx, "ServiceRequest", err) {
		logger.From(ctx, logger.AmfLog).Warn("failed to decode Service Request", zap.Error(err))
		return
	}

	state := ue.State()
	if state != amf.Deregistered && state != amf.Registered {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Service Request message", zap.String("state", string(state)))
		return
	}

	ueConn := ue.Conn()
	if ueConn == nil {
		logger.From(ctx, logger.AmfLog).Warn("ue is not connected to RAN")
		return
	}

	// TS 24.501: reject service request from deregistered UE
	if state == amf.Deregistered {
		rejectService(ctx, ueConn, fgs.GMMCauseUEIdentityCannotBeDerived)
		return
	}

	conn := ue.Conn()
	if conn == nil {
		logger.From(ctx, logger.AmfLog).Warn("no active NAS connection")
		return
	}

	ue.StopPaging()
	conn.StopNASGuard()

	// TS 24.501: an integrity-protected SERVICE REQUEST carrying a NAS
	// message container holds the real initial NAS message in that container;
	// decipher it and use it in place of the outer message.
	if msg.NASMessageContainer != nil && (ue.SecurityContextIsValid() && integrityVerified) {
		contents := append([]byte(nil), msg.NASMessageContainer...)

		err := ue.DecryptUplinkContents(contents)
		if err != nil {
			ue.ClearSecured()
		} else {
			inner, err := fgs.ParseServiceRequest(contents)
			if !decoded(ctx, "ServiceRequest", err) {
				logger.From(ctx, logger.AmfLog).Warn("failed to decode service request NAS message container", zap.Error(err))
				return
			}

			msg = inner
		}
		// TS 33.501: protected initial NAS message that failed the integrity check.
		conn.SetRetransmissionOfInitialNASMsg(!integrityVerified)
	}

	// Service Reject if the SecurityContext is invalid. TS 24.501: a
	// service request failing the integrity check is rejected with 5GMM cause
	// #9 and the 5GMM-context and 5G NAS security context are left unchanged, so
	// an unauthenticated message cannot tear down a genuine UE's security state.
	if !ue.SecurityContextIsValid() || !integrityVerified {
		logger.From(ctx, logger.AmfLog).Warn("No valid security context for service request", logger.SUPI(ue.Supi().String()))

		rejectService(ctx, ueConn, fgs.GMMCauseUEIdentityCannotBeDerived)

		return
	}

	serviceType := msg.ServiceType

	logger.WithTrace(ctx, logger.AmfLog).Debug("Handle Service Request", logger.SUPI(ue.Supi().String()), zap.String("service_type", serviceType.String()))

	var (
		reactivationResult, acceptPduSessionPsi *[16]bool
		errPduSessionID, errCause               []uint8
		targetPduSessionID                      uint8
	)

	var (
		suList  ngap.PDUSessionResourceSetupListSUReq
		ctxList ngap.PDUSessionResourceSetupListCxtReq
	)

	if serviceType == fgs.ServiceTypeEmergencyServices ||
		serviceType == fgs.ServiceTypeEmergencyServicesFallback {
		// Ella does not provide emergency services; the request cannot be accepted, so
		// answer SERVICE REJECT #7 "5GS services not allowed" rather than silently dropping
		// it (TS 24.501 §5.6.1.5).
		logger.From(ctx, logger.AmfLog).Warn("emergency service is not supported; rejecting service request")
		rejectService(ctx, ueConn, fgs.GMMCauseServicesNotAllowed)

		return
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("error getting operator info", zap.Error(err))
		return
	}

	if serviceType == fgs.ServiceTypeSignalling {
		if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, nil, nil, nil, nil, operatorInfo.Guami, nil); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
		}

		return
	}

	if requestData := ue.N1N2Message(); requestData != nil {
		if requestData.BinaryDataN2Information != nil {
			targetPduSessionID = requestData.PduSessionID
		}
	}

	// Copy SmContextList under lock for safe concurrent iteration.
	smContextSnapshot := ue.SmContextSnapshot()

	if msg.UplinkDataStatus != nil {
		uplinkDataPsi := msg.UplinkDataStatus.PSI
		reactivationResult = new([16]bool)

		for pduSessionID, smContext := range smContextSnapshot {
			if int(pduSessionID) >= len(uplinkDataPsi) {
				logger.From(ctx, logger.AmfLog).Warn("Ignoring out-of-range PDU session ID in UplinkDataStatus processing", zap.Uint8("pdu_session_id", pduSessionID))
				continue
			}

			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] {
					binaryDataN2SmInformation, err := amfInstance.Session.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Error("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pdu_session_id", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := fgs.GMMCauseProtocolErrorUnspecified
						errCause = append(errCause, uint8(cause))

						continue
					}

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

	if msg.PDUSessionStatus != nil {
		acceptPduSessionPsi = new([16]bool)

		psiArray := msg.PDUSessionStatus.PSI
		for pduSessionID, smContext := range smContextSnapshot {
			if int(pduSessionID) >= len(psiArray) {
				logger.From(ctx, logger.AmfLog).Warn("Ignoring out-of-range PDU session ID in PDUSessionStatus processing", zap.Uint8("pdu_session_id", pduSessionID))
				continue
			}

			if !psiArray[pduSessionID] { // #nosec: G602 -- bounds checked above
				err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref)
				if err != nil {
					logger.From(ctx, logger.AmfLog).Error("Release amf.SmContext Error", zap.Error(err))
				}
			} else {
				acceptPduSessionPsi[pduSessionID] = true
			}
		}
	}

	switch serviceType {
	case fgs.ServiceTypeMobileTerminatedServices:
		// TS 24.501 requires assigning a new GUTI after a successful Service Request
		// triggered by a paging request.
		if requestData := ue.N1N2Message(); requestData != nil {
			n1Msg := requestData.BinaryDataN1Message
			n2Info := requestData.BinaryDataN2Information

			// Paging was triggered for downlink signaling only
			if n2Info == nil && n1Msg != nil {
				if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami, nil); err != nil {
					logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
					return
				}

				amf.SendDLNASTransport(ctx, ueConn, fgs.PayloadContainerTypeN1SMInfo, n1Msg, fgs.PDUSessionID(requestData.PduSessionID), 0)

				logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport message")

				ue.ClearN1N2Message()
			} else {
				_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
				if !exist {
					ue.ClearN1N2Message()
					logger.From(ctx, logger.AmfLog).Warn("service Request triggered by Network for pduSessionID that does not exist")

					return
				}

				ue.SetSmContextActive(requestData.PduSessionID)

				pending := &pendingN1{
					pduSessionID: requestData.PduSessionID,
					snssai:       requestData.SNssai,
					n1Msg:        n1Msg,
					n2Info:       n2Info,
				}

				logger.From(ctx, logger.AmfLog).Debug("sending service accept")

				if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami, pending); err != nil {
					logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
					return
				}
			}
		} else {
			if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami, nil); err != nil {
				logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
				return
			}
		}

		err := amfInstance.ReallocateGUTI(ctx, ue)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error reallocating GUTI to UE", zap.Error(err))
			return
		}

		amf.SendConfigurationUpdateCommand(ctx, amfInstance, ue, true)

	case fgs.ServiceTypeData, fgs.ServiceTypeHighPriorityAccess:
		if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami, nil); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
			return
		}
	default:
		// TS 24.501 §5.6.1.5: a service request with an unsupported or unknown service type
		// cannot be accepted; answer SERVICE REJECT rather than silently dropping it.
		logger.From(ctx, logger.AmfLog).Warn("service type is not supported; rejecting", zap.Stringer("service_type", serviceType))
		rejectService(ctx, ueConn, fgs.GMMCauseProtocolErrorUnspecified)

		return
	}

	if len(errPduSessionID) != 0 {
		logger.From(ctx, logger.AmfLog).Info("", zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	}

	ue.ClearN1N2Message()
}

// rejectService answers a service request the AMF cannot accept with a SERVICE REJECT
// carrying cause, then releases the RAN connection (TS 24.501 §5.6.1.5).
func rejectService(ctx context.Context, ueConn *amf.UeConn, cause fgs.GMMCause) {
	amf.SendServiceReject(ctx, ueConn, cause)

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease
	ueConn.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
}
