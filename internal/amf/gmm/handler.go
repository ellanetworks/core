// Copyright 2024 Ella Networks
package gmm

import (
	"bytes"
	ctxt "context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

func PlmnIDStringToModels(plmnIDStr string) models.PlmnID {
	var plmnID models.PlmnID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}

func HandleULNASTransport(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, ulNasTransport *nasMessage.ULNASTransport) error {
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	switch ulNasTransport.GetPayloadContainerType() {
	// TS 24.501 5.4.5.2.3 case a)
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return transport5GSMMessage(ctx, ue, anType, ulNasTransport)
	case nasMessage.PayloadContainerTypeSMS:
		return fmt.Errorf("PayloadContainerTypeSMS has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeLPP:
		return fmt.Errorf("PayloadContainerTypeLPP has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeSOR:
		return fmt.Errorf("PayloadContainerTypeSOR has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeUEPolicy:
		ue.GmmLog.Info("AMF Transfer UEPolicy To PCF")
	case nasMessage.PayloadContainerTypeUEParameterUpdate:
		ue.GmmLog.Info("AMF Transfer UEParameterUpdate To UDM")
		upuMac, err := nasConvert.UpuAckToModels(ulNasTransport.PayloadContainer.GetPayloadContainerContents())
		if err != nil {
			return err
		}
		ue.GmmLog.Debug("UpuMac in UPU ACK NAS Msg", zap.String("UpuMac", upuMac))
	case nasMessage.PayloadContainerTypeMultiplePayload:
		return fmt.Errorf("PayloadContainerTypeMultiplePayload has not been implemented yet in UL NAS TRANSPORT")
	}
	return nil
}

func getRequestTypeName(value uint8) string {
	switch value {
	case nasMessage.ULNASTransportRequestTypeInitialRequest:
		return "Initial Request"
	case nasMessage.ULNASTransportRequestTypeExistingPduSession:
		return "Existing PDU Session"
	case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest:
		return "Initial Emergency Request"
	case nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
		return "Existing Emergency PDU Session"
	case nasMessage.ULNASTransportRequestTypeModificationRequest:
		return "Modification Request"
	case nasMessage.ULNASTransportRequestTypeReserved:
		return "Reserved"
	default:
		return "Unknown"
	}
}

func getGsmMessageTypeName(msg *nas.Message) string {
	if msg.GsmMessage == nil {
		return "Not a 5GSM Message"
	}
	if msg.PDUSessionEstablishmentRequest != nil {
		return "5GSM PDU Session Establishment Request"
	}
	if msg.PDUSessionEstablishmentAccept != nil {
		return "5GSM PDU Session Establishment Accept"
	}
	if msg.PDUSessionEstablishmentReject != nil {
		return "5GSM PDU Session Establishment Reject"
	}
	if msg.PDUSessionAuthenticationCommand != nil {
		return "5GSM PDU Session Authentication Command"
	}
	if msg.PDUSessionAuthenticationComplete != nil {
		return "5GSM PDU Session Authentication Complete"
	}
	if msg.PDUSessionAuthenticationResult != nil {
		return "5GSM PDU Session Authentication Result"
	}
	if msg.PDUSessionModificationRequest != nil {
		return "5GSM PDU Session Modification Request"
	}
	if msg.PDUSessionModificationReject != nil {
		return "5GSM PDU Session Modification Reject"
	}
	if msg.PDUSessionModificationCommand != nil {
		return "5GSM PDU Session Modification Command"
	}
	if msg.PDUSessionModificationComplete != nil {
		return "5GSM PDU Session Modification Complete"
	}
	if msg.PDUSessionModificationCommandReject != nil {
		return "5GSM PDU Session Modification Command Reject"
	}
	if msg.PDUSessionReleaseRequest != nil {
		return "5GSM PDU Session Release Request"
	}
	if msg.PDUSessionReleaseReject != nil {
		return "5GSM PDU Session Release Reject"
	}
	if msg.PDUSessionReleaseCommand != nil {
		return "5GSM PDU Session Release Command"
	}
	if msg.PDUSessionReleaseComplete != nil {
		return "5GSM PDU Session Release Complete"
	}
	if msg.Status5GSM != nil {
		return "5GSM Status"
	}
	return "Unknown 5GSM Message"
}

func transport5GSMMessage(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, ulNasTransport *nasMessage.ULNASTransport) error {
	var pduSessionID int32
	smMessage := ulNasTransport.PayloadContainer.GetPayloadContainerContents()

	id := ulNasTransport.PduSessionID2Value
	if id == nil {
		return fmt.Errorf("pdu session id is nil")
	}
	pduSessionID = int32(id.GetPduSessionID2Value())
	logger.AmfLog.Warn("TO DELETE: UL NAS Transport PDU Session ID", zap.Int32("pduSessionID", pduSessionID))

	if ulNasTransport.OldPDUSessionID != nil {
		return fmt.Errorf("old pdu session id is not supported")
	}
	// case 1): looks up a PDU session routing context for the UE and the PDU session ID IE in case the Old PDU
	// session ID IE is not included
	smContext, smContextExist := ue.SmContextFindByPDUSessionID(pduSessionID)
	requestType := ulNasTransport.RequestType

	if requestType != nil {
		logger.AmfLog.Warn("TO DELETE: UL NAS Transport Request Type", zap.String("requestType", getRequestTypeName(requestType.GetRequestTypeValue())))
		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest:
			fallthrough
		case nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
			ue.GmmLog.Warn("Emergency PDU Session is not supported")
			err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.GmmLog.Info("sent downlink nas transport to UE")
			return nil
		}
	}

	// // note: we probably don't want to delete sm context,
	// if smContextExist && requestType != nil {
	// 	/* AMF releases context locally as this is duplicate pdu session */
	// 	if requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeInitialRequest {
	// 		logger.AmfLog.Warn("TO DELETE: UL NAS Transport Duplicate PDU Session ID, deleting existing SM Context", zap.Int32("pduSessionID", pduSessionID))
	// 		ue.SmContextList.Delete(pduSessionID)
	// 		smContextExist = false
	// 	}
	// }

	if !smContextExist {
		msg := new(nas.Message)
		if err := msg.PlainNasDecode(&smMessage); err != nil {
			ue.GmmLog.Error("Could not decode Nas message", zap.Error(err))
		}
		if msg.GsmMessage != nil && msg.GsmMessage.Status5GSM != nil {
			ue.GmmLog.Warn("SmContext doesn't exist, 5GSM Status message received from UE", zap.Any("cause", msg.GsmMessage.Status5GSM.Cause5GSM))
			return nil
		}
	}
	// AMF has a PDU session routing context for the PDU session ID and the UE
	if smContextExist {
		// case i) Request type IE is either not included
		if requestType == nil {
			return forward5GSMMessageToSMF(ctx, ue, anType, pduSessionID, smContext, smMessage)
		}

		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialRequest:

			smContext.StoreULNASTransport(ulNasTransport)
			//  perform a local release of the PDU session identified by the PDU session ID and shall request
			// the SMF to perform a local release of the PDU session
			ue.GmmLog.Warn("TO DELETE: Sending pdu session resumed to smf", zap.Int32("pduSessionID", pduSessionID))
			updateData := models.SmContextUpdateData{
				Release: false,
				Cause:   models.CausePduSessionResumed,
				SmContextStatusURI: fmt.Sprintf("%s/namf-callback/v1/smContextStatus/%s/%d",
					ue.ServingAMF.GetIPv4Uri(), ue.Guti, pduSessionID),
			}
			smContext.SetDuplicatedPduSessionID(true)
			response, err := consumer.SendUpdateSmContextRequest(ctx, smContext, updateData, smMessage, nil)
			if err != nil {
				return fmt.Errorf("couldn't send update sm context request: %v", err)
			}
			logger.AmfLog.Warn("TO DELETE: UL NAS Transport Initial Request with existing PDU Session ID, SMF Response", zap.Any("response", response))
			// if response == nil {
			// 	ue.GmmLog.Error("PDU Session can't be released in DUPLICATE_SESSION_ID case", zap.Int32("pduSessionID", pduSessionID))
			// 	err = gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			// 	if err != nil {
			// 		return fmt.Errorf("error sending downlink nas transport: %s", err)
			// 	}
			// 	ue.GmmLog.Info("sent downlink nas transport to UE")
			// } else {
			// 	smContext.SetUserLocation(ue.Location)
			// 	responseData := response.JSONData
			// 	n2Info := response.BinaryDataN2SmInformation
			// 	if n2Info != nil {
			// 		switch responseData.N2SmInfoType {
			// 		case models.N2SmInfoTypePduResRelCmd:
			// 			ue.GmmLog.Debug("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
			// 			list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
			// 			ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2Info)
			// 			err := ngap_message.SendPDUSessionResourceReleaseCommand(ue.RanUe[anType], nil, list)
			// 			if err != nil {
			// 				return fmt.Errorf("error sending pdu session resource release command: %s", err)
			// 			}
			// 			ue.GmmLog.Info("sent pdu session resource release command to UE")
			// 		}
			// 	}
			// }

		// case ii) AMF has a PDU session routing context, and Request type is "existing PDU session"
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			if ue.InAllowedNssai(smContext.Snssai(), anType) {
				return forward5GSMMessageToSMF(ctx, ue, anType, pduSessionID, smContext, smMessage)
			} else {
				ue.GmmLog.Error("S-NSSAI is not allowed for access type", zap.Any("snssai", smContext.Snssai()), zap.Any("accessType", anType), zap.Int32("pduSessionID", pduSessionID))
				err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}
				ue.GmmLog.Info("sent downlink nas transport to UE")
			}
		// other requestType: AMF forward the 5GSM message, and the PDU session ID IE towards the SMF identified
		// by the SMF ID of the PDU session routing context
		default:
			return forward5GSMMessageToSMF(ctx, ue, anType, pduSessionID, smContext, smMessage)
		}
	} else { // AMF does not have a PDU session routing context for the PDU session ID and the UE
		switch requestType.GetRequestTypeValue() {
		// case iii) if the AMF does not have a PDU session routing context for the PDU session ID and the UE
		// and the Request type IE is included and is set to "initial request"
		case nasMessage.ULNASTransportRequestTypeInitialRequest:
			var (
				snssai models.Snssai
				dnn    string
			)
			// A) AMF shall select an SMF

			// If the S-NSSAI IE is not included and the user's subscription context obtained from UDM. AMF shall
			// select a default snssai
			if ulNasTransport.SNSSAI != nil {
				snssai = util.SnssaiToModels(ulNasTransport.SNSSAI)
			} else {
				if allowedNssai, ok := ue.AllowedNssai[anType]; ok {
					snssai = *allowedNssai[0].AllowedSnssai
				} else {
					return fmt.Errorf("allowed nssai is not found for access type: %s in UE context", anType)
				}
			}

			if ulNasTransport.DNN != nil && ulNasTransport.DNN.GetLen() > 0 {
				dnn = string(ulNasTransport.DNN.GetDNN())
			} else {
				// if user's subscription context obtained from UDM does not contain the default DNN for the,
				// S-NSSAI, the AMF shall use a locally configured DNN as the DNN
				subscriber, err := ue.ServingAMF.DBInstance.GetSubscriber(ctxt.Background(), ue.Supi)
				if err != nil {
					return fmt.Errorf("couldn't get subscriber information: %v", err)
				}

				policy, err := ue.ServingAMF.DBInstance.GetPolicyByID(ctxt.Background(), subscriber.PolicyID)
				if err != nil {
					return fmt.Errorf("couldn't get policy information: %v", err)
				}

				dataNetwork, err := ue.ServingAMF.DBInstance.GetDataNetworkByID(ctxt.Background(), policy.DataNetworkID)
				if err != nil {
					return fmt.Errorf("couldn't get data network information: %v", err)
				}

				dnn = dataNetwork.Name
			}

			newSmContext := consumer.SelectSmf(ue, anType, pduSessionID, snssai, dnn)

			smContextRef, errResponse, err := consumer.SendCreateSmContextRequest(ctx, ue, newSmContext, smMessage)
			if err != nil {
				ue.GmmLog.Error("couldn't send create sm context request", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
			}

			if errResponse != nil {
				err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, errResponse.BinaryDataN1SmMessage, pduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}

				return fmt.Errorf("pdu session establishment request was rejected by SMF for pdu session id %d", pduSessionID)
			}

			newSmContext.SetSmContextRef(smContextRef)
			newSmContext.SetUserLocation(ue.Location)

			ue.StoreSmContext(pduSessionID, newSmContext)
			ue.GmmLog.Debug("Created sm context for pdu session", zap.Int32("pduSessionID", pduSessionID))

		case nasMessage.ULNASTransportRequestTypeModificationRequest:
			fallthrough
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			if ue.UeContextInSmfData != nil {
				// TS 24.501 5.4.5.2.5 case a) 3)
				pduSessionIDStr := fmt.Sprintf("%d", pduSessionID)
				if ueContextInSmf, ok := ue.UeContextInSmfData.PduSessions[pduSessionIDStr]; !ok {
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport: %s", err)
					}
					ue.GmmLog.Info("sent downlink nas transport to UE")
				} else {
					// TS 24.501 5.4.5.2.3 case a) 1) iv)
					smContext = context.NewSmContext(pduSessionID)
					smContext.SetAccessType(anType)
					smContext.SetDnn(ueContextInSmf.Dnn)
					smContext.SetPlmnID(*ueContextInSmf.PlmnID)
					ue.StoreSmContext(pduSessionID, smContext)
					return forward5GSMMessageToSMF(ctx, ue, anType, pduSessionID, smContext, smMessage)
				}
			} else {
				err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}
				ue.GmmLog.Info("sent downlink nas transport to UE")
			}
		default:
		}
	}
	return nil
}

