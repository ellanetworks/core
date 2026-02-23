package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
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
	ue *amfContext.AmfUe,
	ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq,
	pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool,
	errPduSessionID []uint8,
	errCause []uint8,
	supportedGUAMI *models.Guami,
) error {
	if ue.RanUe.UeContextRequest {
		// update Kgnb/Kn3iwf
		err := ue.UpdateSecurityContext()
		if err != nil {
			return fmt.Errorf("error updating security context: %v", err)
		}

		nasPdu, err := message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building service accept message: %v", err)
		}

		ue.RanUe.SentInitialContextSetupRequest = true

		err = ue.RanUe.Radio.NGAPSender.SendInitialContextSetupRequest(
			ctx,
			ue.RanUe.AmfUeNgapID,
			ue.RanUe.RanUeNgapID,
			ue.Ambr.Uplink,
			ue.Ambr.Downlink,
			ue.AllowedNssai,
			ue.Kgnb,
			ue.PlmnID,
			ue.UeRadioCapability,
			ue.UeRadioCapabilityForPaging,
			ue.UESecurityCapability,
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

		err = ue.RanUe.Radio.NGAPSender.SendPDUSessionResourceSetupRequest(
			ctx,
			ue.RanUe.AmfUeNgapID,
			ue.RanUe.RanUeNgapID,
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
		err := message.SendServiceAccept(ctx, ue.RanUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}

		ue.Log.Info("sent service accept")
	}

	return nil
}

// TS 24501 5.6.1
func handleServiceRequest(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe, msg *nasMessage.ServiceRequest) error {
	if ue.GetState() != amfContext.Deregistered && ue.GetState() != amfContext.Registered {
		return fmt.Errorf("state mismatch: receive Service Request message in state %s", ue.GetState())
	}

	if ue.T3513 != nil {
		ue.T3513.Stop()
		ue.T3513 = nil // clear the timer
	}

	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	// Set No ongoing
	if procedure := ue.GetOnGoing(); procedure == amfContext.OnGoingProcedurePaging {
		ue.SetOnGoing(amfContext.OnGoingProcedureNothing)
	} else if procedure != amfContext.OnGoingProcedureNothing {
		ue.Log.Warn("UE should not in OnGoing", zap.Any("procedure", procedure))
	}

	// TS 24.501 8.2.6.21: if the UE is sending a REGISTRATION REQUEST message as an initial NAS message,
	// the UE has a valid 5G NAS security context and the UE needs to send non-cleartext IEs
	// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS message
	// container IE, the UE shall set the security header type of the initial NAS message to "integrity protected"
	if msg.NASMessageContainer != nil && ue.SecurityContextIsValid() {
		contents := msg.GetNASMessageContainerContents()

		// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS
		// message container IE, the UE shall set the security header type of the initial NAS message to
		// "integrity protected"; then the AMF shall decipher the value part of the NAS message container IE
		err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, contents)
		if err != nil {
			ue.SecurityContextAvailable = false
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
		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	// Service Reject if the SecurityContext is invalid or the UE is Deregistered
	if !ue.SecurityContextIsValid() || ue.GetState() == amfContext.Deregistered {
		ue.Log.Warn("No security context", zap.String("supi", ue.Supi))
		ue.SecurityContextAvailable = false

		err := message.SendServiceReject(ctx, ue.RanUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}

		ue.Log.Info("sent service reject")
		ue.RanUe.ReleaseAction = amfContext.UeContextN2NormalRelease

		err = ue.RanUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}

		return nil
	}

	serviceType := msg.GetServiceTypeValue()

	logger.AmfLog.Debug("Handle Service Request", zap.String("supi", ue.Supi), zap.String("serviceType", serviceTypeToString(serviceType)))

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

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	if serviceType == nasMessage.ServiceTypeSignalling {
		err := sendServiceAccept(ctx, ue, ctxList, suList, nil, nil, nil, nil, operatorInfo.Guami)
		return err
	}

	if ue.N1N2Message != nil {
		requestData := ue.N1N2Message
		if ue.N1N2Message.BinaryDataN2Information != nil {
			targetPduSessionID = requestData.PduSessionID
		}
	}

	// If the UE has uplink data pending for some PDU sessions, we need to activate them
	if msg.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(msg.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)

		for pduSessionID, smContext := range ue.SmContextList {
			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] {
					binaryDataN2SmInformation, err := amf.Smf.ActivateSmContext(smContext.Ref)
					if err != nil {
						ue.Log.Error("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else if ue.RanUe.UeContextRequest {
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
		for pduSessionID, smContext := range ue.SmContextList {
			if !psiArray[pduSessionID] {
				err := amf.Smf.ReleaseSmContext(ctx, smContext.Ref)
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
		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message
			n1Msg := ue.N1N2Message.BinaryDataN1Message
			n2Info := ue.N1N2Message.BinaryDataN2Information

			// Paging was triggered for downlink signaling only
			if n2Info == nil && n1Msg != nil {
				err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
				if err != nil {
					return fmt.Errorf("error sending service accept: %v", err)
				}

				err = message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport message: %v", err)
				}

				ue.Log.Info("sent downlink nas transport message")

				ue.N1N2Message = nil
			} else {
				_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
				if !exist {
					ue.N1N2Message = nil
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

				if ue.RanUe.UeContextRequest {
					send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				} else {
					send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				}

				ue.Log.Debug("sending service accept")

				err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
				if err != nil {
					return fmt.Errorf("error sending service accept: %v", err)
				}
			}
		} else {
			err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
			if err != nil {
				return fmt.Errorf("error sending service accept: %v", err)
			}
		}

		err := ue.ReAllocateGuti(operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error reallocating GUTI to UE: %v", err)
		}

		message.SendConfigurationUpdateCommand(ctx, amf, ue)

	case nasMessage.ServiceTypeData:
		err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}
	default:
		return fmt.Errorf("service type is not supported: %d", serviceType)
	}

	if len(errPduSessionID) != 0 {
		ue.Log.Info("", zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	}

	ue.N1N2Message = nil

	return nil
}
