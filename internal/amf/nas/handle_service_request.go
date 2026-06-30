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
	"github.com/ellanetworks/core/internal/amf/procedure"
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
	ranUe *amf.RanUe,
	ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID []uint8,
	errCause []uint8,
	supportedGUAMI *models.Guami,
) error {
	if ranUe.UeContextRequest {
		err := ue.UpdateSecurityContext()
		if err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}

		nasPdu, err := amf.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

		ranUe.ICS = amf.ICSPending

		err = ranUe.SendInitialContextSetupRequest(
			ctx,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb(),
			ue.PlmnID,
			ue.UeRadioCapability,
			ue.UeRadioCapabilityForPaging,
			ue.UESecCap(),
			nasPdu,
			&ctxList,
			supportedGUAMI,
		)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %v", err)
		}

		ue.Log.Info("sent service accept with initial context setup request")
	} else if len(suList.List) != 0 {
		nasPdu, err := amf.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

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

		ue.Log.Info("sent service accept")
	} else {
		amf.SendServiceAccept(ctx, ranUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)

		ue.Log.Info("sent service accept")
	}

	return nil
}

// TS 24501 5.6.1
func handleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.ServiceRequest, integrityVerified bool) error {
	state := ue.State()
	if state != amf.Deregistered && state != amf.Registered {
		return fmt.Errorf("state mismatch: receive Service Request message in state %s", state)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	// TS 24.501: reject service request from deregistered UE
	if state == amf.Deregistered {
		amf.SendServiceReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

		ranUe.ReleaseAction = amf.UeContextN2NormalRelease

		if err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease); err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}

		return nil
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	conn.T3513.Stop()
	conn.T3565.Stop()

	if conn.Procedures.Active(procedure.Paging) {
		conn.Procedures.End(procedure.Paging)
	}

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
				return err
			}

			messageType := m.GmmHeader.GetMessageType()
			if messageType != nas.MsgTypeServiceRequest {
				return fmt.Errorf("expected service request message, got %d", messageType)
			}

			msg = m.ServiceRequest
		}
		// TS 33.501: protected initial NAS message that failed the integrity check.
		conn.RetransmissionOfInitialNASMsg = !integrityVerified
	}

	// Service Reject if the SecurityContext is invalid. TS 24.501: a
	// service request failing the integrity check is rejected with 5GMM cause
	// #9 and the 5GMM-context and 5G NAS security context are left unchanged, so
	// an unauthenticated message cannot tear down a genuine UE's security state.
	if !ue.SecurityContextIsValid() || !integrityVerified {
		ue.Log.Warn("No valid security context for service request", logger.SUPI(ue.Supi().String()))

		amf.SendServiceReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)

		ue.Log.Info("sent service reject")

		ranUe.ReleaseAction = amf.UeContextN2NormalRelease

		if err := ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease); err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}

		return nil
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
		ue.Log.Warn("emergency service is not supported")
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	if serviceType == nasMessage.ServiceTypeSignalling {
		err := sendServiceAccept(ctx, ue, ranUe, ctxList, suList, nil, nil, nil, nil, operatorInfo.Guami)
		return err
	}

	if conn.N1N2Message != nil {
		requestData := conn.N1N2Message
		if conn.N1N2Message.BinaryDataN2Information != nil {
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
				ue.Log.Warn("Ignoring out-of-range PDU session ID in UplinkDataStatus processing", zap.Uint8("pduSessionID", pduSessionID))
				continue
			}

			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] {
					binaryDataN2SmInformation, err := amfInstance.Smf.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						ue.Log.Error("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else if ranUe.UeContextRequest {
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
				ue.Log.Warn("Ignoring out-of-range PDU session ID in PDUSessionStatus processing", zap.Uint8("pduSessionID", pduSessionID))
				continue
			}

			if !psiArray[pduSessionID] { // #nosec: G602 -- bounds checked above
				err := amfInstance.Smf.ReleaseSmContext(ctx, smContext.Ref)
				if err != nil {
					ue.Log.Error("Release amf.SmContext Error", zap.Error(err))
				}
			} else {
				acceptPduSessionPsi[pduSessionID] = true
			}
		}
	}

	switch serviceType {
	case nasMessage.ServiceTypeMobileTerminatedServices: // Triggered by Network
		// TS 24.501 requires assigning a new GUTI after a successful Service Request
		// triggered by a paging request.
		if conn.N1N2Message != nil {
			requestData := conn.N1N2Message
			n1Msg := conn.N1N2Message.BinaryDataN1Message
			n2Info := conn.N1N2Message.BinaryDataN2Information

			// Paging was triggered for downlink signaling only
			if n2Info == nil && n1Msg != nil {
				err := sendServiceAccept(ctx, ue, ranUe, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
				if err != nil {
					return fmt.Errorf("error sending service accept: %v", err)
				}

				amf.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)

				ue.Log.Info("sent downlink nas transport message")

				conn.N1N2Message = nil
			} else {
				_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
				if !exist {
					conn.N1N2Message = nil
					return fmt.Errorf("service Request triggered by Network for pduSessionID that does not exist")
				}

				var nasPdu []byte
				if n1Msg != nil {
					// Untested path: the stored N1 message may not decrypt correctly here.
					nasPdu, err = amf.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
					if err != nil {
						return fmt.Errorf("error building DL NAS transport message: %v", err)
					}
				}

				if ranUe.UeContextRequest {
					send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				} else {
					send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				}

				ue.Log.Debug("sending service accept")

				err := sendServiceAccept(ctx, ue, ranUe, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
				if err != nil {
					return fmt.Errorf("error sending service accept: %v", err)
				}
			}
		} else {
			err := sendServiceAccept(ctx, ue, ranUe, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
			if err != nil {
				return fmt.Errorf("error sending service accept: %v", err)
			}
		}

		err := amfInstance.ReAllocateGuti(ctx, ue, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error reallocating GUTI to UE: %v", err)
		}

		amf.SendConfigurationUpdateCommand(ctx, amfInstance, ue, true)

	case nasMessage.ServiceTypeData:
		err := sendServiceAccept(ctx, ue, ranUe, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}
	default:
		return fmt.Errorf("service type is not supported: %d", serviceType)
	}

	if len(errPduSessionID) != 0 {
		ue.Log.Info("", zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	}

	conn.N1N2Message = nil

	return nil
}