func forward5GSMMessageToSMF(
	ctx ctxt.Context,
	ue *context.AmfUe,
	accessType models.AccessType,
	pduSessionID int32,
	smContext *context.SmContext,
	smMessage []byte,
) error {
	logger.AmfLog.Warn("TO DELETE: Forwarding 5GSM Message to SMF", zap.Int32("pduSessionID", pduSessionID), zap.String("messageType", getGsmMessageTypeName(&nas.Message{GsmMessage: &nas.GsmMessage{}})))
	smContextUpdateData := models.SmContextUpdateData{
		N1SmMsg: &models.RefToBinaryData{
			ContentID: "N1SmMsg",
		},
	}
	smContextUpdateData.Pei = ue.Pei
	smContextUpdateData.Gpsi = ue.Gpsi
	if !context.CompareUserLocation(ue.Location, smContext.UserLocation()) {
		smContextUpdateData.AddUeLocation = &ue.Location
	}

	if accessType != smContext.AccessType() {
		smContextUpdateData.AnType = accessType
	}

	response, err := consumer.SendUpdateSmContextRequest(ctx, smContext, smContextUpdateData, smMessage, nil)
	if err != nil {
		ue.GmmLog.Error("couldn't send update sm context request", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
		return nil
	} else if response != nil {
		// update SmContext in AMF
		smContext.SetAccessType(accessType)
		smContext.SetUserLocation(ue.Location)

		responseData := response.JSONData
		var n1Msg []byte
		n2SmInfo := response.BinaryDataN2SmInformation
		if response.BinaryDataN1SmMessage != nil {
			ue.GmmLog.Debug("Receive N1 SM Message from SMF")
			n1Msg, err = gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, response.BinaryDataN1SmMessage, uint8(pduSessionID), nil)
			if err != nil {
				return err
			}
		}

		if response.BinaryDataN2SmInformation != nil {
			ue.GmmLog.Debug("Receive N2 SM Information from SMF", zap.Any("type", responseData.N2SmInfoType))
			switch responseData.N2SmInfoType {
			case models.N2SmInfoTypePduResModReq:
				list := ngapType.PDUSessionResourceModifyListModReq{}
				ngap_message.AppendPDUSessionResourceModifyListModReq(&list, pduSessionID, n1Msg, n2SmInfo)
				err := ngap_message.SendPDUSessionResourceModifyRequest(ue.RanUe[accessType], list)
				if err != nil {
					return fmt.Errorf("error sending pdu session resource modify request: %s", err)
				}
				ue.GmmLog.Info("sent pdu session resource modify request to UE")
			case models.N2SmInfoTypePduResRelCmd:
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2SmInfo)
				err := ngap_message.SendPDUSessionResourceReleaseCommand(ue.RanUe[accessType], n1Msg, list)
				if err != nil {
					return fmt.Errorf("error sending pdu session resource release command: %s", err)
				}
				ue.GmmLog.Info("sent pdu session resource release command to UE")
			default:
				return fmt.Errorf("error N2 SM information type[%s]", responseData.N2SmInfoType)
			}
		} else if n1Msg != nil {
			ue.GmmLog.Debug("AMF forward Only N1 SM Message to UE")
			err := ngap_message.SendDownlinkNasTransport(ue.RanUe[accessType], n1Msg, nil)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.GmmLog.Info("sent downlink nas transport to UE")
		}
	}
	return nil
}

func getRegistrationType5GSName(regType5Gs uint8) string {
	switch regType5Gs {
	case nasMessage.RegistrationType5GSInitialRegistration:
		return "Initial Registration"
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		return "Mobility Registration Updating"
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		return "Periodic Registration Updating"
	case nasMessage.RegistrationType5GSEmergencyRegistration:
		return "Emergency Registration"
	case nasMessage.RegistrationType5GSReserved:
		return "Reserved"
	default:
		return "Unknown"
	}
}

