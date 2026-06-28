// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
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
		// update Kgnb/Kn3iwf
		err := ue.UpdateSecurityContext()
		if err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}

		nasPdu, err := message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

		ranUe.ICS = amf.ICSPending

		err = ranUe.SendInitialContextSetupRequest(
			ctx,
			ue.Current().Ambr.Uplink,
			ue.Current().Ambr.Downlink,
			ue.Current().AllowedNssai,
			ue.Current().Kgnb,
			ue.PlmnID,
			ue.Current().UeRadioCapability,
			ue.Current().UeRadioCapabilityForPaging,
			ue.Current().UESecurityCapability,
			nasPdu,
			&ctxList,
			supportedGUAMI,
		)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %v", err)
		}

		ue.Log.Info("sent service accept with initial context setup request")
	} else if len(suList.List) != 0 {
		nasPdu, err := message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

		err = ranUe.SendPDUSessionResourceSetupRequest(
			ctx,
			ue.Current().Ambr.Uplink,
			ue.Current().Ambr.Downlink,
			nasPdu,
			suList,
		)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource setup request: %v", err)
		}

		ue.Log.Info("sent service accept")
	} else {
		err := message.SendServiceAccept(ctx, ranUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}

		ue.Log.Info("sent service accept")
	}

	return nil
}

// TS 24501 5.6.1
func handleServiceRequest(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.ServiceRequest, integrityVerified bool) error {
	// Validate state before accessing RanUe — state checks are cheap and
	// independent of the RAN connection.
	state := ue.GetState()
	if state != amf.Deregistered && state != amf.Registered {
		return fmt.Errorf("state mismatch: receive Service Request message in state %s", state)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	// TS 24.501 5.6.1.1: reject service request from deregistered UE
	if state == amf.Deregistered {
		err := message.SendServiceReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}

		ranUe.ReleaseAction = amf.UeContextN2NormalRelease

		err = ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}

		return nil
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	if conn.T3513 != nil {
		conn.T3513.Stop()
		conn.T3513 = nil
	}

	if conn.T3565 != nil {
		conn.T3565.Stop()
		conn.T3565 = nil
	}

	if conn.Procedures.Active(procedure.Paging) {
		conn.Procedures.End(procedure.Paging)
	}

	// TS 24.501 8.2.6.21: if the UE is sending a REGISTRATION REQUEST message as an initial NAS message,
	// the UE has a valid 5G NAS security context and the UE needs to send non-cleartext IEs
	// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS message
	// container IE, the UE shall set the security header type of the initial NAS message to "integrity protected"
	if msg.NASMessageContainer != nil && (ue.SecurityContextIsValid() && integrityVerified) {
		contents := msg.GetNASMessageContainerContents()

		// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS
		// message container IE, the UE shall set the security header type of the initial NAS message to
		// "integrity protected"; then the AMF shall decipher the value part of the NAS message container IE
		err := security.NASEncrypt(ue.Current().CipheringAlg, ue.Current().KnasEnc, ue.Current().ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, contents)
		if err != nil {
			ue.Current().SecurityContextAvailable = false
		} else {
			m := nas.NewMessage()
			if err := m.GmmMessageDecode(&contents); err != nil {
				return err
			}

			messageType := m.GmmHeader.GetMessageType()
			if messageType != nas.MsgTypeServiceRequest {
				return fmt.Errorf("expected service request message, got %d", messageType)
			}
			// TS 24.501 4.4.6: The AMF shall consider the NAS message that is obtained from the NAS message container
			// IE as the initial NAS message that triggered the procedure
			msg = m.ServiceRequest
		}
		// TS 33.501 6.4.6 step 3: if the initial NAS message was protected but did not pass the integrity check
		conn.RetransmissionOfInitialNASMsg = !integrityVerified
	}

	// Service Reject if the SecurityContext is invalid. TS 24.501 §4.4.4.3: a
	// service request failing the integrity check is rejected with 5GMM cause
	// #9 and the 5GMM-context and 5G NAS security context are left unchanged, so
	// an unauthenticated message cannot tear down a genuine UE's security state.
	if !ue.SecurityContextIsValid() || !integrityVerified {
		ue.Log.Warn("No valid security context for service request", logger.SUPI(ue.Supi.String()))

		err := message.SendServiceReject(ctx, ranUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}

		ue.Log.Info("sent service reject")

		ranUe.ReleaseAction = amf.UeContextN2NormalRelease

		err = ranUe.SendUEContextReleaseCommand(ctx, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}

		return nil
	}

	serviceType := msg.GetServiceTypeValue()

	logger.WithTrace(ctx, logger.AmfLog).Debug("Handle Service Request", logger.SUPI(ue.Supi.String()), zap.String("serviceType", serviceTypeToString(serviceType)))

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

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
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
	ue.Mutex.Lock()

	smContextSnapshot := make(map[uint8]*amf.SmContext, len(ue.Current().SmContextList))
	for id, sc := range ue.Current().SmContextList {
		smContextSnapshot[id] = sc
	}
	ue.Mutex.Unlock()

	// If the UE has uplink data pending for some PDU sessions, we need to activate them
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
					ue.Log.Error("Release SmContext Error", zap.Error(err))
				}
			} else {
				acceptPduSessionPsi[pduSessionID] = true
			}
		}
	}

	switch serviceType {
	case nasMessage.ServiceTypeMobileTerminatedServices: // Triggered by Network
		// TS 24.501 5.4.4.1 - We need to assign a new GUTI after a successful Service Request
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

				err = message.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport message: %v", err)
				}

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
					// This case is currently not tested and seems wrong. I was not able to find a case
					// for this, and the NAS message stored for the UE is added in way that decryption does
					// not seem to work.
					nasPdu, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
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

		message.SendConfigurationUpdateCommand(ctx, amfInstance, ue, true)

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
