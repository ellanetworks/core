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
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func serviceTypeToString(serviceType uint8) string {
	switch serviceType {
	case nasMessage.ServiceTypeSignalling:
		return "Signalling"
	case nasMessage.ServiceTypeData:
		return "Data"
	case nasMessage.ServiceTypeMobileTerminatedServices:
		return "Mobile Terminated Services"
	case nasMessage.ServiceTypeEmergencyServices:
		return "Emergency Services"
	case nasMessage.ServiceTypeEmergencyServicesFallback:
		return "Emergency Services Fallback"
	case nasMessage.ServiceTypeHighPriorityAccess:
		return "High Priority Access"
	default:
		return "Unknown"
	}
}

func sendServiceAccept(
	ctx context.Context,
	ue *amf.UeContext,
	ueConn *amf.UeConn,
	ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID []uint8,
	errCause []uint8,
	supportedGUAMI *models.Guami,
) error {
	if ueConn.UeContextRequest {
		err := ue.UpdateSecurityContext()
		if err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}

		nasPdu, err := amf.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

		ueConn.MarkICSPending()

		err = ueConn.SendInitialContextSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb(),
			ue.RadioCapability,
			ue.RadioCapabilityForPaging,
			ue.UESecCap(),
			nasPdu,
			&ctxList,
			supportedGUAMI,
		)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("sent service accept with initial context setup request")
	} else if len(suList.List) != 0 {
		nasPdu, err := amf.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

		err = ueConn.SendPDUSessionResourceSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			nasPdu,
			suList,
		)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource setup request: %v", err)
		}

		logger.From(ctx, logger.AmfLog).Info("sent service accept")
	} else {
		amf.SendServiceAccept(ctx, ueConn, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)

		logger.From(ctx, logger.AmfLog).Info("sent service accept")
	}

	return nil
}

// TS 24501 5.6.1
func handleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.ServiceRequest, integrityVerified bool) {
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
		rejectService(ctx, ueConn, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
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
		contents := msg.GetNASMessageContainerContents()

		err := ue.DecryptUplinkContents(contents)
		if err != nil {
			ue.ClearSecured()
		} else {
			m := nas.NewMessage()
			if err := m.GmmMessageDecode(&contents); err != nil {
				logger.From(ctx, logger.AmfLog).Warn("failed to decode service request NAS message container", zap.Error(err))
				return
			}

			messageType := m.GmmHeader.GetMessageType()
			if messageType != nas.MsgTypeServiceRequest {
				logger.From(ctx, logger.AmfLog).Warn("expected service request message", zap.Uint8("messageType", messageType))
				return
			}

			msg = m.ServiceRequest
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

		rejectService(ctx, ueConn, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

		return
	}

	serviceType := msg.GetServiceTypeValue()

	logger.WithTrace(ctx, logger.AmfLog).Debug("Handle Service Request", logger.SUPI(ue.Supi().String()), zap.String("serviceType", serviceTypeToString(serviceType)))

	var (
		reactivationResult, acceptPduSessionPsi *[16]bool
		errPduSessionID, errCause               []uint8
		targetPduSessionID                      uint8
	)

	suList := ngapType.PDUSessionResourceSetupListSUReq{}
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}

	if serviceType == nasMessage.ServiceTypeEmergencyServices ||
		serviceType == nasMessage.ServiceTypeEmergencyServicesFallback {
		// Ella does not provide emergency services; the request cannot be accepted, so
		// answer SERVICE REJECT #7 "5GS services not allowed" rather than silently dropping
		// it (TS 24.501 §5.6.1.5).
		logger.From(ctx, logger.AmfLog).Warn("emergency service is not supported; rejecting service request")
		rejectService(ctx, ueConn, nasMessage.Cause5GMM5GSServicesNotAllowed)

		return
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("error getting operator info", zap.Error(err))
		return
	}

	if serviceType == nasMessage.ServiceTypeSignalling {
		if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, nil, nil, nil, nil, operatorInfo.Guami); err != nil {
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
		uplinkDataPsi := nasConvert.PSIToBooleanArray(msg.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)

		for pduSessionID, smContext := range smContextSnapshot {
			if int(pduSessionID) >= len(uplinkDataPsi) {
				logger.From(ctx, logger.AmfLog).Warn("Ignoring out-of-range PDU session ID in UplinkDataStatus processing", zap.Uint8("pduSessionID", pduSessionID))
				continue
			}

			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] {
					binaryDataN2SmInformation, err := amfInstance.Session.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Error("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else if ueConn.UeContextRequest {
						send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
					} else {
						send.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
					}
				}
			}
		}
	}

	if msg.PDUSessionStatus != nil {
		acceptPduSessionPsi = new([16]bool)

		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionStatus.Buffer)
		for pduSessionID, smContext := range smContextSnapshot {
			if int(pduSessionID) >= len(psiArray) {
				logger.From(ctx, logger.AmfLog).Warn("Ignoring out-of-range PDU session ID in PDUSessionStatus processing", zap.Uint8("pduSessionID", pduSessionID))
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
	case nasMessage.ServiceTypeMobileTerminatedServices:
		// TS 24.501 requires assigning a new GUTI after a successful Service Request
		// triggered by a paging request.
		if requestData := ue.N1N2Message(); requestData != nil {
			n1Msg := requestData.BinaryDataN1Message
			n2Info := requestData.BinaryDataN2Information

			// Paging was triggered for downlink signaling only
			if n2Info == nil && n1Msg != nil {
				if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami); err != nil {
					logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
					return
				}

				amf.SendDLNASTransport(ctx, ueConn, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)

				logger.From(ctx, logger.AmfLog).Info("sent downlink nas transport message")

				ue.ClearN1N2Message()
			} else {
				_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
				if !exist {
					ue.ClearN1N2Message()
					logger.From(ctx, logger.AmfLog).Warn("service Request triggered by Network for pduSessionID that does not exist")

					return
				}

				var nasPdu []byte
				if n1Msg != nil {
					nasPdu, err = amf.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
					if err != nil {
						logger.From(ctx, logger.AmfLog).Warn("error building DL NAS transport message", zap.Error(err))
						return
					}
				}

				if ueConn.UeContextRequest {
					send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				} else {
					send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				}

				logger.From(ctx, logger.AmfLog).Debug("sending service accept")

				if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami); err != nil {
					logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
					return
				}
			}
		} else {
			if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami); err != nil {
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

	case nasMessage.ServiceTypeData, nasMessage.ServiceTypeHighPriorityAccess:
		if err := sendServiceAccept(ctx, ue, ueConn, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("error sending service accept", zap.Error(err))
			return
		}
	default:
		// TS 24.501 §5.6.1.5: a service request with an unsupported or unknown service type
		// cannot be accepted; answer SERVICE REJECT rather than silently dropping it.
		logger.From(ctx, logger.AmfLog).Warn("service type is not supported; rejecting", zap.Uint8("serviceType", serviceType))
		rejectService(ctx, ueConn, nasMessage.Cause5GMMProtocolErrorUnspecified)

		return
	}

	if len(errPduSessionID) != 0 {
		logger.From(ctx, logger.AmfLog).Info("", zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	}

	ue.ClearN1N2Message()
}

// rejectService answers a service request the AMF cannot accept with a SERVICE REJECT
// carrying cause, then releases the RAN connection (TS 24.501 §5.6.1.5).
func rejectService(ctx context.Context, ueConn *amf.UeConn, cause uint8) {
	amf.SendServiceReject(ctx, ueConn, cause)

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease
	ueConn.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
}