// Handle cleartext IEs of Registration Request, which cleattext IEs defined in TS 24.501 4.4.6
func HandleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, procedureCode int64, registrationRequest *nasMessage.RegistrationRequest) error {
	var guamiFromUeGuti models.Guami
	amfSelf := context.AMFSelf()

	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	if ue.RanUe[anType] == nil {
		return fmt.Errorf("RanUe is nil")
	}

	// MacFailed is set if plain Registration Request message received with GUTI/SUCI or
	// integrity protected Registration Reguest message received but mac verification Failed
	if ue.MacFailed {
		amfSelf.ReAllocateGutiToUe(ctx, ue)
		ue.SecurityContextAvailable = false
	}

	ue.SetOnGoing(anType, &context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedureRegistration,
	})

	if ue.T3513 != nil {
		ue.T3513.Stop()
		ue.T3513 = nil // clear the timer
	}
	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	// TS 24.501 8.2.6.21: if the UE is sending a REGISTRATION REQUEST message as an initial NAS message,
	// the UE has a valid 5G NAS security context and the UE needs to send non-cleartext IEs
	// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS message
	// container IE, the UE shall set the security header type of the initial NAS message to "integrity protected"
	if registrationRequest.NASMessageContainer != nil {
		contents := registrationRequest.NASMessageContainer.GetNASMessageContainerContents()

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
			if messageType != nas.MsgTypeRegistrationRequest {
				return fmt.Errorf("expected registration request, got %d", messageType)
			}
			// TS 24.501 4.4.6: The AMF shall consider the NAS message that is obtained from the NAS message container
			// IE as the initial NAS message that triggered the procedure
			registrationRequest = m.RegistrationRequest
		}
		// TS 33.501 6.4.6 step 3: if the initial NAS message was protected but did not pass the integrity check
		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	ue.RegistrationRequest = registrationRequest
	ue.RegistrationType5GS = registrationRequest.NgksiAndRegistrationType5GS.GetRegistrationType5GS()
	regName := getRegistrationType5GSName(ue.RegistrationType5GS)
	ue.GmmLog.Debug("Received Registration Request", zap.String("registrationType", regName), zap.Int64("procedureCode", procedureCode))

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSReserved {
		ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	}

	mobileIdentity5GSContents := registrationRequest.MobileIdentity5GS.GetMobileIdentity5GSContents()
	ue.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch ue.IdentityTypeUsedForRegistration { // get type of identity
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.GmmLog.Debug("No Identity")
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnID string
		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		guamiFromUeGutiTmp, guti := util.GutiToString(mobileIdentity5GSContents)
		guamiFromUeGuti = guamiFromUeGutiTmp
		ue.Guti = guti
		ue.GmmLog.Debug("GUTI", zap.String("guti", guti))

		guamiList := context.GetServedGuamiList(ctx)
		servedGuami := guamiList[0]
		if reflect.DeepEqual(guamiFromUeGuti, servedGuami) {
			ue.ServingAmfChanged = false
		} else {
			ue.GmmLog.Debug("Serving AMF has changed but 5G-Core is not supporting for now")
			ue.ServingAmfChanged = false
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imei
		ue.GmmLog.Debug("PEI", zap.String("imei", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imeisv
		ue.GmmLog.Debug("PEI", zap.String("imeisv", imeisv))
	}

	// NgKsi: TS 24.501 9.11.3.32
	switch registrationRequest.NgksiAndRegistrationType5GS.GetTSC() {
	case nasMessage.TypeOfSecurityContextFlagNative:
		ue.NgKsi.Tsc = models.ScTypeNative
	case nasMessage.TypeOfSecurityContextFlagMapped:
		ue.NgKsi.Tsc = models.ScTypeMapped
	}
	ue.NgKsi.Ksi = int32(registrationRequest.NgksiAndRegistrationType5GS.GetNasKeySetIdentifiler())
	if ue.NgKsi.Tsc == models.ScTypeNative && ue.NgKsi.Ksi != 7 {
	} else {
		ue.NgKsi.Tsc = models.ScTypeNative
		ue.NgKsi.Ksi = 0
	}

	// Copy UserLocation from ranUe
	ue.Location = ue.RanUe[anType].Location
	ue.Tai = ue.RanUe[anType].Tai

	// Check TAI
	supportTaiList := context.GetSupportTaiList(ctx)
	taiList := make([]models.Tai, len(supportTaiList))
	copy(taiList, supportTaiList)
	for i := range taiList {
		tac, err := util.TACConfigToModels(taiList[i].Tac)
		if err != nil {
			logger.AmfLog.Warn("failed to convert TAC to models.Tac", zap.Error(err), zap.String("tac", taiList[i].Tac))
			continue
		}
		taiList[i].Tac = tac
	}
	if !context.InTaiList(ue.Tai, taiList) {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMTrackingAreaNotAllowed, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return fmt.Errorf("registration Reject[Tracking area not allowed]")
	}

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSInitialRegistration && registrationRequest.UESecurityCapability == nil {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMProtocolErrorUnspecified, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return fmt.Errorf("registration request does not contain UE security capability for initial registration")
	}

	if registrationRequest.UESecurityCapability != nil {
		ue.UESecurityCapability = *registrationRequest.UESecurityCapability
	}

	if ue.ServingAmfChanged {
		ue.TargetAmfURI = amfSelf.GetIPv4Uri()
		logger.AmfLog.Debug("Serving AMF has changed - Unsupported", zap.String("targetAmfUri", ue.TargetAmfURI))
	}

	return nil
}

func IdentityVerification(ue *context.AmfUe) bool {
	return ue.Supi != "" || len(ue.Suci) != 0
}

func HandleInitialRegistration(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType) error {
	amfSelf := context.AMFSelf()

	ue.ClearRegistrationData()

	// update Kgnb/Kn3iwf
	ue.UpdateSecurityContext(anType)

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if len(ue.SubscribedNssai) == 0 {
		getSubscribedNssai(ctx, ue)
	}

	if err := handleRequestedNssai(ctx, ue, anType); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	}

	if len(ue.AllowedNssai[anType]) == 0 {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMM5GSServicesNotAllowed, "")
		if err != nil {
			ue.GmmLog.Error("error sending registration reject", zap.Error(err))
		}
		err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			ue.GmmLog.Error("error sending ue context release command", zap.Error(err))
		}
		ue.Remove()
		return fmt.Errorf("no allowed nssai")
	}

	storeLastVisitedRegisteredTAI(ue, ue.RegistrationRequest.LastVisitedRegisteredTAI)

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.GmmLog.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", ue.RegistrationRequest.MICOIndication.GetRAAI()))
	}

	negotiateDRXParameters(ue, ue.RegistrationRequest.RequestedDRXParameters)

	if ue.ServingAmfChanged || ue.State[models.AccessTypeNon3GPPAccess].Is(context.Registered) ||
		!ue.SubscriptionDataValid {
		if err := communicateWithUDM(ctx, ue, anType); err != nil {
			return err
		}
	}

	err := consumer.AMPolicyControlCreate(ctx, ue, anType)
	if err != nil {
		ue.GmmLog.Error("AM Policy Control Create Error", zap.Error(err))
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMM5GSServicesNotAllowed, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return err
	}

	// Service Area Restriction are applicable only to 3GPP access
	if anType == models.AccessType3GPPAccess {
		if ue.AmPolicyAssociation != nil && ue.AmPolicyAssociation.ServAreaRes != nil {
			servAreaRes := ue.AmPolicyAssociation.ServAreaRes
			if servAreaRes.RestrictionType == models.RestrictionTypeAllowedAreas {
				numOfallowedTAs := 0
				for _, area := range servAreaRes.Areas {
					numOfallowedTAs += len(area.Tacs)
				}
			}
		}
	}

	amfSelf.AllocateRegistrationArea(ctx, ue, anType)
	ue.GmmLog.Debug("use original GUTI", zap.String("guti", ue.Guti))

	assignLadnInfo(ue, anType)

	amfSelf.AddAmfUeToUePool(ue, ue.Supi)
	ue.T3502Value = amfSelf.T3502Value
	if anType == models.AccessType3GPPAccess {
		ue.T3512Value = amfSelf.T3512Value
	} else {
		ue.Non3gppDeregistrationTimerValue = amfSelf.Non3gppDeregistrationTimerValue
	}

	if anType == models.AccessType3GPPAccess {
		err := gmm_message.SendRegistrationAccept(ctx, ue, anType, nil, nil, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("error sending GMM registration accept: %v", err)
		}
		ue.GmmLog.Info("Sent GMM registration accept to UE")
	} else {
		// TS 23.502 4.12.2.2 10a ~ 13: if non-3gpp, AMF should send initial context setup request to N3IWF first,
		// and send registration accept after receiving initial context setup response
		err := ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nil, nil, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %v", err)
		}
		ue.GmmLog.Info("Sent NGAP initial context setup request to N3IWF")
		registrationAccept, err := gmm_message.BuildRegistrationAccept(ctx, ue, anType, nil, nil, nil, nil)
		if err != nil {
			ue.GmmLog.Error("Build Registration Accept", zap.Error(err))
			return nil
		}
		ue.RegistrationAcceptForNon3GPPAccess = registrationAccept
	}
	return nil
}

