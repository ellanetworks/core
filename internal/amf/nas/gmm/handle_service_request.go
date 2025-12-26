package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
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

func sendServiceAccept(ctx ctxt.Context, ue *context.AmfUe, ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq, pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool, errPduSessionID, errCause []uint8, supportedGUAMI *models.Guami,
) error {
	if ue.RanUe.UeContextRequest {
		// update Kgnb/Kn3iwf
		ue.UpdateSecurityContext()

		nasPdu, err := message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionID, errCause)
		if err != nil {
			return err
		}
		if len(ctxList.List) != 0 {
			ue.RanUe.SentInitialContextSetupRequest = true
			err := ue.RanUe.Ran.NGAPSender.SendInitialContextSetupRequest(
				ctx,
				ue.RanUe.AmfUeNgapID,
				ue.RanUe.RanUeNgapID,
				ue.RanUe.AmfUe.Ambr.Uplink,
				ue.RanUe.AmfUe.Ambr.Downlink,
				ue.RanUe.AmfUe.AllowedNssai,
				ue.RanUe.AmfUe.Kgnb,
				ue.RanUe.AmfUe.PlmnID,
				ue.RanUe.AmfUe.UeRadioCapability,
				ue.RanUe.AmfUe.UeRadioCapabilityForPaging,
				ue.RanUe.AmfUe.UESecurityCapability,
				nasPdu,
				&ctxList,
				supportedGUAMI,
			)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.Log.Info("sent service accept with context list", zap.Int("len", len(ctxList.List)))
		} else {
			ue.RanUe.SentInitialContextSetupRequest = true
			err := ue.RanUe.Ran.NGAPSender.SendInitialContextSetupRequest(
				ctx,
				ue.RanUe.AmfUeNgapID,
				ue.RanUe.RanUeNgapID,
				ue.RanUe.AmfUe.Ambr.Uplink,
				ue.RanUe.AmfUe.Ambr.Downlink,
				ue.RanUe.AmfUe.AllowedNssai,
				ue.RanUe.AmfUe.Kgnb,
				ue.RanUe.AmfUe.PlmnID,
				ue.RanUe.AmfUe.UeRadioCapability,
				ue.RanUe.AmfUe.UeRadioCapabilityForPaging,
				ue.RanUe.AmfUe.UESecurityCapability,
				nasPdu,
				nil,
				supportedGUAMI,
			)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.Log.Info("sent service accept")
		}
	} else if len(suList.List) != 0 {
		nasPdu, err := message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionID, errCause)
		if err != nil {
			return err
		}
		err = ue.RanUe.Ran.NGAPSender.SendPDUSessionResourceSetupRequest(
			ctx,
			ue.RanUe.AmfUeNgapID,
			ue.RanUe.RanUeNgapID,
			ue.RanUe.AmfUe.Ambr.Uplink,
			ue.RanUe.AmfUe.Ambr.Downlink,
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
func handleServiceRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Service Request", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleServiceRequest")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != context.Deregistered && ue.State.Current() != context.Registered {
		return fmt.Errorf("state mismatch: receive Service Request message in state %s", ue.State.Current())
	}

	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
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
	if procedure := ue.GetOnGoing().Procedure; procedure == context.OnGoingProcedurePaging {
		ue.SetOnGoing(&context.OnGoingProcedureWithPrio{
			Procedure: context.OnGoingProcedureNothing,
		})
	} else if procedure != context.OnGoingProcedureNothing {
		ue.Log.Warn("UE should not in OnGoing", zap.Any("procedure", procedure))
	}

	// Send Authtication / Security Procedure not support
	// Rejecting ServiceRequest if it is received in Deregistered State
	if !ue.SecurityContextIsValid() || ue.State.Current() == context.Deregistered {
		ue.Log.Warn("No security context", zap.String("supi", ue.Supi))
		err := message.SendServiceReject(ctx, ue.RanUe, nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.Log.Info("sent service reject")
		ue.RanUe.ReleaseAction = context.UeContextN2NormalRelease
		err = ue.RanUe.Ran.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		return nil
	}

	// TS 24.501 8.2.6.21: if the UE is sending a REGISTRATION REQUEST message as an initial NAS message,
	// the UE has a valid 5G NAS security context and the UE needs to send non-cleartext IEs
	// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS message
	// container IE, the UE shall set the security header type of the initial NAS message to "integrity protected"
	if msg.ServiceRequest.NASMessageContainer != nil {
		contents := msg.ServiceRequest.NASMessageContainer.GetNASMessageContainerContents()

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

			messageType := m.GmmMessage.GmmHeader.GetMessageType()
			if messageType != nas.MsgTypeServiceRequest {
				return fmt.Errorf("expected service request message, got %d", messageType)
			}
			// TS 24.501 4.4.6: The AMF shall consider the NAS message that is obtained from the NAS message container
			// IE as the initial NAS message that triggered the procedure
			msg.ServiceRequest = m.ServiceRequest
		}
		// TS 33.501 6.4.6 step 3: if the initial NAS message was protected but did not pass the integrity check
		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	serviceType := msg.ServiceRequest.GetServiceTypeValue()

	logger.AmfLog.Debug("Handle Service Request", zap.String("supi", ue.Supi), zap.String("serviceType", serviceTypeToString(serviceType)))

	var reactivationResult, acceptPduSessionPsi *[16]bool
	var errPduSessionID, errCause []uint8
	var targetPduSessionID uint8
	suList := ngapType.PDUSessionResourceSetupListSUReq{}
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}

	if serviceType == nasMessage.ServiceTypeEmergencyServices ||
		serviceType == nasMessage.ServiceTypeEmergencyServicesFallback {
		ue.Log.Warn("emergency service is not supported")
	}

	if ue.MacFailed {
		ue.SecurityContextAvailable = false
		ue.Log.Warn("Security Context Exist, But Integrity Check Failed with existing Context", zap.String("supi", ue.Supi))
		err := message.SendServiceReject(ctx, ue.RanUe, nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.Log.Info("sent service reject")
		ue.RanUe.ReleaseAction = context.UeContextN2NormalRelease

		err = ue.RanUe.Ran.NGAPSender.SendUEContextReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}

		return nil
	}

	amfSelf := context.AMFSelf()

	operatorInfo, err := amfSelf.GetOperatorInfo(ctx)
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

	if msg.ServiceRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(msg.ServiceRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)

		for pduSessionID, smContext := range ue.SmContextList {
			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] {
					binaryDataN2SmInformation, err := pdusession.ActivateSmContext(smContext.SmContextRef())
					if err != nil {
						ue.Log.Error("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else if ue.RanUe.UeContextRequest {
						ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionID, smContext.Snssai(), nil, binaryDataN2SmInformation)
					} else {
						ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionID, smContext.Snssai(), nil, binaryDataN2SmInformation)
					}
				}
			}
		}
	}

	if msg.ServiceRequest.PDUSessionStatus != nil {
		acceptPduSessionPsi = new([16]bool)
		psiArray := nasConvert.PSIToBooleanArray(msg.ServiceRequest.PDUSessionStatus.Buffer)
		for pduSessionID, smContext := range ue.SmContextList {
			if !psiArray[pduSessionID] {
				err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
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
					nasPdu, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
					if err != nil {
						return fmt.Errorf("error building DL NAS transport message: %v", err)
					}
				}
				if ue.RanUe.UeContextRequest {
					ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
				} else {
					ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
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

		amfSelf := context.AMFSelf()
		amfSelf.ReAllocateGutiToUe(ctx, ue, operatorInfo.Guami)
		message.SendConfigurationUpdateCommand(ctx, ue, &context.ConfigurationUpdateCommandFlags{NeedGUTI: true})

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