func HandleMobilityAndPeriodicRegistrationUpdating(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType) error {
	ue.GmmLog.Debug("Handle MobilityAndPeriodicRegistrationUpdating")

	ue.DerivateAnKey(anType)

	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.UpdateType5GS != nil {
		if ue.RegistrationRequest.UpdateType5GS.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.UeRadioCapability = ""
			ue.UeRadioCapabilityForPaging = nil
		}
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if len(ue.SubscribedNssai) == 0 {
		getSubscribedNssai(ctx, ue)
	}

	if err := handleRequestedNssai(ctx, ue, anType); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	} else {
		if ue.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
			err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMProtocolErrorUnspecified, "")
			if err != nil {
				return fmt.Errorf("error sending registration reject: %v", err)
			}
			return fmt.Errorf("Capability5GMM is nil")
		}
	}

	storeLastVisitedRegisteredTAI(ue, ue.RegistrationRequest.LastVisitedRegisteredTAI)

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.GmmLog.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", ue.RegistrationRequest.MICOIndication.GetRAAI()))
	}

	negotiateDRXParameters(ue, ue.RegistrationRequest.RequestedDRXParameters)

	if len(ue.Pei) == 0 {
		err := gmm_message.SendIdentityRequest(ue.RanUe[anType], nasMessage.MobileIdentity5GSTypeImei)
		if err != nil {
			return fmt.Errorf("error sending identity request: %v", err)
		}
		ue.GmmLog.Info("sent identity request to UE")
		return nil
	}

	if ue.ServingAmfChanged || ue.State[models.AccessTypeNon3GPPAccess].Is(context.Registered) ||
		!ue.SubscriptionDataValid {
		if err := communicateWithUDM(ctx, ue, anType); err != nil {
			return err
		}
	}

	var reactivationResult *[16]bool
	var errPduSessionID, errCause []uint8
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}
	suList := ngapType.PDUSessionResourceSetupListSUReq{}

	if ue.RegistrationRequest.PDUSessionStatus != nil {
		logger.AmfLog.Warn("TO DELETE: Handle PDU Session Status in Registration Request", zap.Any("pduSessionStatus", ue.RegistrationRequest.PDUSessionStatus))
	}

	if ue.RegistrationRequest.UplinkDataStatus != nil {
		logger.AmfLog.Warn("TO DELETE: Handle Uplink Data Status in Registration Request", zap.Any("uplinkDataStatus", ue.RegistrationRequest.UplinkDataStatus))
		uplinkDataPsi := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)
		allowReEstablishPduSession := true

		// determines that the UE is in non-allowed area or is not in allowed area
		if ue.AmPolicyAssociation != nil && ue.AmPolicyAssociation.ServAreaRes != nil {
			switch ue.AmPolicyAssociation.ServAreaRes.RestrictionType {
			case models.RestrictionTypeAllowedAreas:
				allowReEstablishPduSession = context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
			case models.RestrictionTypeNotAllowedAreas:
				allowReEstablishPduSession = !context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
			}
		}

		if !allowReEstablishPduSession {
			for pduSessionID, hasUplinkData := range uplinkDataPsi {
				if hasUplinkData {
					errPduSessionID = append(errPduSessionID, uint8(pduSessionID))
					errCause = append(errCause, nasMessage.Cause5GMMRestrictedServiceArea)
				}
			}
		} else {
			for idx, hasUplinkData := range uplinkDataPsi {
				pduSessionID := int32(idx)
				if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
					// uplink data are pending for the corresponding PDU session identity
					if hasUplinkData && smContext.AccessType() == models.AccessType3GPPAccess {
						response, err := consumer.SendUpdateSmContextActivateUpCnxState(ctx, ue, smContext, anType)
						if response == nil {
							reactivationResult[pduSessionID] = true
							errPduSessionID = append(errPduSessionID, uint8(pduSessionID))
							cause := nasMessage.Cause5GMMProtocolErrorUnspecified
							errCause = append(errCause, cause)

							if err != nil {
								ue.GmmLog.Error("Update SmContext Error", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
							}
						} else {
							if ue.RanUe[anType].UeContextRequest {
								ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionID,
									smContext.Snssai(), response.BinaryDataN1SmMessage, response.BinaryDataN2SmInformation)
							} else {
								ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionID,
									smContext.Snssai(), response.BinaryDataN1SmMessage, response.BinaryDataN2SmInformation)
							}
						}
					}
				}
			}
		}
	}

	var pduSessionStatus *[16]bool
	if ue.RegistrationRequest.PDUSessionStatus != nil {
		pduSessionStatus = new([16]bool)
		psiArray := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.PDUSessionStatus.Buffer)
		logger.AmfLog.Warn("TO DELETE: PDU Session Status from UE", zap.Any("psiArray", psiArray))
		for psi := 1; psi <= 15; psi++ {
			pduSessionID := int32(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				logger.AmfLog.Warn("TO DELETE: Found SM Context for PDU Session ID", zap.Int32("pduSessionID", pduSessionID), zap.Bool("psiArray", psiArray[psi]), zap.String("accessType", string(smContext.AccessType())))
				if !psiArray[psi] && smContext.AccessType() == anType {
					logger.AmfLog.Warn("TO DELETE: Releasing SM Context for PDU Session ID", zap.Int32("pduSessionID", pduSessionID))
					err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
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

	logger.AmfLog.Warn("TO DELETE: PDU Session Status after processing", zap.Any("pduSessionStatus", pduSessionStatus))

	if ue.RegistrationRequest.AllowedPDUSessionStatus != nil {
		logger.AmfLog.Warn("TO DELETE: Handle Allowed PDU Session Status in Registration Request", zap.Any("allowedPduSessionStatus", ue.RegistrationRequest.AllowedPDUSessionStatus))
		allowedPsis := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.AllowedPDUSessionStatus.Buffer)
		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message.Request.JSONData
			n1Msg := ue.N1N2Message.Request.BinaryDataN1Message
			n2Info := ue.N1N2Message.Request.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := gmm_message.BuildRegistrationAccept(ctx, ue, anType, pduSessionStatus,
						reactivationResult, errPduSessionID, errCause)
					if err != nil {
						return err
					}
					err = ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nasPdu, suList)
					if err != nil {
						return fmt.Errorf("error sending pdu session resource setup request: %v", err)
					}
					ue.GmmLog.Info("Sent NGAP pdu session resource setup request")
				} else {
					logger.AmfLog.Warn("TO DELETE: Send Registration Accept without PDU Session Resource Setup Request")
					err := gmm_message.SendRegistrationAccept(ctx, ue, anType, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList)
					if err != nil {
						return fmt.Errorf("error sending GMM registration accept: %v", err)
					}
					ue.GmmLog.Info("Sent GMM registration accept")
				}
				switch requestData.N1MessageContainer.N1MessageClass {
				case models.N1MessageClassSM:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassLPP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassSMS:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassUPDP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				}
				ue.N1N2Message = nil
				return nil
			}

			smInfo := requestData.N2InfoContainer.SmInfo
			smContext, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				ue.N1N2Message = nil
				return fmt.Errorf("pdu Session Id does not Exists")
			}

			if smContext.AccessType() == models.AccessTypeNon3GPPAccess {
				if reactivationResult == nil {
					reactivationResult = new([16]bool)
				}
				if allowedPsis[requestData.PduSessionID] {
					response, err := consumer.SendUpdateSmContextChangeAccessType(ctx, ue, smContext, true)
					if err != nil {
						return err
					} else if response == nil {
						reactivationResult[requestData.PduSessionID] = true
						errPduSessionID = append(errPduSessionID, uint8(requestData.PduSessionID))
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else {
						smContext.SetUserLocation(ue.Location)
						smContext.SetAccessType(models.AccessType3GPPAccess)
						if response.BinaryDataN2SmInformation != nil &&
							response.JSONData.N2SmInfoType == models.N2SmInfoTypePduResSetupReq {
							ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID,
								smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
						}
					}
				} else {
					ue.GmmLog.Warn("UE was reachable but did not accept to re-activate the PDU Session", zap.Int32("pduSessionID", requestData.PduSessionID))
				}
			} else if smInfo.N2InfoContent.NgapIeType == models.NgapIeTypePduResSetupReq {
				var nasPdu []byte
				var err error
				if n1Msg != nil {
					pduSessionID := uint8(smInfo.PduSessionID)
					nasPdu, err = gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
					if err != nil {
						return err
					}
				}
				omecSnssai := models.Snssai{
					Sst: smInfo.SNssai.Sst,
					Sd:  smInfo.SNssai.Sd,
				}
				ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, smInfo.PduSessionID,
					omecSnssai, nasPdu, n2Info)
			}
		}
	}

	if ue.LocationChanged && ue.RequestTriggerLocationChange {
		updateReq := models.PolicyAssociationUpdateRequest{}
		updateReq.Triggers = append(updateReq.Triggers, models.RequestTriggerLocCh)
		updateReq.UserLoc = &ue.Location
		err := consumer.AMPolicyControlUpdate(ctx, ue, updateReq)
		if err != nil {
			ue.GmmLog.Error("AM Policy Control Update Error", zap.Error(err))
		}
		ue.LocationChanged = false
	}

	amfSelf.AllocateRegistrationArea(ctx, ue, anType)
	assignLadnInfo(ue, anType)

	if ue.RanUe[anType].UeContextRequest {
		if anType == models.AccessType3GPPAccess {
			err := gmm_message.SendRegistrationAccept(ctx, ue, anType, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList)
			if err != nil {
				return fmt.Errorf("error sending GMM registration accept: %v", err)
			}
			ue.GmmLog.Info("Sent GMM registration accept")
		} else {
			err := ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nil, &ctxList, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Info("Sent NGAP initial context setup request")
			registrationAccept, err := gmm_message.BuildRegistrationAccept(ctx, ue, anType,
				pduSessionStatus, reactivationResult, errPduSessionID, errCause)
			if err != nil {
				ue.GmmLog.Error("Build Registration Accept", zap.Error(err))
				return nil
			}
			ue.RegistrationAcceptForNon3GPPAccess = registrationAccept
		}
		return nil
	} else {
		nasPdu, err := gmm_message.BuildRegistrationAccept(ctx, ue, anType, pduSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error building registration accept: %v", err)
		}
		if len(suList.List) != 0 {
			err := ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nasPdu, suList)
			if err != nil {
				return fmt.Errorf("error sending pdu session resource setup request: %v", err)
			}
			ue.GmmLog.Info("Sent NGAP pdu session resource setup request")
		} else {
			err := ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasPdu, nil)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}
			ue.GmmLog.Info("sent downlink nas transport message")
		}
		return nil
	}
}

// TS 23.502 4.2.2.2.2 step 1
// If available, the last visited TAI shall be included in order to help the AMF produce Registration Area for the UE
func storeLastVisitedRegisteredTAI(ue *context.AmfUe, lastVisitedRegisteredTAI *nasType.LastVisitedRegisteredTAI) {
	if lastVisitedRegisteredTAI != nil {
		plmnID := nasConvert.PlmnIDToString(lastVisitedRegisteredTAI.Octet[1:4])
		nasTac := lastVisitedRegisteredTAI.GetTAC()
		tac := hex.EncodeToString(nasTac[:])

		tai := models.Tai{
			PlmnID: &models.PlmnID{
				Mcc: plmnID[:3],
				Mnc: plmnID[3:],
			},
			Tac: tac,
		}

		ue.LastVisitedRegisteredTai = tai
		ue.GmmLog.Debug("Ue Last Visited Registered Tai", zap.String("plmnID", plmnID), zap.String("tac", tac))
	}
}

func negotiateDRXParameters(ue *context.AmfUe, requestedDRXParameters *nasType.RequestedDRXParameters) {
	if requestedDRXParameters != nil {
		switch requestedDRXParameters.GetDRXValue() {
		case nasMessage.DRXcycleParameterT32:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT32
		case nasMessage.DRXcycleParameterT64:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT64
		case nasMessage.DRXcycleParameterT128:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT128
		case nasMessage.DRXcycleParameterT256:
			ue.UESpecificDRX = nasMessage.DRXcycleParameterT256
		case nasMessage.DRXValueNotSpecified:
			fallthrough
		default:
			ue.UESpecificDRX = nasMessage.DRXValueNotSpecified
		}
	}
}

func communicateWithUDM(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType) error {
	err := consumer.UeCmRegistration(ctx, ue, accessType, true)
	if err != nil {
		ue.GmmLog.Error("UECM_Registration Error", zap.Error(err))
	}

	err = consumer.SDMGetAmData(ctx, ue)
	if err != nil {
		return fmt.Errorf("error getting am data: %v", err)
	}

	err = consumer.SDMGetSmfSelectData(ctx, ue)
	if err != nil {
		return fmt.Errorf("error getting smf selection data: %v", err)
	}

	err = consumer.SDMGetUeContextInSmfData(ctx, ue)
	if err != nil {
		return fmt.Errorf("error getting ue context in smf data: %v", err)
	}

	ue.SubscriptionDataValid = true
	return nil
}

func getSubscribedNssai(ctx ctxt.Context, ue *context.AmfUe) {
	err := consumer.SDMGetSliceSelectionSubscriptionData(ctx, ue)
	if err != nil {
		ue.GmmLog.Error("error getting slice selection subscription data", zap.Error(err))
	}
}

// TS 23.502 4.2.2.2.3 Registration with AMF Re-allocation
func handleRequestedNssai(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType) error {
	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.RequestedNSSAI != nil {
		requestedNssai, err := util.RequestedNssaiToModels(ue.RegistrationRequest.RequestedNSSAI)
		if err != nil {
			return fmt.Errorf("failed to decode requested NSSAI[%s]", err)
		}

		needSliceSelection := false
		var newAllowed []models.AllowedSnssai

		for _, requestedSnssai := range requestedNssai {
			if ue.InSubscribedNssai(requestedSnssai.ServingSnssai) {
				allowedSnssai := models.AllowedSnssai{
					AllowedSnssai: &models.Snssai{
						Sst: requestedSnssai.ServingSnssai.Sst,
						Sd:  requestedSnssai.ServingSnssai.Sd,
					},
					MappedHomeSnssai: requestedSnssai.HomeSnssai,
				}
				newAllowed = append(newAllowed, allowedSnssai)
			} else {
				needSliceSelection = true
				break
			}
		}
		ue.AllowedNssai[anType] = newAllowed

		if needSliceSelection {
			// Step 4
			err := consumer.NSSelectionGetForRegistration(ue, requestedNssai)
			if err != nil {
				err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMProtocolErrorUnspecified, "")
				if err != nil {
					return fmt.Errorf("error sending registration reject: %v", err)
				}
				ue.GmmLog.Info("sent registration reject to UE")
				return fmt.Errorf("failed to get network slice selection: %s", err)
			}

			// Guillaume: I'm not sure if what we have here is the right thing to do
			// As we removed the NRF, we don't search for other AMF's anymore and we hardcode the
			// target AMF to the AMF's own address.
			// It's possible we need to change this whole block to the following:
			//  allowedNssaiNgap := ngapConvert.AllowedNssaiToNgap(ue.AllowedNssai[anType])
			//	ngap_message.SendRerouteNasRequest(ue, anType, nil, ue.RanUe[anType].InitialUEMessage, &allowedNssaiNgap)
			ue.TargetAmfURI = amfSelf.GetIPv4Uri()

			var n1Message bytes.Buffer
			ue.RegistrationRequest.EncodeRegistrationRequest(&n1Message)
			return nil
		}
	}

	// if registration request has no requested nssai, or non of snssai in requested nssai is permitted by nssf
	// then use ue subscribed snssai which is marked as default as allowed nssai
	if len(ue.AllowedNssai[anType]) == 0 {
		var newAllowed []models.AllowedSnssai
		for _, snssai := range ue.SubscribedNssai {
			if snssai.DefaultIndication {
				if amfSelf.InPlmnSupport(ctx, *snssai.SubscribedSnssai) {
					allowedSnssai := models.AllowedSnssai{
						AllowedSnssai: snssai.SubscribedSnssai,
					}
					newAllowed = append(newAllowed, allowedSnssai)
				}
			}
		}
		ue.AllowedNssai[anType] = newAllowed
	}
	return nil
}

func assignLadnInfo(ue *context.AmfUe, accessType models.AccessType) {
	amfSelf := context.AMFSelf()

	ue.LadnInfo = nil
	if ue.RegistrationRequest.LADNIndication != nil {
		ue.LadnInfo = make([]context.LADN, 0)
		// request for LADN information
		if ue.RegistrationRequest.LADNIndication.GetLen() == 0 {
			if ue.HasWildCardSubscribedDNN() {
				for _, ladn := range amfSelf.LadnPool {
					if ue.TaiListInRegistrationArea(ladn.TaiLists, accessType) {
						ue.LadnInfo = append(ue.LadnInfo, *ladn)
					}
				}
			} else {
				for _, snssaiInfos := range ue.SmfSelectionData.SubscribedSnssaiInfos {
					for _, dnnInfo := range snssaiInfos.DnnInfos {
						if ladn, ok := amfSelf.LadnPool[dnnInfo.Dnn]; ok { // check if this dnn is a ladn
							if ue.TaiListInRegistrationArea(ladn.TaiLists, accessType) {
								ue.LadnInfo = append(ue.LadnInfo, *ladn)
							}
						}
					}
				}
			}
		} else {
			requestedLadnList := nasConvert.LadnToModels(ue.RegistrationRequest.LADNIndication.GetLADNDNNValue())
			for _, requestedLadn := range requestedLadnList {
				if ladn, ok := amfSelf.LadnPool[requestedLadn]; ok {
					if ue.TaiListInRegistrationArea(ladn.TaiLists, accessType) {
						ue.LadnInfo = append(ue.LadnInfo, *ladn)
					}
				}
			}
		}
	} else if ue.SmfSelectionData != nil {
		for _, snssaiInfos := range ue.SmfSelectionData.SubscribedSnssaiInfos {
			for _, dnnInfo := range snssaiInfos.DnnInfos {
				if dnnInfo.Dnn != "*" {
					if ladn, ok := amfSelf.LadnPool[dnnInfo.Dnn]; ok {
						if ue.TaiListInRegistrationArea(ladn.TaiLists, accessType) {
							ue.LadnInfo = append(ue.LadnInfo, *ladn)
						}
					}
				}
			}
		}
	}
}

func HandleIdentityResponse(ue *context.AmfUe, identityResponse *nasMessage.IdentityResponse) error {
	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	mobileIdentityContents := identityResponse.MobileIdentity.GetMobileIdentityContents()
	switch nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) { // get type of identity
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnID string
		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentityContents)
		ue.PlmnID = PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		_, guti := nasConvert.GutiToString(mobileIdentityContents)
		ue.Guti = guti
		ue.GmmLog.Debug("get GUTI", zap.String("guti", guti))
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		sTmsi := hex.EncodeToString(mobileIdentityContents[1:])
		if tmp, err := strconv.ParseInt(sTmsi[4:], 10, 32); err != nil {
			return err
		} else {
			ue.Tmsi = int32(tmp)
		}
		ue.GmmLog.Debug("get 5G-S-TMSI", zap.String("5G-S-TMSI", sTmsi))
	case nasMessage.MobileIdentity5GSTypeImei:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		imei := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imei
		ue.GmmLog.Debug("get PEI", zap.String("PEI", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		imeisv := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imeisv
		ue.GmmLog.Debug("get PEI", zap.String("PEI", imeisv))
	}
	return nil
}

// TS 24501 5.6.3.2
func HandleNotificationResponse(ctx ctxt.Context, ue *context.AmfUe, notificationResponse *nasMessage.NotificationResponse) error {
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	if notificationResponse != nil && notificationResponse.PDUSessionStatus != nil {
		psiArray := nasConvert.PSIToBooleanArray(notificationResponse.PDUSessionStatus.Buffer)
		for psi := 1; psi <= 15; psi++ {
			pduSessionID := int32(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
					err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
					if err != nil {
						return fmt.Errorf("failed to release sm context: %s", err)
					}
				}
			}
		}
	}
	return nil
}

func HandleConfigurationUpdateComplete(ue *context.AmfUe, configurationUpdateComplete *nasMessage.ConfigurationUpdateComplete) error {
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	return nil
}

func AuthenticationProcedure(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType) (bool, error) {
	// Check whether UE has SUCI and SUPI
	if IdentityVerification(ue) {
		ue.GmmLog.Debug("UE has SUCI / SUPI")
		if ue.SecurityContextIsValid() {
			ue.GmmLog.Debug("UE has a valid security context - skip the authentication procedure")
			return true, nil
		}
	} else {
		// Request UE's SUCI by sending identity request
		err := gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
		if err != nil {
			return false, fmt.Errorf("error sending identity request: %v", err)
		}
		ue.GmmLog.Info("sent identity request")
		return false, nil
	}

	response, err := consumer.SendUEAuthenticationAuthenticateRequest(ctx, ue, nil)
	if err != nil {
		return false, fmt.Errorf("Authentication procedure failed: %s", err)
	}
	ue.AuthenticationCtx = response
	ue.ABBA = []uint8{0x00, 0x00} // set ABBA value as described at TS 33.501 Annex A.7.1

	err = gmm_message.SendAuthenticationRequest(ue.RanUe[accessType])
	if err != nil {
		return false, fmt.Errorf("error sending authentication request: %v", err)
	}

	return false, nil
}

func NetworkInitiatedDeregistrationProcedure(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType) (err error) {
	anType := AnTypeToNas(accessType)
	if ue.CmConnect(accessType) && ue.State[accessType].Is(context.Registered) {
		// setting reregistration required flag to true
		err := gmm_message.SendDeregistrationRequest(ue.RanUe[accessType], anType, true, 0)
		if err != nil {
			return fmt.Errorf("error sending deregistration request: %v", err)
		}
		ue.GmmLog.Info("sent deregistration request")
	} else {
		SetDeregisteredState(ue, anType)
	}

	ue.SmContextList.Range(func(key, value interface{}) bool {
		smContext := value.(*context.SmContext)

		if smContext.AccessType() == accessType {
			ue.GmmLog.Info("Sending SmContext Release Request to SMF", zap.Any("slice", smContext.Snssai()), zap.String("dnn", smContext.Dnn()))
			err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
			if err != nil {
				ue.GmmLog.Error("Release SmContext Error", zap.Error(err))
			}
		}
		return true
	})

	if ue.AmPolicyAssociation != nil {
		terminateAmPolicyAssocaition := true
		switch accessType {
		case models.AccessType3GPPAccess:
			terminateAmPolicyAssocaition = ue.State[models.AccessTypeNon3GPPAccess].Is(context.Deregistered)
		case models.AccessTypeNon3GPPAccess:
			terminateAmPolicyAssocaition = ue.State[models.AccessType3GPPAccess].Is(context.Deregistered)
		}

		if terminateAmPolicyAssocaition {
			err = consumer.AMPolicyControlDelete(ctx, ue)
			if err != nil {
				ue.GmmLog.Error("AM Policy Control Delete Error", zap.Error(err))
			}
			ue.GmmLog.Info("deleted AM Policy Association")
		}
	}
	// if ue is not connected mode, removing UE Context
	if !ue.State[accessType].Is(context.Registered) {
		if ue.CmConnect(accessType) {
			err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType3GPPAccess], context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		} else {
			ue.Remove()
			ue.GmmLog.Info("removed ue context")
		}
	}
	return err
}

func getServiceRequestTypeString(serviceType uint8) string {
	switch serviceType {
	case nasMessage.ServiceTypeSignalling:
		return "signalling"
	case nasMessage.ServiceTypeData:
		return "data"
	case nasMessage.ServiceTypeMobileTerminatedServices:
		return "mobile terminated services"
	case nasMessage.ServiceTypeEmergencyServices:
		return "emergency services"
	case nasMessage.ServiceTypeEmergencyServicesFallback:
		return "emergency services fallback"
	case nasMessage.ServiceTypeHighPriorityAccess:
		return "high priority access"
	default:
		return "unknown service type"
	}
}

const psiArraySize = 16

func getPDUSessionStatus(ue *context.AmfUe, anType models.AccessType) *[psiArraySize]bool {
	var pduStatusResult [psiArraySize]bool
	ue.SmContextList.Range(func(key, value any) bool {
		pduSessionID := key.(int32)
		smContext := value.(*context.SmContext)

		if smContext.AccessType() != anType {
			return true
		}

		pduStatusResult[pduSessionID] = true
		return true
	})
	return &pduStatusResult
}

// TS 24501 5.6.1
func HandleServiceRequest(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, serviceRequest *nasMessage.ServiceRequest) error {
	logger.AmfLog.Warn("TO DELETE: Service Request", zap.String("type", getServiceRequestTypeString(serviceRequest.GetServiceTypeValue())))

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
	if procedure := ue.GetOnGoing(anType).Procedure; procedure == context.OnGoingProcedurePaging {
		ue.SetOnGoing(anType, &context.OnGoingProcedureWithPrio{
			Procedure: context.OnGoingProcedureNothing,
		})
	} else if procedure != context.OnGoingProcedureNothing {
		ue.GmmLog.Warn("UE should not in OnGoing", zap.Any("procedure", procedure))
	}

	// Send Authtication / Security Procedure not support
	// Rejecting ServiceRequest if it is received in Deregistered State
	if !ue.SecurityContextIsValid() || ue.State[anType].Current() == context.Deregistered {
		ue.GmmLog.Warn("No security context", zap.String("supi", ue.Supi))
		err := gmm_message.SendServiceReject(ue.RanUe[anType], nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.GmmLog.Info("sent service reject")
		err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		ue.GmmLog.Info("sent ue context release command")
		return nil
	}

	// TS 24.501 8.2.6.21: if the UE is sending a REGISTRATION REQUEST message as an initial NAS message,
	// the UE has a valid 5G NAS security context and the UE needs to send non-cleartext IEs
	// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS message
	// container IE, the UE shall set the security header type of the initial NAS message to "integrity protected"
	if serviceRequest.NASMessageContainer != nil {
		logger.AmfLog.Warn("TO DELETE: Service Request has NAS Message Container")
		contents := serviceRequest.NASMessageContainer.GetNASMessageContainerContents()

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
			logger.AmfLog.Warn("TO DELETE: Initial NAS Message from NAS Message Container", zap.String("messageType", nas.MessageName(messageType)))
			if messageType != nas.MsgTypeServiceRequest {
				return fmt.Errorf("expected service request message, got %d", messageType)
			}
			// TS 24.501 4.4.6: The AMF shall consider the NAS message that is obtained from the NAS message container
			// IE as the initial NAS message that triggered the procedure
			serviceRequest = m.ServiceRequest
		}
		// TS 33.501 6.4.6 step 3: if the initial NAS message was protected but did not pass the integrity check
		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	logger.AmfLog.Warn("TO DELETE: Service Request", zap.String("type", getServiceRequestTypeString(serviceRequest.GetServiceTypeValue())))

	var pduStatusResult *[psiArraySize]bool
	if serviceRequest.PDUSessionStatus != nil {
		pduStatusResult = getPDUSessionStatus(ue, anType)
		logger.AmfLog.Warn("TO DELETE: PDU Session Status Result", zap.Any("pduStatusResult", pduStatusResult))
	}

	serviceType := serviceRequest.GetServiceTypeValue()
	logger.AmfLog.Warn("TO DELETE: Service Request Type", zap.String("type", getServiceRequestTypeString(serviceType)))
	var reactivationResult, acceptPduSessionPsi *[16]bool
	var errPduSessionID, errCause []uint8
	var targetPduSessionID int32
	suList := ngapType.PDUSessionResourceSetupListSUReq{}
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}

	if serviceType == nasMessage.ServiceTypeEmergencyServices || serviceType == nasMessage.ServiceTypeEmergencyServicesFallback {
		ue.GmmLog.Warn("emergency service is not supported")
	}

	if serviceRequest.UplinkDataStatus != nil {
		logger.AmfLog.Warn("TO DELETE: Uplink Data Status in Service Request")
		// print all PDU sessions with uplink data status
		uplinkDataPsi := nasConvert.PSIToBooleanArray(serviceRequest.UplinkDataStatus.Buffer)
		for psi, status := range uplinkDataPsi {
			if status {
				ue.GmmLog.Warn("TO DELETE: Uplink Data Status for PDU Session", zap.Int("pduSessionID", psi))
			}
		}
	}

	if serviceRequest.PDUSessionStatus != nil {
		logger.AmfLog.Warn("TO DELETE: PDU Session Status in Service Request")
		// print all PDU sessions with pdu session status
		psiArray := nasConvert.PSIToBooleanArray(serviceRequest.PDUSessionStatus.Buffer)
		for psi, status := range psiArray {
			if status {
				ue.GmmLog.Warn("TO DELETE: PDU Session Status for PDU Session", zap.Int("pduSessionID", psi))
			}
		}
	}

	if ue.MacFailed {
		ue.SecurityContextAvailable = false
		ue.GmmLog.Warn("Security Context Exist, But Integrity Check Failed with existing Context", zap.String("supi", ue.Supi))
		err := gmm_message.SendServiceReject(ue.RanUe[anType], nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.GmmLog.Info("sent service reject")
		err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		ue.GmmLog.Info("sent ue context release command")
		return nil
	}

	ue.RanUe[anType].UeContextRequest = true
	if serviceType == nasMessage.ServiceTypeSignalling {
		logger.AmfLog.Warn("TO DELETE: Service Request Type is Signalling", zap.Any("pduStatusResult", pduStatusResult))
		err := sendServiceAccept(ctx, ue, anType, ctxList, suList, pduStatusResult, nil, nil, nil)
		return err
	}
	if ue.N1N2Message != nil {
		requestData := ue.N1N2Message.Request.JSONData
		if ue.N1N2Message.Request.BinaryDataN2Information != nil {
			if requestData.N2InfoContainer.N2InformationClass == models.N2InformationClassSM {
				targetPduSessionID = requestData.N2InfoContainer.SmInfo.PduSessionID
			} else {
				ue.N1N2Message = nil
				return fmt.Errorf("n2 information class not supported: %v", requestData.N2InfoContainer.N2InformationClass)
			}
		}
	}

	if serviceRequest.UplinkDataStatus != nil {
		logger.AmfLog.Warn("TO DELETE: Uplink Data Status in Service Request")
		uplinkDataPsi := nasConvert.PSIToBooleanArray(serviceRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)
		ue.SmContextList.Range(func(key, value interface{}) bool {
			pduSessionID := key.(int32)
			smContext := value.(*context.SmContext)

			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] && smContext.AccessType() == models.AccessType3GPPAccess {
					logger.AmfLog.Warn("TO DELETE: Reactivating PDU Session from Uplink Data Status", zap.Int32("pduSessionID", pduSessionID))
					response, err := consumer.SendUpdateSmContextActivateUpCnxState(
						ctx, ue, smContext, models.AccessType3GPPAccess)
					if err != nil {
						ue.GmmLog.Error("SendUpdateSmContextActivateUpCnxState Error", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
					} else if response == nil {
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, uint8(pduSessionID))
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else if ue.RanUe[anType].UeContextRequest {
						ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList,
							pduSessionID, smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
					} else {
						ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList,
							pduSessionID, smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
					}
				}
			}
			return true
		})
	}
	if serviceRequest.PDUSessionStatus != nil {
		acceptPduSessionPsi = new([16]bool)
		psiArray := nasConvert.PSIToBooleanArray(serviceRequest.PDUSessionStatus.Buffer)
		ue.SmContextList.Range(func(key, value any) bool {
			pduSessionID := key.(int32)
			smContext := value.(*context.SmContext)
			if smContext.AccessType() == anType {
				if !psiArray[pduSessionID] {
					err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
					if err != nil {
						ue.GmmLog.Error("Release SmContext Error", zap.Error(err))
					}
				} else {
					acceptPduSessionPsi[pduSessionID] = true
				}
			}
			return true
		})
	}
	switch serviceType {
	case nasMessage.ServiceTypeMobileTerminatedServices: // Trigger by Network
		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message.Request.JSONData
			n1Msg := ue.N1N2Message.Request.BinaryDataN1Message
			n2Info := ue.N1N2Message.Request.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				err := sendServiceAccept(ctx, ue, anType, ctxList, suList, acceptPduSessionPsi,
					reactivationResult, errPduSessionID, errCause)
				if err != nil {
					return err
				}
				switch requestData.N1MessageContainer.N1MessageClass {
				case models.N1MessageClassSM:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				case models.N1MessageClassLPP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				case models.N1MessageClassSMS:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				case models.N1MessageClassUPDP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				}
				ue.N1N2Message = nil
				return nil
			}
			smInfo := requestData.N2InfoContainer.SmInfo
			smContext, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				ue.N1N2Message = nil
				return fmt.Errorf("service Request triggered by Network error for pduSessionID does not exist")
			}

			if smContext.AccessType() == models.AccessTypeNon3GPPAccess {
				if serviceRequest.AllowedPDUSessionStatus != nil {
					allowPduSessionPsi := nasConvert.PSIToBooleanArray(serviceRequest.AllowedPDUSessionStatus.Buffer)
					if reactivationResult == nil {
						reactivationResult = new([16]bool)
					}
					if allowPduSessionPsi[requestData.PduSessionID] {
						response, err := consumer.SendUpdateSmContextChangeAccessType(ctx, ue, smContext, true)
						if err != nil {
							return err
						} else if response == nil {
							reactivationResult[requestData.PduSessionID] = true
							errPduSessionID = append(errPduSessionID, uint8(requestData.PduSessionID))
							cause := nasMessage.Cause5GMMProtocolErrorUnspecified
							errCause = append(errCause, cause)
						} else {
							smContext.SetUserLocation(ue.Location)
							smContext.SetAccessType(models.AccessType3GPPAccess)
							if response.BinaryDataN2SmInformation != nil &&
								response.JSONData.N2SmInfoType == models.N2SmInfoTypePduResSetupReq {
								if ue.RanUe[anType].UeContextRequest {
									ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList,
										requestData.PduSessionID, smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
								} else {
									ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList,
										requestData.PduSessionID, smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
								}
							}
						}
					} else {
						ue.GmmLog.Warn("UE was reachable but did not accept to re-activate the PDU Session", zap.Int32("pduSessionID", requestData.PduSessionID))
					}
				}
			} else if smInfo.N2InfoContent.NgapIeType == models.NgapIeTypePduResSetupReq {
				var nasPdu []byte
				var err error
				if n1Msg != nil {
					pduSessionID := uint8(smInfo.PduSessionID)
					nasPdu, err = gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
					if err != nil {
						return err
					}
				}
				omecSnssai := models.Snssai{
					Sst: smInfo.SNssai.Sst,
					Sd:  smInfo.SNssai.Sd,
				}
				if ue.RanUe[anType].UeContextRequest {
					ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
				} else {
					ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
				}
			}
			err := sendServiceAccept(ctx, ue, anType, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause)
			if err != nil {
				return err
			}
		}
		// downlink signaling
		if ue.ConfigurationUpdateMessage != nil {
			err := sendServiceAccept(ctx, ue, anType, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause)
			if err != nil {
				return err
			}
			mobilityRestrictionList := ngap_message.BuildIEMobilityRestrictionList(ue)
			err = ngap_message.SendDownlinkNasTransport(ue.RanUe[models.AccessType3GPPAccess], ue.ConfigurationUpdateMessage, &mobilityRestrictionList)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}
			ue.GmmLog.Info("sent downlink nas transport")
			ue.ConfigurationUpdateMessage = nil
		}
	case nasMessage.ServiceTypeData:
		if anType == models.AccessType3GPPAccess {
			if ue.AmPolicyAssociation != nil && ue.AmPolicyAssociation.ServAreaRes != nil {
				var accept bool
				switch ue.AmPolicyAssociation.ServAreaRes.RestrictionType {
				case models.RestrictionTypeAllowedAreas:
					accept = context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
				case models.RestrictionTypeNotAllowedAreas:
					accept = !context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
				}

				if !accept {
					err := gmm_message.SendServiceReject(ue.RanUe[anType], nil, nasMessage.Cause5GMMRestrictedServiceArea)
					if err != nil {
						return fmt.Errorf("error sending service reject: %v", err)
					}
					ue.GmmLog.Info("sent service reject")
					return nil
				}
			}
			err := sendServiceAccept(ctx, ue, anType, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause)
			if err != nil {
				return err
			}
		} else {
			err := sendServiceAccept(ctx, ue, anType, ctxList, suList, acceptPduSessionPsi,
				reactivationResult, errPduSessionID, errCause)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("service type is not supported: %d", serviceType)
	}
	if len(errPduSessionID) != 0 {
		ue.GmmLog.Info("", zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	}
	ue.N1N2Message = nil
	return nil
}

func sendServiceAccept(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq, pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool, errPduSessionID, errCause []uint8,
) error {
	logger.AmfLog.Warn("TO DELETE: Send service accept", zap.String("anType", string(anType)), zap.Int("ctxList", len(ctxList.List)), zap.Int("suList", len(suList.List)), zap.Any("pDUSessionStatus", pDUSessionStatus), zap.Any("reactivationResult", reactivationResult), zap.Any("errPduSessionID", errPduSessionID), zap.Any("errCause", errCause))
	if ue.RanUe[anType].UeContextRequest {
		// update Kgnb/Kn3iwf
		ue.UpdateSecurityContext(anType)

		nasPdu, err := gmm_message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return err
		}
		logger.AmfLog.Warn("TO DELETE: Built Service Accept NAS PDU", zap.ByteString("nasPdu", nasPdu))
		if len(ctxList.List) != 0 {
			err := ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nasPdu, &ctxList, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Info("Sent NGAP initial context setup request")
		} else {
			err := ngap_message.SendInitialContextSetupRequest(ctx, ue, anType, nasPdu, nil, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Info("Sent NGAP initial context setup request")
		}
	} else if len(suList.List) != 0 {
		nasPdu, err := gmm_message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionID, errCause)
		if err != nil {
			return err
		}
		err = ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nasPdu, suList)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource setup request: %v", err)
		}
		ue.GmmLog.Info("Sent NGAP pdu session resource setup request")
	} else {
		err := gmm_message.SendServiceAccept(ue.RanUe[anType], pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}
		ue.GmmLog.Info("sent service accept")
	}
	return nil
}

// TS 24.501 5.4.1
func HandleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, authenticationResponse *nasMessage.AuthenticationResponse) error {
	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.AuthenticationCtx == nil {
		return fmt.Errorf("ue Authentication Context is nil")
	}

	switch ue.AuthenticationCtx.AuthType {
	case models.AuthType5GAka:
		av5gAka, ok := ue.AuthenticationCtx.Var5gAuthData.(models.Av5gAka)
		if !ok {
			return fmt.Errorf("Var5gAuthData type assertion failed: got %T", ue.AuthenticationCtx.Var5gAuthData)
		}
		resStar := authenticationResponse.AuthenticationResponseParameter.GetRES()

		// Calculate HRES* (TS 33.501 Annex A.5)
		p0, err := hex.DecodeString(av5gAka.Rand)
		if err != nil {
			return err
		}
		p1 := resStar[:]
		concat := append(p0, p1...)
		hResStarBytes := sha256.Sum256(concat)
		hResStar := hex.EncodeToString(hResStarBytes[16:])

		if hResStar != av5gAka.HxresStar {
			ue.GmmLog.Error("HRES* Validation Failure", zap.String("received", hResStar), zap.String("expected", av5gAka.HxresStar))

			if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
				err := gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
				if err != nil {
					return fmt.Errorf("send identity request error: %s", err)
				}
				ue.GmmLog.Info("sent identity request")
				return nil
			} else {
				err := gmm_message.SendAuthenticationReject(ue.RanUe[accessType], "")
				if err != nil {
					return fmt.Errorf("error sending GMM authentication reject: %v", err)
				}

				return GmmFSM.SendEvent(ctx, ue.State[accessType], AuthFailEvent, fsm.ArgsType{
					ArgAmfUe:      ue,
					ArgAccessType: accessType,
				})
			}
		}

		response, err := consumer.SendAuth5gAkaConfirmRequest(ctx, ue, hex.EncodeToString(resStar[:]))
		if err != nil {
			return fmt.Errorf("Authentication procedure failed: %s", err)
		}
		switch response.AuthResult {
		case models.AuthResultSuccess:
			ue.Kseaf = response.Kseaf
			ue.Supi = response.Supi
			ue.DerivateKamf()
			return GmmFSM.SendEvent(ctx, ue.State[accessType], AuthSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      ue,
				ArgAccessType: accessType,
				ArgEAPSuccess: false,
				ArgEAPMessage: "",
			})
		case models.AuthResultFailure:
			if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
				err := gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
				if err != nil {
					return fmt.Errorf("send identity request error: %s", err)
				}
				ue.GmmLog.Info("sent identity request")
				return nil
			} else {
				err := gmm_message.SendAuthenticationReject(ue.RanUe[accessType], "")
				if err != nil {
					return fmt.Errorf("error sending GMM authentication reject: %v", err)
				}

				return GmmFSM.SendEvent(ctx, ue.State[accessType], AuthFailEvent, fsm.ArgsType{
					ArgAmfUe:      ue,
					ArgAccessType: accessType,
				})
			}
		}
	case models.AuthTypeEAPAkaPrime:
		response, err := consumer.SendEapAuthConfirmRequest(ctx, ue.Suci, *authenticationResponse.EAPMessage)
		if err != nil {
			return err
		}

		switch response.AuthResult {
		case models.AuthResultSuccess:
			ue.Kseaf = response.KSeaf
			ue.Supi = response.Supi
			ue.DerivateKamf()
			return GmmFSM.SendEvent(ctx, ue.State[accessType], SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      ue,
				ArgAccessType: accessType,
				ArgEAPSuccess: true,
				ArgEAPMessage: response.EapPayload,
			})
		case models.AuthResultFailure:
			if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
				err := gmm_message.SendAuthenticationResult(ue.RanUe[accessType], false, response.EapPayload)
				if err != nil {
					return fmt.Errorf("send authentication result error: %s", err)
				}
				ue.GmmLog.Info("sent authentication result")
				err = gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
				if err != nil {
					return fmt.Errorf("send identity request error: %s", err)
				}
				ue.GmmLog.Info("sent identity request")
				return nil
			} else {
				err := gmm_message.SendAuthenticationReject(ue.RanUe[accessType], response.EapPayload)
				if err != nil {
					return fmt.Errorf("error sending GMM authentication reject: %v", err)
				}

				return GmmFSM.SendEvent(ctx, ue.State[accessType], AuthFailEvent, fsm.ArgsType{
					ArgAmfUe:      ue,
					ArgAccessType: accessType,
				})
			}
		case models.AuthResultOngoing:
			ue.AuthenticationCtx.Var5gAuthData = response.EapPayload
			err := gmm_message.SendAuthenticationRequest(ue.RanUe[accessType])
			if err != nil {
				return fmt.Errorf("send authentication request error: %s", err)
			}
			ue.GmmLog.Info("Sent authentication request")
		}
	}

	return nil
}

func HandleAuthenticationFailure(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, authenticationFailure *nasMessage.AuthenticationFailure) error {
	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause5GMM := authenticationFailure.Cause5GMM.GetCauseValue()

	if ue.AuthenticationCtx.AuthType == models.AuthType5GAka {
		switch cause5GMM {
		case nasMessage.Cause5GMMMACFailure:
			ue.GmmLog.Warn("Authentication Failure Cause: Mac Failure")
			err := gmm_message.SendAuthenticationReject(ue.RanUe[anType], "")
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return GmmFSM.SendEvent(ctx, ue.State[anType], AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue, ArgAccessType: anType})
		case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
			ue.GmmLog.Warn("Authentication Failure Cause: Non-5G Authentication Unacceptable")
			err := gmm_message.SendAuthenticationReject(ue.RanUe[anType], "")
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return GmmFSM.SendEvent(ctx, ue.State[anType], AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue, ArgAccessType: anType})
		case nasMessage.Cause5GMMngKSIAlreadyInUse:
			ue.GmmLog.Warn("Authentication Failure Cause: NgKSI Already In Use")
			ue.AuthFailureCauseSynchFailureTimes = 0
			ue.GmmLog.Warn("Select new NgKsi")
			// select new ngksi
			if ue.NgKsi.Ksi < 6 { // ksi is range from 0 to 6
				ue.NgKsi.Ksi += 1
			} else {
				ue.NgKsi.Ksi = 0
			}
			err := gmm_message.SendAuthenticationRequest(ue.RanUe[anType])
			if err != nil {
				return fmt.Errorf("send authentication request error: %s", err)
			}
			ue.GmmLog.Info("Sent authentication request")
		case nasMessage.Cause5GMMSynchFailure: // TS 24.501 5.4.1.3.7 case f
			ue.GmmLog.Warn("Authentication Failure 5GMM Cause: Synch Failure")

			ue.AuthFailureCauseSynchFailureTimes++
			if ue.AuthFailureCauseSynchFailureTimes >= 2 {
				ue.GmmLog.Warn("2 consecutive Synch Failure, terminate authentication procedure")
				err := gmm_message.SendAuthenticationReject(ue.RanUe[anType], "")
				if err != nil {
					return fmt.Errorf("error sending GMM authentication reject: %v", err)
				}

				return GmmFSM.SendEvent(ctx, ue.State[anType], AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue, ArgAccessType: anType})
			}

			auts := authenticationFailure.AuthenticationFailureParameter.GetAuthenticationFailureParameter()
			resynchronizationInfo := &models.ResynchronizationInfo{
				Auts: hex.EncodeToString(auts[:]),
			}

			response, err := consumer.SendUEAuthenticationAuthenticateRequest(ctx, ue, resynchronizationInfo)
			if err != nil {
				return fmt.Errorf("send UE Authentication Authenticate Request Error: %s", err.Error())
			}
			ue.AuthenticationCtx = response
			ue.ABBA = []uint8{0x00, 0x00}

			err = gmm_message.SendAuthenticationRequest(ue.RanUe[anType])
			if err != nil {
				return fmt.Errorf("send authentication request error: %s", err)
			}
			ue.GmmLog.Info("Sent authentication request")
		}
	} else if ue.AuthenticationCtx.AuthType == models.AuthTypeEAPAkaPrime {
		switch cause5GMM {
		case nasMessage.Cause5GMMngKSIAlreadyInUse:
			ue.GmmLog.Warn("Authentication Failure 5GMM Cause: NgKSI Already In Use")
			if ue.NgKsi.Ksi < 6 { // ksi is range from 0 to 6
				ue.NgKsi.Ksi += 1
			} else {
				ue.NgKsi.Ksi = 0
			}
			err := gmm_message.SendAuthenticationRequest(ue.RanUe[anType])
			if err != nil {
				return fmt.Errorf("send authentication request error: %s", err)
			}
			ue.GmmLog.Info("Sent authentication request")
		}
	}

	return nil
}

func getRegistrationTypeString(registrationType uint8) string {
	switch registrationType {
	case nasMessage.RegistrationType5GSInitialRegistration:
		return "Initial Registration"
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		return "Mobility Registration Updating"
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		return "Periodic Registration Updating"
	case nasMessage.RegistrationType5GSEmergencyRegistration:
		return "Emergency Registration"
	default:
		return "Reserved"
	}
}

func HandleRegistrationComplete(ctx ctxt.Context, ue *context.AmfUe, accessType models.AccessType, registrationComplete *nasMessage.RegistrationComplete) error {
	if ue.T3550 != nil {
		ue.T3550.Stop()
		ue.T3550 = nil // clear the timer
	}

	logger.AmfLog.Warn("TO DELETE: Registration Complete received", zap.String("RegistrationType", getRegistrationTypeString(ue.RegistrationType5GS)))

	forPending := ue.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	uds := ue.RegistrationRequest.UplinkDataStatus

	udsHasPending := uds != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !(forPending || udsHasPending || hasActiveSessions)

	logger.AmfLog.Warn("TO DELETE: shouldRelease", zap.Bool("shouldRelease", shouldRelease), zap.Bool("forPending", forPending), zap.Bool("udsHasPending", udsHasPending), zap.Bool("hasActiveSessions", hasActiveSessions))

	if shouldRelease {
		err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[accessType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		ue.GmmLog.Info("sent ue context release command")
	}

	return GmmFSM.SendEvent(ctx, ue.State[accessType], ContextSetupSuccessEvent, fsm.ArgsType{
		ArgAmfUe:      ue,
		ArgAccessType: accessType,
	})
}

// TS 33.501 6.7.2
func HandleSecurityModeComplete(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType, procedureCode int64, securityModeComplete *nasMessage.SecurityModeComplete) error {
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.SecurityContextIsValid() {
		// update Kgnb/Kn3iwf
		ue.UpdateSecurityContext(anType)
	}

	if securityModeComplete.IMEISV != nil {
		ue.GmmLog.Debug("receieve IMEISV")
		ue.Pei = nasConvert.PeiToString(securityModeComplete.IMEISV.Octet[:])
	}

	if securityModeComplete.NASMessageContainer != nil {
		contents := securityModeComplete.NASMessageContainer.GetNASMessageContainerContents()
		m := nas.NewMessage()
		if err := m.GmmMessageDecode(&contents); err != nil {
			return err
		}

		messageType := m.GmmMessage.GmmHeader.GetMessageType()
		if messageType != nas.MsgTypeRegistrationRequest && messageType != nas.MsgTypeServiceRequest {
			ue.GmmLog.Error("nas message container Iei type error")
			return errors.New("nas message container Iei type error")
		} else {
			return GmmFSM.SendEvent(ctx, ue.State[anType], SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:         ue,
				ArgAccessType:    anType,
				ArgProcedureCode: procedureCode,
				ArgNASMessage:    m.GmmMessage.RegistrationRequest,
			})
		}
	}
	return GmmFSM.SendEvent(ctx, ue.State[anType], SecurityModeSuccessEvent, fsm.ArgsType{
		ArgAmfUe:         ue,
		ArgAccessType:    anType,
		ArgProcedureCode: procedureCode,
		ArgNASMessage:    ue.RegistrationRequest,
	})
}

func HandleSecurityModeReject(ue *context.AmfUe, anType models.AccessType,
	securityModeReject *nasMessage.SecurityModeReject,
) error {
	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause := securityModeReject.Cause5GMM.GetCauseValue()
	ue.GmmLog.Warn("Reject", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
	ue.GmmLog.Error("UE reject the security mode command, abort the ongoing procedure")

	ue.SecurityContextAvailable = false

	err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}
	ue.GmmLog.Info("sent ue context release command")
	return nil
}

// TS 23.502 4.2.2.3
func HandleDeregistrationRequest(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType,
	deregistrationRequest *nasMessage.DeregistrationRequestUEOriginatingDeregistration,
) error {
	targetDeregistrationAccessType := deregistrationRequest.GetAccessType()
	ue.SmContextList.Range(func(key, value interface{}) bool {
		smContext := value.(*context.SmContext)

		if smContext.AccessType() == anType ||
			targetDeregistrationAccessType == nasMessage.AccessTypeBoth {
			err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
			if err != nil {
				ue.GmmLog.Error("Release SmContext Error", zap.Error(err))
			}
		}
		return true
	})

	if ue.AmPolicyAssociation != nil {
		terminateAmPolicyAssocaition := true
		switch anType {
		case models.AccessType3GPPAccess:
			terminateAmPolicyAssocaition = ue.State[models.AccessTypeNon3GPPAccess].Is(context.Deregistered)
		case models.AccessTypeNon3GPPAccess:
			terminateAmPolicyAssocaition = ue.State[models.AccessType3GPPAccess].Is(context.Deregistered)
		}

		if terminateAmPolicyAssocaition {
			err := consumer.AMPolicyControlDelete(ctx, ue)
			if err != nil {
				ue.GmmLog.Error("AM Policy Control Delete Error", zap.Error(err))
			}
		}
	}

	// if Deregistration type is not switch-off, send Deregistration Accept
	if deregistrationRequest.GetSwitchOff() == 0 && ue.RanUe[anType] != nil {
		err := gmm_message.SendDeregistrationAccept(ue.RanUe[anType])
		if err != nil {
			return fmt.Errorf("error sending deregistration accept: %v", err)
		}
		ue.GmmLog.Info("sent deregistration accept")
	}

	// TS 23.502 4.2.6, 4.12.3
	switch targetDeregistrationAccessType {
	case nasMessage.AccessType3GPP:
		if ue.RanUe[models.AccessType3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType3GPPAccess], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
		return GmmFSM.SendEvent(ctx, ue.State[models.AccessType3GPPAccess], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
	case nasMessage.AccessTypeNon3GPP:
		if ue.RanUe[models.AccessTypeNon3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessTypeNon3GPPAccess], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
		return GmmFSM.SendEvent(ctx, ue.State[models.AccessTypeNon3GPPAccess], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
	case nasMessage.AccessTypeBoth:
		if ue.RanUe[models.AccessType3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType3GPPAccess], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
		if ue.RanUe[models.AccessTypeNon3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessTypeNon3GPPAccess], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}

		err := GmmFSM.SendEvent(ctx, ue.State[models.AccessType3GPPAccess], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
		if err != nil {
			ue.GmmLog.Error("Send Deregistration Accept Event Error", zap.Error(err))
		}
		return GmmFSM.SendEvent(ctx, ue.State[models.AccessTypeNon3GPPAccess], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
	}

	return nil
}

// TS 23.502 4.2.2.3
func HandleDeregistrationAccept(ctx ctxt.Context, ue *context.AmfUe, anType models.AccessType,
	deregistrationAccept *nasMessage.DeregistrationAcceptUETerminatedDeregistration,
) error {
	if ue.T3522 != nil {
		ue.T3522.Stop()
		ue.T3522 = nil // clear the timer
	}

	switch ue.DeregistrationTargetAccessType {
	case nasMessage.AccessType3GPP:
		if ue.RanUe[models.AccessType3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType3GPPAccess], context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
	case nasMessage.AccessTypeNon3GPP:
		if ue.RanUe[models.AccessTypeNon3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessTypeNon3GPPAccess],
				context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
	case nasMessage.AccessTypeBoth:
		if ue.RanUe[models.AccessType3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType3GPPAccess],
				context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
		if ue.RanUe[models.AccessTypeNon3GPPAccess] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessTypeNon3GPPAccess],
				context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Info("sent ue context release command")
		}
	}

	ue.DeregistrationTargetAccessType = 0

	return GmmFSM.SendEvent(ctx, ue.State[models.AccessType3GPPAccess], DeregistrationAcceptEvent, fsm.ArgsType{
		ArgAmfUe:      ue,
		ArgAccessType: anType,
	})
}

func HandleStatus5GMM(ue *context.AmfUe, anType models.AccessType, status5GMM *nasMessage.Status5GMM) error {
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	cause := status5GMM.Cause5GMM.GetCauseValue()
	ue.GmmLog.Error("Error condition", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
	return nil
}

func HandleAuthenticationError(ue *context.AmfUe, anType models.AccessType) error {
	if ue.RegistrationRequest != nil {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		ue.GmmLog.Info("sent registration reject")
	}
	return nil
}
