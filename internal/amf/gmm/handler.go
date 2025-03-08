// Copyright 2024 Ella Networks
package gmm

import (
	"bytes"
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
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/ngap/ngapType"
)

const (
	S_NSSAI_CONGESTION        = "S-NSSAI_CONGESTION"
	DNN_CONGESTION            = "DNN_CONGESTION"
	PRIORITIZED_SERVICES_ONLY = "PRIORITIZED_SERVICES_ONLY"
	OUT_OF_LADN_SERVICE_AREA  = "OUT_OF_LADN_SERVICE_AREA"
)

func SnssaiModelsToHex(snssai models.Snssai) string {
	sst := fmt.Sprintf("%02x", snssai.Sst)
	return sst + snssai.Sd
}

func PlmnIdStringToModels(plmnId string) (plmnID models.PlmnId) {
	plmnID.Mcc = plmnId[:3]
	plmnID.Mnc = plmnId[3:]
	return
}

func HandleULNASTransport(ue *context.AmfUe, anType models.AccessType,
	ulNasTransport *nasMessage.ULNASTransport,
) error {
	ue.GmmLog.Infoln("Handle UL NAS Transport")

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	switch ulNasTransport.GetPayloadContainerType() {
	// TS 24.501 5.4.5.2.3 case a)
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return transport5GSMMessage(ue, anType, ulNasTransport)
	case nasMessage.PayloadContainerTypeSMS:
		return fmt.Errorf("PayloadContainerTypeSMS has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeLPP:
		return fmt.Errorf("PayloadContainerTypeLPP has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeSOR:
		return fmt.Errorf("PayloadContainerTypeSOR has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeUEPolicy:
		ue.GmmLog.Infoln("AMF Transfer UEPolicy To PCF")
	case nasMessage.PayloadContainerTypeUEParameterUpdate:
		ue.GmmLog.Infoln("AMF Transfer UEParameterUpdate To UDM")
		upuMac, err := nasConvert.UpuAckToModels(ulNasTransport.PayloadContainer.GetPayloadContainerContents())
		if err != nil {
			return err
		}
		ue.GmmLog.Debugf("UpuMac[%s] in UPU ACK NAS Msg", upuMac)
	case nasMessage.PayloadContainerTypeMultiplePayload:
		return fmt.Errorf("PayloadContainerTypeMultiplePayload has not been implemented yet in UL NAS TRANSPORT")
	}
	return nil
}

func transport5GSMMessage(ue *context.AmfUe, anType models.AccessType,
	ulNasTransport *nasMessage.ULNASTransport,
) error {
	var pduSessionID int32

	ue.GmmLog.Info("Transport 5GSM Message to SMF")

	smMessage := ulNasTransport.PayloadContainer.GetPayloadContainerContents()

	if id := ulNasTransport.PduSessionID2Value; id != nil {
		pduSessionID = int32(id.GetPduSessionID2Value())
	} else {
		return errors.New("PDU Session ID is nil")
	}

	// case 1): looks up a PDU session routing context for the UE and the PDU session ID IE in case the Old PDU
	// session ID IE is not included
	if ulNasTransport.OldPDUSessionID == nil {
		smContext, smContextExist := ue.SmContextFindByPDUSessionID(pduSessionID)
		requestType := ulNasTransport.RequestType

		if requestType != nil {
			switch requestType.GetRequestTypeValue() {
			case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest:
				fallthrough
			case nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
				ue.GmmLog.Warnf("Emergency PDU Session is not supported")
				err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}
				ue.GmmLog.Infof("sent downlink nas transport to UE")
				return nil
			}
		}

		if smContextExist && requestType != nil {
			/* AMF releases context locally as this is duplicate pdu session */
			if requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeInitialRequest {
				ue.SmContextList.Delete(pduSessionID)
				smContextExist = false
			}
		}

		if !smContextExist {
			msg := new(nas.Message)
			if err := msg.PlainNasDecode(&smMessage); err != nil {
				ue.GmmLog.Errorf("Could not decode Nas message: %v", err)
			}
			if msg.GsmMessage != nil && msg.GsmMessage.Status5GSM != nil {
				ue.GmmLog.Warnf("SmContext doesn't exist, 5GSM Status message received from UE with cause %v", msg.GsmMessage.Status5GSM.Cause5GSM)
				return nil
			}
		}
		// AMF has a PDU session routing context for the PDU session ID and the UE
		if smContextExist {
			// case i) Request type IE is either not included
			if requestType == nil {
				return forward5GSMMessageToSMF(ue, anType, pduSessionID, smContext, smMessage)
			}

			switch requestType.GetRequestTypeValue() {
			case nasMessage.ULNASTransportRequestTypeInitialRequest:
				smContext.StoreULNASTransport(ulNasTransport)
				//  perform a local release of the PDU session identified by the PDU session ID and shall request
				// the SMF to perform a local release of the PDU session
				updateData := models.SmContextUpdateData{
					Release: true,
					Cause:   models.Cause_REL_DUE_TO_DUPLICATE_SESSION_ID,
					SmContextStatusUri: fmt.Sprintf("%s/namf-callback/v1/smContextStatus/%s/%d",
						ue.ServingAMF.GetIPv4Uri(), ue.Guti, pduSessionID),
				}
				ue.GmmLog.Warnf("Duplicated PDU session ID[%d]", pduSessionID)
				smContext.SetDuplicatedPduSessionID(true)
				response, err := consumer.SendUpdateSmContextRequest(smContext, updateData, nil, nil)
				if err != nil {
					return err
				}
				if response == nil {
					ue.GmmLog.Errorf("PDU Session ID[%d] can't be released in DUPLICATE_SESSION_ID case", pduSessionID)
					err = gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport: %s", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport to UE")
				} else {
					smContext.SetUserLocation(ue.Location)
					responseData := response.JsonData
					n2Info := response.BinaryDataN2SmInformation
					if n2Info != nil {
						switch responseData.N2SmInfoType {
						case models.N2SmInfoType_PDU_RES_REL_CMD:
							ue.GmmLog.Debugln("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
							list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
							ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2Info)
							err := ngap_message.SendPDUSessionResourceReleaseCommand(ue.RanUe[anType], nil, list)
							if err != nil {
								return fmt.Errorf("error sending pdu session resource release command: %s", err)
							}
							ue.GmmLog.Infof("sent pdu session resource release command to UE")
						}
					}
				}

			// case ii) AMF has a PDU session routing context, and Request type is "existing PDU session"
			case nasMessage.ULNASTransportRequestTypeExistingPduSession:
				if ue.InAllowedNssai(smContext.Snssai(), anType) {
					return forward5GSMMessageToSMF(ue, anType, pduSessionID, smContext, smMessage)
				} else {
					ue.GmmLog.Errorf("S-NSSAI[%v] is not allowed for access type[%s] (PDU Session ID: %d)", smContext.Snssai(), anType, pduSessionID)
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport: %s", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport to UE")
				}
			// other requestType: AMF forward the 5GSM message, and the PDU session ID IE towards the SMF identified
			// by the SMF ID of the PDU session routing context
			default:
				return forward5GSMMessageToSMF(ue, anType, pduSessionID, smContext, smMessage)
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
						return errors.New("Ue doesn't have allowedNssai")
					}
				}

				if ulNasTransport.DNN != nil {
					dnn = string(ulNasTransport.DNN.GetDNN())
				} else {
					// if user's subscription context obtained from UDM does not contain the default DNN for the,
					// S-NSSAI, the AMF shall use a locally configured DNN as the DNN
					dnn = ue.ServingAMF.SupportedDnns[0]

					if ue.SmfSelectionData != nil {
						snssaiStr := SnssaiModelsToHex(snssai)
						if snssaiInfo, ok := ue.SmfSelectionData.SubscribedSnssaiInfos[snssaiStr]; ok {
							for _, dnnInfo := range snssaiInfo.DnnInfos {
								if dnnInfo.DefaultDnnIndicator {
									dnn = dnnInfo.Dnn
								}
							}
						}
					}
				}

				if newSmContext, cause, err := consumer.SelectSmf(ue, anType, pduSessionID, snssai, dnn); err != nil {
					ue.GmmLog.Errorf("Select SMF failed: %+v", err)
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, cause)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport: %s", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport to UE")
				} else {
					smContextRef, errResponse, err := consumer.SendCreateSmContextRequest(ue, newSmContext, smMessage)
					if err != nil {
						ue.GmmLog.Errorf("error sending sm context request: %v", err)
						return nil
					} else if errResponse != nil {
						ue.GmmLog.Warnf("pdu Session Establishment Request was rejected by SMF [pduSessionId: %d]", pduSessionID)
						err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, errResponse.BinaryDataN1SmMessage, pduSessionID, 0)
						if err != nil {
							return fmt.Errorf("error sending downlink nas transport: %s", err)
						}
						ue.GmmLog.Infof("sent downlink nas transport to UE")
					} else {
						newSmContext.SetSmContextRef(smContextRef)
						newSmContext.SetUserLocation(ue.Location)
						ue.StoreSmContext(pduSessionID, newSmContext)
						ue.GmmLog.Infof("created sm context for pdu session id %d", pduSessionID)
					}
				}
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
						ue.GmmLog.Infof("sent downlink nas transport to UE")
					} else {
						// TS 24.501 5.4.5.2.3 case a) 1) iv)
						smContext = context.NewSmContext(pduSessionID)
						smContext.SetAccessType(anType)
						smContext.SetDnn(ueContextInSmf.Dnn)
						smContext.SetPlmnID(*ueContextInSmf.PlmnId)
						ue.StoreSmContext(pduSessionID, smContext)
						return forward5GSMMessageToSMF(ue, anType, pduSessionID, smContext, smMessage)
					}
				} else {
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport: %s", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport to UE")
				}
			default:
			}
		}
	} else {
		return fmt.Errorf("SSC mode3 operation has not been implemented yet")
	}
	return nil
}

func forward5GSMMessageToSMF(
	ue *context.AmfUe,
	accessType models.AccessType,
	pduSessionID int32,
	smContext *context.SmContext,
	smMessage []byte,
) error {
	smContextUpdateData := models.SmContextUpdateData{
		N1SmMsg: &models.RefToBinaryData{
			ContentId: "N1SmMsg",
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

	response, err := consumer.SendUpdateSmContextRequest(smContext, smContextUpdateData, smMessage, nil)
	if err != nil {
		ue.GmmLog.Errorf("Update SMContext error [pduSessionID: %d], Error[%v]", pduSessionID, err)
		return nil
	} else if response != nil {
		// update SmContext in AMF
		smContext.SetAccessType(accessType)
		smContext.SetUserLocation(ue.Location)

		responseData := response.JsonData
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
			ue.GmmLog.Debugf("Receive N2 SM Information[%s] from SMF", responseData.N2SmInfoType)
			switch responseData.N2SmInfoType {
			case models.N2SmInfoType_PDU_RES_MOD_REQ:
				list := ngapType.PDUSessionResourceModifyListModReq{}
				ngap_message.AppendPDUSessionResourceModifyListModReq(&list, pduSessionID, n1Msg, n2SmInfo)
				err := ngap_message.SendPDUSessionResourceModifyRequest(ue.RanUe[accessType], list)
				if err != nil {
					return fmt.Errorf("error sending pdu session resource modify request: %s", err)
				}
				ue.GmmLog.Infof("sent pdu session resource modify request to UE")
			case models.N2SmInfoType_PDU_RES_REL_CMD:
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2SmInfo)
				err := ngap_message.SendPDUSessionResourceReleaseCommand(ue.RanUe[accessType], n1Msg, list)
				if err != nil {
					return fmt.Errorf("error sending pdu session resource release command: %s", err)
				}
				ue.GmmLog.Infof("sent pdu session resource release command to UE")
			default:
				return fmt.Errorf("error N2 SM information type[%s]", responseData.N2SmInfoType)
			}
		} else if n1Msg != nil {
			ue.GmmLog.Debugf("AMF forward Only N1 SM Message to UE")
			err := ngap_message.SendDownlinkNasTransport(ue.RanUe[accessType], n1Msg, nil)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.GmmLog.Infof("sent downlink nas transport to UE")
		}
	}
	return nil
}

// Handle cleartext IEs of Registration Request, which cleattext IEs defined in TS 24.501 4.4.6
func HandleRegistrationRequest(ue *context.AmfUe, anType models.AccessType, procedureCode int64,
	registrationRequest *nasMessage.RegistrationRequest,
) error {
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
		amfSelf.ReAllocateGutiToUe(ue)
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
				return errors.New("The payload of NAS Message Container is not Registration Request")
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
	switch ue.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		ue.GmmLog.Debugf("RegistrationType: Initial Registration")
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		ue.GmmLog.Debugf("RegistrationType: Mobility Registration Updating")
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		ue.GmmLog.Debugf("RegistrationType: Periodic Registration Updating")
	case nasMessage.RegistrationType5GSEmergencyRegistration:
		return fmt.Errorf("Not Supportted RegistrationType: Emergency Registration")
	case nasMessage.RegistrationType5GSReserved:
		ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
		ue.GmmLog.Debugf("RegistrationType: Reserved")
	default:
		ue.GmmLog.Debugf("RegistrationType: %v, chage state to InitialRegistration", ue.RegistrationType5GS)
		ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	}

	mobileIdentity5GSContents := registrationRequest.MobileIdentity5GS.GetMobileIdentity5GSContents()
	ue.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch ue.IdentityTypeUsedForRegistration { // get type of identity
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.GmmLog.Debugf("No Identity")
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnId string
		ue.Suci, plmnId = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnId = PlmnIdStringToModels(plmnId)
		ue.GmmLog.Debugf("SUCI: %s", ue.Suci)
	case nasMessage.MobileIdentity5GSType5gGuti:
		guamiFromUeGutiTmp, guti := util.GutiToString(mobileIdentity5GSContents)
		guamiFromUeGuti = guamiFromUeGutiTmp
		ue.Guti = guti
		ue.GmmLog.Debugf("GUTI: %s", guti)

		guamiList := context.GetServedGuamiList()
		servedGuami := guamiList[0]
		if reflect.DeepEqual(guamiFromUeGuti, servedGuami) {
			ue.ServingAmfChanged = false
		} else {
			ue.GmmLog.Debugf("Serving AMF has changed but 5G-Core is not supporting for now")
			ue.ServingAmfChanged = false
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imei
		ue.GmmLog.Debugf("PEI: %s", imei)
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imeisv
		ue.GmmLog.Debugf("PEI: %s", imeisv)
	}

	// NgKsi: TS 24.501 9.11.3.32
	switch registrationRequest.NgksiAndRegistrationType5GS.GetTSC() {
	case nasMessage.TypeOfSecurityContextFlagNative:
		ue.NgKsi.Tsc = models.ScType_NATIVE
	case nasMessage.TypeOfSecurityContextFlagMapped:
		ue.NgKsi.Tsc = models.ScType_MAPPED
	}
	ue.NgKsi.Ksi = int32(registrationRequest.NgksiAndRegistrationType5GS.GetNasKeySetIdentifiler())
	if ue.NgKsi.Tsc == models.ScType_NATIVE && ue.NgKsi.Ksi != 7 {
	} else {
		ue.NgKsi.Tsc = models.ScType_NATIVE
		ue.NgKsi.Ksi = 0
	}

	// Copy UserLocation from ranUe
	ue.Location = ue.RanUe[anType].Location
	ue.Tai = ue.RanUe[anType].Tai

	// Check TAI
	supportTaiList := context.GetSupportTaiList()
	taiList := make([]models.Tai, len(supportTaiList))
	copy(taiList, supportTaiList)
	for i := range taiList {
		tac, err := util.TACConfigToModels(taiList[i].Tac)
		if err != nil {
			logger.AmfLog.Warnf("failed to convert TAC[%s] to models.Tac", taiList[i].Tac)
			continue
		}
		taiList[i].Tac = tac
	}
	if !context.InTaiList(ue.Tai, taiList) {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMTrackingAreaNotAllowed, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Infof("sent registration reject to UE")
		return fmt.Errorf("registration Reject[Tracking area not allowed]")
	}

	if registrationRequest.UESecurityCapability != nil {
		ue.UESecurityCapability = *registrationRequest.UESecurityCapability
	} else {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMProtocolErrorUnspecified, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Infof("sent registration reject to UE")
		return fmt.Errorf("UESecurityCapability is nil")
	}
	if ue.ServingAmfChanged {
		var transferReason models.TransferReason
		switch ue.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			transferReason = models.TransferReason_INIT_REG
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			transferReason = models.TransferReason_MOBI_REG
		}

		ue.TargetAmfUri = amfSelf.GetIPv4Uri()

		ueContextTransferRspData, err := consumer.UEContextTransferRequest(ue, anType, transferReason)
		if err != nil {
			ue.GmmLog.Errorf("UE Context Transfer Request Error[%+v]", err)
		} else {
			ue.CopyDataFromUeContextModel(*ueContextTransferRspData.UeContext)
		}
	}
	return nil
}

func IdentityVerification(ue *context.AmfUe) bool {
	return ue.Supi != "" || len(ue.Suci) != 0
}

func HandleInitialRegistration(ue *context.AmfUe, anType models.AccessType) error {
	ue.GmmLog.Infoln("Handle InitialRegistration")

	amfSelf := context.AMFSelf()

	ue.ClearRegistrationData()

	// update Kgnb/Kn3iwf
	ue.UpdateSecurityContext(anType)

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if len(ue.SubscribedNssai) == 0 {
		getSubscribedNssai(ue)
	}

	if err := handleRequestedNssai(ue, anType); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	}

	if len(ue.AllowedNssai[anType]) == 0 {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMM5GSServicesNotAllowed, "")
		if err != nil {
			ue.GmmLog.Errorf("error sending registration reject: %v", err)
		}
		err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			ue.GmmLog.Errorf("error sending ue context release command: %v", err)
		}
		ue.Remove()
		return fmt.Errorf("no allowed nssai")
	}

	storeLastVisitedRegisteredTAI(ue, ue.RegistrationRequest.LastVisitedRegisteredTAI)

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.GmmLog.Warnf("Receive MICO Indication[RAAI: %d], Not Supported",
			ue.RegistrationRequest.MICOIndication.GetRAAI())
	}

	negotiateDRXParameters(ue, ue.RegistrationRequest.RequestedDRXParameters)

	if ue.ServingAmfChanged {
		// If the AMF has changed the new AMF notifies the old AMF that the registration of the UE in the new AMF is completed
		req := models.UeRegStatusUpdateReqData{
			TransferStatus: models.UeContextTransferStatus_TRANSFERRED,
		}
		regStatusTransferComplete, err := consumer.RegistrationStatusUpdate(ue, req)
		if err != nil {
			ue.GmmLog.Errorf("Registration Status Update Error[%+v]", err)
		} else {
			if regStatusTransferComplete {
				ue.GmmLog.Infof("Registration Status Transfer complete")
			}
		}
	}

	if ue.ServingAmfChanged || ue.State[models.AccessType_NON_3_GPP_ACCESS].Is(context.Registered) ||
		!ue.SubscriptionDataValid {
		if err := communicateWithUDM(ue, anType); err != nil {
			return err
		}
	}

	err := consumer.AMPolicyControlCreate(ue, anType)
	if err != nil {
		ue.GmmLog.Errorf("AM Policy Control Create Error[%+v]", err)
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMM5GSServicesNotAllowed, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Infof("sent registration reject to UE")
		return err
	}

	// Service Area Restriction are applicable only to 3GPP access
	if anType == models.AccessType__3_GPP_ACCESS {
		if ue.AmPolicyAssociation != nil && ue.AmPolicyAssociation.ServAreaRes != nil {
			servAreaRes := ue.AmPolicyAssociation.ServAreaRes
			if servAreaRes.RestrictionType == models.RestrictionType_ALLOWED_AREAS {
				numOfallowedTAs := 0
				for _, area := range servAreaRes.Areas {
					numOfallowedTAs += len(area.Tacs)
				}
			}
		}
	}

	amfSelf.AllocateRegistrationArea(ue, anType)
	ue.GmmLog.Debugf("Use original GUTI[%s]", ue.Guti)

	assignLadnInfo(ue, anType)

	amfSelf.AddAmfUeToUePool(ue, ue.Supi)
	ue.T3502Value = amfSelf.T3502Value
	if anType == models.AccessType__3_GPP_ACCESS {
		ue.T3512Value = amfSelf.T3512Value
	} else {
		ue.Non3gppDeregistrationTimerValue = amfSelf.Non3gppDeregistrationTimerValue
	}

	if anType == models.AccessType__3_GPP_ACCESS {
		err := gmm_message.SendRegistrationAccept(ue, anType, nil, nil, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("error sending registration accept: %v", err)
		}
		ue.GmmLog.Infof("sent registration accept to UE")
	} else {
		// TS 23.502 4.12.2.2 10a ~ 13: if non-3gpp, AMF should send initial context setup request to N3IWF first,
		// and send registration accept after receiving initial context setup response
		err := ngap_message.SendInitialContextSetupRequest(ue, anType, nil, nil, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("error sending initial context setup request: %v", err)
		}
		ue.GmmLog.Infof("sent initial context setup request to N3IWF")
		registrationAccept, err := gmm_message.BuildRegistrationAccept(ue, anType, nil, nil, nil, nil)
		if err != nil {
			ue.GmmLog.Errorf("Build Registration Accept: %+v", err)
			return nil
		}
		ue.RegistrationAcceptForNon3GPPAccess = registrationAccept
	}
	return nil
}

func HandleMobilityAndPeriodicRegistrationUpdating(ue *context.AmfUe, anType models.AccessType) error {
	ue.GmmLog.Infoln("Handle MobilityAndPeriodicRegistrationUpdating")

	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.UpdateType5GS != nil {
		if ue.RegistrationRequest.UpdateType5GS.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.UeRadioCapability = ""
			ue.UeRadioCapabilityForPaging = nil
		}
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if len(ue.SubscribedNssai) == 0 {
		getSubscribedNssai(ue)
	}

	if err := handleRequestedNssai(ue, anType); err != nil {
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
			ue.GmmLog.Infof("sent registration reject to UE")
			return fmt.Errorf("Capability5GMM is nil")
		}
	}

	storeLastVisitedRegisteredTAI(ue, ue.RegistrationRequest.LastVisitedRegisteredTAI)

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.GmmLog.Warnf("Receive MICO Indication[RAAI: %d], Not Supported",
			ue.RegistrationRequest.MICOIndication.GetRAAI())
	}

	negotiateDRXParameters(ue, ue.RegistrationRequest.RequestedDRXParameters)

	if len(ue.Pei) == 0 {
		err := gmm_message.SendIdentityRequest(ue.RanUe[anType], nasMessage.MobileIdentity5GSTypeImei)
		if err != nil {
			return fmt.Errorf("error sending identity request: %v", err)
		}
		ue.GmmLog.Infof("sent identity request to UE")
		return nil
	}

	if ue.ServingAmfChanged || ue.State[models.AccessType_NON_3_GPP_ACCESS].Is(context.Registered) ||
		!ue.SubscriptionDataValid {
		if err := communicateWithUDM(ue, anType); err != nil {
			return err
		}
	}

	var reactivationResult *[16]bool
	var errPduSessionId, errCause []uint8
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}
	suList := ngapType.PDUSessionResourceSetupListSUReq{}

	if ue.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)
		allowReEstablishPduSession := true

		// determines that the UE is in non-allowed area or is not in allowed area
		if ue.AmPolicyAssociation != nil && ue.AmPolicyAssociation.ServAreaRes != nil {
			switch ue.AmPolicyAssociation.ServAreaRes.RestrictionType {
			case models.RestrictionType_ALLOWED_AREAS:
				allowReEstablishPduSession = context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
			case models.RestrictionType_NOT_ALLOWED_AREAS:
				allowReEstablishPduSession = !context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
			}
		}

		if !allowReEstablishPduSession {
			for pduSessionId, hasUplinkData := range uplinkDataPsi {
				if hasUplinkData {
					errPduSessionId = append(errPduSessionId, uint8(pduSessionId))
					errCause = append(errCause, nasMessage.Cause5GMMRestrictedServiceArea)
				}
			}
		} else {
			for idx, hasUplinkData := range uplinkDataPsi {
				pduSessionId := int32(idx)
				if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionId); ok {
					// uplink data are pending for the corresponding PDU session identity
					if hasUplinkData && smContext.AccessType() == models.AccessType__3_GPP_ACCESS {
						response, err := consumer.SendUpdateSmContextActivateUpCnxState(ue, smContext, anType)
						if response == nil {
							reactivationResult[pduSessionId] = true
							errPduSessionId = append(errPduSessionId, uint8(pduSessionId))
							cause := nasMessage.Cause5GMMProtocolErrorUnspecified
							errCause = append(errCause, cause)

							if err != nil {
								ue.GmmLog.Errorf("Update SmContext Error[%v]", err.Error())
							}
						} else {
							if ue.RanUe[anType].UeContextRequest {
								ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionId,
									smContext.Snssai(), response.BinaryDataN1SmMessage, response.BinaryDataN2SmInformation)
							} else {
								ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionId,
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
		for psi := 1; psi <= 15; psi++ {
			pduSessionId := int32(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionId); ok {
				if !psiArray[psi] && smContext.AccessType() == anType {
					err := consumer.SendReleaseSmContextRequest(smContext)
					if err != nil {
						pduSessionStatus[psi] = true
						ue.GmmLog.Errorf("error sending release sm context request: %v", err)
					} else {
						pduSessionStatus[psi] = false
					}
				} else {
					pduSessionStatus[psi] = false
				}
			}
		}
	}

	if ue.RegistrationRequest.AllowedPDUSessionStatus != nil {
		allowedPsis := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.AllowedPDUSessionStatus.Buffer)
		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message.Request.JsonData
			n1Msg := ue.N1N2Message.Request.BinaryDataN1Message
			n2Info := ue.N1N2Message.Request.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := gmm_message.BuildRegistrationAccept(ue, anType, pduSessionStatus,
						reactivationResult, errPduSessionId, errCause)
					if err != nil {
						return err
					}
					err = ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nasPdu, suList)
					if err != nil {
						return fmt.Errorf("error sending pdu session resource setup request: %v", err)
					}
					ue.GmmLog.Infof("sent pdu session resource setup request")
				} else {
					err := gmm_message.SendRegistrationAccept(ue, anType, pduSessionStatus, reactivationResult, errPduSessionId, errCause, &ctxList)
					if err != nil {
						return fmt.Errorf("error sending registration accept: %v", err)
					}
					ue.GmmLog.Infof("sent registration accept")
				}
				switch requestData.N1MessageContainer.N1MessageClass {
				case models.N1MessageClass_SM:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionId, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message to UE")
				case models.N1MessageClass_LPP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message to UE")
				case models.N1MessageClass_SMS:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message to UE")
				case models.N1MessageClass_UPDP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message to UE")
				}
				ue.N1N2Message = nil
				return nil
			}

			smInfo := requestData.N2InfoContainer.SmInfo
			smContext, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionId)
			if !exist {
				ue.N1N2Message = nil
				return fmt.Errorf("pdu Session Id does not Exists")
			}

			if smContext.AccessType() == models.AccessType_NON_3_GPP_ACCESS {
				if reactivationResult == nil {
					reactivationResult = new([16]bool)
				}
				if allowedPsis[requestData.PduSessionId] {
					response, err := consumer.SendUpdateSmContextChangeAccessType(ue, smContext, true)
					if err != nil {
						return err
					} else if response == nil {
						reactivationResult[requestData.PduSessionId] = true
						errPduSessionId = append(errPduSessionId, uint8(requestData.PduSessionId))
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else {
						smContext.SetUserLocation(ue.Location)
						smContext.SetAccessType(models.AccessType__3_GPP_ACCESS)
						if response.BinaryDataN2SmInformation != nil &&
							response.JsonData.N2SmInfoType == models.N2SmInfoType_PDU_RES_SETUP_REQ {
							ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionId,
								smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
						}
					}
				} else {
					ue.GmmLog.Warnf("UE was reachable but did not accept to re-activate the PDU Session[%d]",
						requestData.PduSessionId)
				}
			} else if smInfo.N2InfoContent.NgapIeType == models.NgapIeType_PDU_RES_SETUP_REQ {
				var nasPdu []byte
				var err error
				if n1Msg != nil {
					pduSessionId := uint8(smInfo.PduSessionId)
					nasPdu, err = gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionId, nil)
					if err != nil {
						return err
					}
				}
				omecSnssai := models.Snssai{
					Sst: smInfo.SNssai.Sst,
					Sd:  smInfo.SNssai.Sd,
				}
				ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, smInfo.PduSessionId,
					omecSnssai, nasPdu, n2Info)
			}
		}
	}

	if ue.LocationChanged && ue.RequestTriggerLocationChange {
		updateReq := models.PolicyAssociationUpdateRequest{}
		updateReq.Triggers = append(updateReq.Triggers, models.RequestTrigger_LOC_CH)
		updateReq.UserLoc = &ue.Location
		err := consumer.AMPolicyControlUpdate(ue, updateReq)
		if err != nil {
			ue.GmmLog.Errorf("AM Policy Control Update Error: %v", err)
		}
		ue.LocationChanged = false
	}

	amfSelf.AllocateRegistrationArea(ue, anType)
	assignLadnInfo(ue, anType)

	if ue.RanUe[anType].UeContextRequest {
		if anType == models.AccessType__3_GPP_ACCESS {
			err := gmm_message.SendRegistrationAccept(ue, anType, pduSessionStatus, reactivationResult, errPduSessionId, errCause, &ctxList)
			if err != nil {
				return fmt.Errorf("error sending registration accept: %v", err)
			}
			ue.GmmLog.Infof("sent registration accept")
		} else {
			err := ngap_message.SendInitialContextSetupRequest(ue, anType, nil, &ctxList, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Infof("sent initial context setup request")
			registrationAccept, err := gmm_message.BuildRegistrationAccept(ue, anType,
				pduSessionStatus, reactivationResult, errPduSessionId, errCause)
			if err != nil {
				ue.GmmLog.Errorf("Build Registration Accept: %+v", err)
				return nil
			}
			ue.RegistrationAcceptForNon3GPPAccess = registrationAccept
		}
		return nil
	} else {
		nasPdu, err := gmm_message.BuildRegistrationAccept(ue, anType, pduSessionStatus, reactivationResult, errPduSessionId, errCause)
		if err != nil {
			return fmt.Errorf("error building registration accept: %v", err)
		}
		if len(suList.List) != 0 {
			err := ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nasPdu, suList)
			if err != nil {
				return fmt.Errorf("error sending pdu session resource setup request: %v", err)
			}
			ue.GmmLog.Infof("sent pdu session resource setup request")
		} else {
			err := ngap_message.SendDownlinkNasTransport(ue.RanUe[anType], nasPdu, nil)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}
			ue.GmmLog.Infof("sent downlink nas transport message")
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
			PlmnId: &models.PlmnId{
				Mcc: plmnID[:3],
				Mnc: plmnID[3:],
			},
			Tac: tac,
		}

		ue.LastVisitedRegisteredTai = tai
		ue.GmmLog.Debugf("Ue Last Visited Registered Tai; %v", ue.LastVisitedRegisteredTai)
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

func communicateWithUDM(ue *context.AmfUe, accessType models.AccessType) error {
	err := consumer.UeCmRegistration(ue, accessType, true)
	if err != nil {
		ue.GmmLog.Errorf("UECM_Registration Error[%+v]", err)
	}

	err = consumer.SDMGetAmData(ue)
	if err != nil {
		return fmt.Errorf("SDM_Get AmData Error[%+v]", err)
	}

	err = consumer.SDMGetSmfSelectData(ue)
	if err != nil {
		return fmt.Errorf("SDM_Get SmfSelectData Error[%+v]", err)
	}

	err = consumer.SDMGetUeContextInSmfData(ue)
	if err != nil {
		return fmt.Errorf("SDM_Get UeContextInSmfData Error[%+v]", err)
	}

	err = consumer.SDMSubscribe(ue)
	if err != nil {
		return fmt.Errorf("SDM Subscribe Error[%+v]", err)
	}
	ue.SubscriptionDataValid = true
	return nil
}

func getSubscribedNssai(ue *context.AmfUe) {
	err := consumer.SDMGetSliceSelectionSubscriptionData(ue)
	if err != nil {
		ue.GmmLog.Errorf("SDM_Get Slice Selection Subscription Data Error[%+v]", err)
	}
}

// TS 23.502 4.2.2.2.3 Registration with AMF Re-allocation
func handleRequestedNssai(ue *context.AmfUe, anType models.AccessType) error {
	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.RequestedNSSAI != nil {
		requestedNssai, err := util.RequestedNssaiToModels(ue.RegistrationRequest.RequestedNSSAI)
		if err != nil {
			return fmt.Errorf("failed to decode requested NSSAI[%s]", err)
		}

		ue.GmmLog.Infof("RequestedNssai: %+v", requestedNssai)

		needSliceSelection := false
		for _, requestedSnssai := range requestedNssai {
			if ue.InSubscribedNssai(requestedSnssai.ServingSnssai) {
				allowedSnssai := models.AllowedSnssai{
					AllowedSnssai: &models.Snssai{
						Sst: requestedSnssai.ServingSnssai.Sst,
						Sd:  requestedSnssai.ServingSnssai.Sd,
					},
					MappedHomeSnssai: requestedSnssai.HomeSnssai,
				}
				ue.AllowedNssai[anType] = append(ue.AllowedNssai[anType], allowedSnssai)
			} else {
				needSliceSelection = true
				break
			}
		}

		if needSliceSelection {
			// Step 4
			err := consumer.NSSelectionGetForRegistration(ue, requestedNssai)
			if err != nil {
				err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMProtocolErrorUnspecified, "")
				if err != nil {
					return fmt.Errorf("error sending registration reject: %v", err)
				}
				ue.GmmLog.Infof("sent registration reject to UE")
				return fmt.Errorf("failed to get network slice selection: %s", err)
			}

			// Step 5: Initial AMF send Namf_Communication_RegistrationCompleteNotify to old AMF
			req := models.UeRegStatusUpdateReqData{
				TransferStatus: models.UeContextTransferStatus_NOT_TRANSFERRED,
			}
			_, err = consumer.RegistrationStatusUpdate(ue, req)
			if err != nil {
				ue.GmmLog.Errorf("Registration Status Update Error[%+v]", err)
			}

			// Guillaume: I'm not sure if what we have here is the right thing to do
			// As we removed the NRF, we don't search for other AMF's anymore and we hardcode the
			// target AMF to the AMF's own address.
			// It's possible we need to change this whole block to the following:
			//  allowedNssaiNgap := ngapConvert.AllowedNssaiToNgap(ue.AllowedNssai[anType])
			//	ngap_message.SendRerouteNasRequest(ue, anType, nil, ue.RanUe[anType].InitialUEMessage, &allowedNssaiNgap)
			ue.TargetAmfUri = amfSelf.GetIPv4Uri()
			ueContext := consumer.BuildUeContextModel(ue)
			registerContext := models.RegistrationContextContainer{
				UeContext:        &ueContext,
				AnType:           anType,
				AnN2ApId:         int32(ue.RanUe[anType].RanUeNgapId),
				RanNodeId:        ue.RanUe[anType].Ran.RanId,
				InitialAmfName:   amfSelf.Name,
				UserLocation:     &ue.Location,
				RrcEstCause:      ue.RanUe[anType].RRCEstablishmentCause,
				UeContextRequest: ue.RanUe[anType].UeContextRequest,
				AnN2IPv4Addr:     ue.RanUe[anType].Ran.GnbIp,
				AllowedNssai: &models.AllowedNssai{
					AllowedSnssaiList: ue.AllowedNssai[anType],
					AccessType:        anType,
				},
			}
			if len(ue.NetworkSliceInfo.RejectedNssaiInPlmn) > 0 {
				registerContext.RejectedNssaiInPlmn = ue.NetworkSliceInfo.RejectedNssaiInPlmn
			}
			if len(ue.NetworkSliceInfo.RejectedNssaiInTa) > 0 {
				registerContext.RejectedNssaiInTa = ue.NetworkSliceInfo.RejectedNssaiInTa
			}

			var n1Message bytes.Buffer
			ue.RegistrationRequest.EncodeRegistrationRequest(&n1Message)
			return nil
		}
	}

	// if registration request has no requested nssai, or non of snssai in requested nssai is permitted by nssf
	// then use ue subscribed snssai which is marked as default as allowed nssai
	if len(ue.AllowedNssai[anType]) == 0 {
		for _, snssai := range ue.SubscribedNssai {
			if snssai.DefaultIndication {
				if amfSelf.InPlmnSupport(*snssai.SubscribedSnssai) {
					allowedSnssai := models.AllowedSnssai{
						AllowedSnssai: snssai.SubscribedSnssai,
					}
					ue.AllowedNssai[anType] = append(ue.AllowedNssai[anType], allowedSnssai)
				}
			}
		}
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

	ue.GmmLog.Info("Handle Identity Response")

	mobileIdentityContents := identityResponse.MobileIdentity.GetMobileIdentityContents()
	switch nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) { // get type of identity
	case nasMessage.MobileIdentity5GSTypeSuci:
		var plmnId string
		ue.Suci, plmnId = nasConvert.SuciToString(mobileIdentityContents)
		ue.PlmnId = PlmnIdStringToModels(plmnId)
		ue.GmmLog.Debugf("get SUCI: %s", ue.Suci)
	case nasMessage.MobileIdentity5GSType5gGuti:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		_, guti := nasConvert.GutiToString(mobileIdentityContents)
		ue.Guti = guti
		ue.GmmLog.Debugf("get GUTI: %s", guti)
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
		ue.GmmLog.Debugf("get 5G-S-TMSI: %s", sTmsi)
	case nasMessage.MobileIdentity5GSTypeImei:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		imei := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imei
		ue.GmmLog.Debugf("get PEI: %s", imei)
	case nasMessage.MobileIdentity5GSTypeImeisv:
		if ue.MacFailed {
			return fmt.Errorf("NAS message integrity check failed")
		}
		imeisv := nasConvert.PeiToString(mobileIdentityContents)
		ue.Pei = imeisv
		ue.GmmLog.Debugf("get PEI: %s", imeisv)
	}
	return nil
}

// TS 24501 5.6.3.2
func HandleNotificationResponse(ue *context.AmfUe, notificationResponse *nasMessage.NotificationResponse) error {
	ue.GmmLog.Info("Handle Notification Response")

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
			pduSessionId := int32(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionId); ok {
				if !psiArray[psi] {
					err := consumer.SendReleaseSmContextRequest(smContext)
					if err != nil {
						ue.GmmLog.Errorf("Release SmContext Error[%v]", err.Error())
					}
				}
			}
		}
	}
	return nil
}

func HandleConfigurationUpdateComplete(ue *context.AmfUe, configurationUpdateComplete *nasMessage.ConfigurationUpdateComplete) error {
	ue.GmmLog.Info("Handle Configuration Update Complete")
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	return nil
}

func AuthenticationProcedure(ue *context.AmfUe, accessType models.AccessType) (bool, error) {
	ue.GmmLog.Info("Authentication procedure")

	// Check whether UE has SUCI and SUPI
	if IdentityVerification(ue) {
		ue.GmmLog.Debugln("UE has SUCI / SUPI")
		if ue.SecurityContextIsValid() {
			ue.GmmLog.Debugln("UE has a valid security context - skip the authentication procedure")
			return true, nil
		}
	} else {
		// Request UE's SUCI by sending identity request
		err := gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
		if err != nil {
			return false, fmt.Errorf("error sending identity request: %v", err)
		}
		ue.GmmLog.Infof("sent identity request")
		return false, nil
	}

	response, err := consumer.SendUEAuthenticationAuthenticateRequest(ue, nil)
	if err != nil {
		return false, fmt.Errorf("Authentication procedure failed: %s", err)
	}
	ue.AuthenticationCtx = response
	ue.ABBA = []uint8{0x00, 0x00} // set ABBA value as described at TS 33.501 Annex A.7.1

	err = gmm_message.SendAuthenticationRequest(ue.RanUe[accessType])
	if err != nil {
		return false, fmt.Errorf("error sending authentication request: %v", err)
	}
	ue.GmmLog.Infof("sent authentication request")
	return false, nil
}

func NetworkInitiatedDeregistrationProcedure(ue *context.AmfUe, accessType models.AccessType) (err error) {
	anType := AnTypeToNas(accessType)
	if ue.CmConnect(accessType) && ue.State[accessType].Is(context.Registered) {
		// setting reregistration required flag to true
		err := gmm_message.SendDeregistrationRequest(ue.RanUe[accessType], anType, true, 0)
		if err != nil {
			return fmt.Errorf("error sending deregistration request: %v", err)
		}
		ue.GmmLog.Infof("sent deregistration request")
	} else {
		SetDeregisteredState(ue, anType)
	}

	ue.SmContextList.Range(func(key, value interface{}) bool {
		smContext := value.(*context.SmContext)

		if smContext.AccessType() == accessType {
			ue.GmmLog.Infof("Sending SmContext [slice: %v, dnn: %v] Release Request to SMF", smContext.Snssai(), smContext.Dnn())
			err = consumer.SendReleaseSmContextRequest(smContext)
			if err != nil {
				ue.GmmLog.Errorf("Release SmContext Error[%v]", err.Error())
			}
		}
		return true
	})

	if ue.AmPolicyAssociation != nil {
		terminateAmPolicyAssocaition := true
		switch accessType {
		case models.AccessType__3_GPP_ACCESS:
			terminateAmPolicyAssocaition = ue.State[models.AccessType_NON_3_GPP_ACCESS].Is(context.Deregistered)
		case models.AccessType_NON_3_GPP_ACCESS:
			terminateAmPolicyAssocaition = ue.State[models.AccessType__3_GPP_ACCESS].Is(context.Deregistered)
		}

		if terminateAmPolicyAssocaition {
			err = consumer.AMPolicyControlDelete(ue)
			if err != nil {
				ue.GmmLog.Errorf("AM Policy Control Delete Error[%v]", err.Error())
			}
			ue.GmmLog.Infof("deleted AM Policy Association")
		}
	}
	// if ue is not connected mode, removing UE Context
	if !ue.State[accessType].Is(context.Registered) {
		if ue.CmConnect(accessType) {
			err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType__3_GPP_ACCESS], context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		} else {
			ue.Remove()
			ue.GmmLog.Infof("removed ue context")
		}
	}
	return err
}

// TS 24501 5.6.1
func HandleServiceRequest(ue *context.AmfUe, anType models.AccessType,
	serviceRequest *nasMessage.ServiceRequest,
) error {
	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	ue.GmmLog.Info("Handle Service Request")

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
		ue.GmmLog.Warnf("UE should not in OnGoing[%s]", procedure)
	}

	// Send Authtication / Security Procedure not support
	// Rejecting ServiceRequest if it is received in Deregistered State
	if !ue.SecurityContextIsValid() || ue.State[anType].Current() == context.Deregistered {
		ue.GmmLog.Warnf("No Security Context : SUPI[%s]", ue.Supi)
		err := gmm_message.SendServiceReject(ue.RanUe[anType], nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.GmmLog.Infof("sent service reject")
		err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		ue.GmmLog.Infof("sent ue context release command")
		return nil
	}

	// TS 24.501 8.2.6.21: if the UE is sending a REGISTRATION REQUEST message as an initial NAS message,
	// the UE has a valid 5G NAS security context and the UE needs to send non-cleartext IEs
	// TS 24.501 4.4.6: When the UE sends a REGISTRATION REQUEST or SERVICE REQUEST message that includes a NAS message
	// container IE, the UE shall set the security header type of the initial NAS message to "integrity protected"
	if serviceRequest.NASMessageContainer != nil {
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
			if messageType != nas.MsgTypeServiceRequest {
				return errors.New("The payload of NAS message Container is not service request")
			}
			// TS 24.501 4.4.6: The AMF shall consider the NAS message that is obtained from the NAS message container
			// IE as the initial NAS message that triggered the procedure
			serviceRequest = m.ServiceRequest
		}
		// TS 33.501 6.4.6 step 3: if the initial NAS message was protected but did not pass the integrity check
		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	serviceType := serviceRequest.GetServiceTypeValue()
	var reactivationResult, acceptPduSessionPsi *[16]bool
	var errPduSessionId, errCause []uint8
	var targetPduSessionId int32
	suList := ngapType.PDUSessionResourceSetupListSUReq{}
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}

	if serviceType == nasMessage.ServiceTypeEmergencyServices ||
		serviceType == nasMessage.ServiceTypeEmergencyServicesFallback {
		ue.GmmLog.Warnf("emergency service is not supported")
	}

	if ue.MacFailed {
		ue.SecurityContextAvailable = false
		ue.GmmLog.Warnf("Security Context Exist, But Integrity Check Failed with existing Context: SUPI[%s]", ue.Supi)
		err := gmm_message.SendServiceReject(ue.RanUe[anType], nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.GmmLog.Infof("sent service reject")
		err = ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		ue.GmmLog.Infof("sent ue context release command")
		return nil
	}

	ue.RanUe[anType].UeContextRequest = true
	if serviceType == nasMessage.ServiceTypeSignalling {
		err := sendServiceAccept(ue, anType, ctxList, suList, nil, nil, nil, nil)
		return err
	}
	if ue.N1N2Message != nil {
		requestData := ue.N1N2Message.Request.JsonData
		if ue.N1N2Message.Request.BinaryDataN2Information != nil {
			if requestData.N2InfoContainer.N2InformationClass == models.N2InformationClass_SM {
				targetPduSessionId = requestData.N2InfoContainer.SmInfo.PduSessionId
			} else {
				ue.N1N2Message = nil
				return fmt.Errorf("Service Request triggered by Network has not implemented about non SM N2Info")
			}
		}
	}

	if serviceRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(serviceRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)
		ue.SmContextList.Range(func(key, value interface{}) bool {
			pduSessionID := key.(int32)
			smContext := value.(*context.SmContext)

			if pduSessionID != targetPduSessionId {
				if uplinkDataPsi[pduSessionID] && smContext.AccessType() == models.AccessType__3_GPP_ACCESS {
					response, err := consumer.SendUpdateSmContextActivateUpCnxState(
						ue, smContext, models.AccessType__3_GPP_ACCESS)
					if err != nil {
						ue.GmmLog.Errorf("SendUpdateSmContextActivateUpCnxState[pduSessionID:%d] Error: %+v",
							pduSessionID, err)
					} else if response == nil {
						reactivationResult[pduSessionID] = true
						errPduSessionId = append(errPduSessionId, uint8(pduSessionID))
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
		ue.SmContextList.Range(func(key, value interface{}) bool {
			pduSessionID := key.(int32)
			smContext := value.(*context.SmContext)
			if smContext.AccessType() == anType {
				if !psiArray[pduSessionID] {
					err := consumer.SendReleaseSmContextRequest(smContext)
					if err != nil {
						ue.GmmLog.Errorf("Release SmContext Error[%v]", err.Error())
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
			requestData := ue.N1N2Message.Request.JsonData
			n1Msg := ue.N1N2Message.Request.BinaryDataN1Message
			n2Info := ue.N1N2Message.Request.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				err := sendServiceAccept(ue, anType, ctxList, suList, acceptPduSessionPsi,
					reactivationResult, errPduSessionId, errCause)
				if err != nil {
					return err
				}
				switch requestData.N1MessageContainer.N1MessageClass {
				case models.N1MessageClass_SM:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionId, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message")
				case models.N1MessageClass_LPP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message")
				case models.N1MessageClass_SMS:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message")
				case models.N1MessageClass_UPDP:
					err := gmm_message.SendDLNASTransport(ue.RanUe[anType], nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Infof("sent downlink nas transport message")
				}
				ue.N1N2Message = nil
				return nil
			}
			smInfo := requestData.N2InfoContainer.SmInfo
			smContext, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionId)
			if !exist {
				ue.N1N2Message = nil
				return fmt.Errorf("service Request triggered by Network error for pduSessionId does not exist")
			}

			if smContext.AccessType() == models.AccessType_NON_3_GPP_ACCESS {
				if serviceRequest.AllowedPDUSessionStatus != nil {
					allowPduSessionPsi := nasConvert.PSIToBooleanArray(serviceRequest.AllowedPDUSessionStatus.Buffer)
					if reactivationResult == nil {
						reactivationResult = new([16]bool)
					}
					if allowPduSessionPsi[requestData.PduSessionId] {
						response, err := consumer.SendUpdateSmContextChangeAccessType(
							ue, smContext, true)
						if err != nil {
							return err
						} else if response == nil {
							reactivationResult[requestData.PduSessionId] = true
							errPduSessionId = append(errPduSessionId, uint8(requestData.PduSessionId))
							cause := nasMessage.Cause5GMMProtocolErrorUnspecified
							errCause = append(errCause, cause)
						} else {
							smContext.SetUserLocation(ue.Location)
							smContext.SetAccessType(models.AccessType__3_GPP_ACCESS)
							if response.BinaryDataN2SmInformation != nil &&
								response.JsonData.N2SmInfoType == models.N2SmInfoType_PDU_RES_SETUP_REQ {
								if ue.RanUe[anType].UeContextRequest {
									ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList,
										requestData.PduSessionId, smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
								} else {
									ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList,
										requestData.PduSessionId, smContext.Snssai(), nil, response.BinaryDataN2SmInformation)
								}
							}
						}
					} else {
						ue.GmmLog.Warnf("UE was reachable but did not accept to re-activate the PDU Session[%d]",
							requestData.PduSessionId)
					}
				}
			} else if smInfo.N2InfoContent.NgapIeType == models.NgapIeType_PDU_RES_SETUP_REQ {
				var nasPdu []byte
				var err error
				if n1Msg != nil {
					pduSessionId := uint8(smInfo.PduSessionId)
					nasPdu, err = gmm_message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionId, nil)
					if err != nil {
						return err
					}
				}
				omecSnssai := models.Snssai{
					Sst: smInfo.SNssai.Sst,
					Sd:  smInfo.SNssai.Sd,
				}
				if ue.RanUe[anType].UeContextRequest {
					ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, smInfo.PduSessionId, omecSnssai, nasPdu, n2Info)
				} else {
					ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, smInfo.PduSessionId, omecSnssai, nasPdu, n2Info)
				}
			}
			err := sendServiceAccept(ue, anType, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionId, errCause)
			if err != nil {
				return err
			}
		}
		// downlink signaling
		if ue.ConfigurationUpdateMessage != nil {
			err := sendServiceAccept(ue, anType, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionId, errCause)
			if err != nil {
				return err
			}
			mobilityRestrictionList := ngap_message.BuildIEMobilityRestrictionList(ue)
			err = ngap_message.SendDownlinkNasTransport(ue.RanUe[models.AccessType__3_GPP_ACCESS], ue.ConfigurationUpdateMessage, &mobilityRestrictionList)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}
			ue.GmmLog.Infof("sent downlink nas transport")
			ue.ConfigurationUpdateMessage = nil
		}
	case nasMessage.ServiceTypeData:
		if anType == models.AccessType__3_GPP_ACCESS {
			if ue.AmPolicyAssociation != nil && ue.AmPolicyAssociation.ServAreaRes != nil {
				var accept bool
				switch ue.AmPolicyAssociation.ServAreaRes.RestrictionType {
				case models.RestrictionType_ALLOWED_AREAS:
					accept = context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
				case models.RestrictionType_NOT_ALLOWED_AREAS:
					accept = !context.TacInAreas(ue.Tai.Tac, ue.AmPolicyAssociation.ServAreaRes.Areas)
				}

				if !accept {
					err := gmm_message.SendServiceReject(ue.RanUe[anType], nil, nasMessage.Cause5GMMRestrictedServiceArea)
					if err != nil {
						return fmt.Errorf("error sending service reject: %v", err)
					}
					ue.GmmLog.Infof("sent service reject")
					return nil
				}
			}
			err := sendServiceAccept(ue, anType, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionId, errCause)
			if err != nil {
				return err
			}
		} else {
			err := sendServiceAccept(ue, anType, ctxList, suList, acceptPduSessionPsi,
				reactivationResult, errPduSessionId, errCause)
			if err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("service type is not supported: %d", serviceType)
	}
	if len(errPduSessionId) != 0 {
		ue.GmmLog.Info(errPduSessionId, errCause)
	}
	ue.N1N2Message = nil
	return nil
}

func sendServiceAccept(ue *context.AmfUe, anType models.AccessType, ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq, pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool, errPduSessionId, errCause []uint8,
) error {
	if ue.RanUe[anType].UeContextRequest {
		// update Kgnb/Kn3iwf
		ue.UpdateSecurityContext(anType)

		nasPdu, err := gmm_message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionId, errCause)
		if err != nil {
			return err
		}
		if len(ctxList.List) != 0 {
			err := ngap_message.SendInitialContextSetupRequest(ue, anType, nasPdu, &ctxList, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Infof("sent initial context setup request")
		} else {
			err := ngap_message.SendInitialContextSetupRequest(ue, anType, nasPdu, nil, nil, nil, nil)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Infof("sent initial context setup request")
		}
	} else if len(suList.List) != 0 {
		nasPdu, err := gmm_message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionId, errCause)
		if err != nil {
			return err
		}
		err = ngap_message.SendPDUSessionResourceSetupRequest(ue.RanUe[anType], nasPdu, suList)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource setup request: %v", err)
		}
		ue.GmmLog.Infof("sent pdu session resource setup request")
	} else {
		err := gmm_message.SendServiceAccept(ue.RanUe[anType], pDUSessionStatus, reactivationResult, errPduSessionId, errCause)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}
		ue.GmmLog.Infof("sent service accept")
	}
	return nil
}

// TS 24.501 5.4.1
func HandleAuthenticationResponse(ue *context.AmfUe, accessType models.AccessType, authenticationResponse *nasMessage.AuthenticationResponse) error {
	ue.GmmLog.Info("Handle Authentication Response")

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.AuthenticationCtx == nil {
		return fmt.Errorf("ue Authentication Context is nil")
	}

	switch ue.AuthenticationCtx.AuthType {
	case models.AuthType__5_G_AKA:
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
			ue.GmmLog.Errorf("HRES* Validation Failure (received: %s, expected: %s)", hResStar, av5gAka.HxresStar)

			if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
				err := gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
				if err != nil {
					return fmt.Errorf("send identity request error: %s", err)
				}
				ue.GmmLog.Infof("sent identity request")
				return nil
			} else {
				err := gmm_message.SendAuthenticationReject(ue.RanUe[accessType], "")
				if err != nil {
					return fmt.Errorf("error sending authentication reject: %v", err)
				}
				ue.GmmLog.Infof("sent authentication reject")
				return GmmFSM.SendEvent(ue.State[accessType], AuthFailEvent, fsm.ArgsType{
					ArgAmfUe:      ue,
					ArgAccessType: accessType,
				})
			}
		}

		response, err := consumer.SendAuth5gAkaConfirmRequest(ue, hex.EncodeToString(resStar[:]))
		if err != nil {
			return fmt.Errorf("Authentication procedure failed: %s", err)
		}
		switch response.AuthResult {
		case models.AuthResult_SUCCESS:
			ue.UnauthenticatedSupi = false
			ue.Kseaf = response.Kseaf
			ue.Supi = response.Supi
			ue.DerivateKamf()
			return GmmFSM.SendEvent(ue.State[accessType], AuthSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      ue,
				ArgAccessType: accessType,
				ArgEAPSuccess: false,
				ArgEAPMessage: "",
			})
		case models.AuthResult_FAILURE:
			if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
				err := gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
				if err != nil {
					return fmt.Errorf("send identity request error: %s", err)
				}
				ue.GmmLog.Infof("sent identity request")
				return nil
			} else {
				err := gmm_message.SendAuthenticationReject(ue.RanUe[accessType], "")
				if err != nil {
					return fmt.Errorf("error sending authentication reject: %v", err)
				}
				ue.GmmLog.Infof("sent authentication reject")
				return GmmFSM.SendEvent(ue.State[accessType], AuthFailEvent, fsm.ArgsType{
					ArgAmfUe:      ue,
					ArgAccessType: accessType,
				})
			}
		}
	case models.AuthType_EAP_AKA_PRIME:
		response, err := consumer.SendEapAuthConfirmRequest(ue.Suci, *authenticationResponse.EAPMessage)
		if err != nil {
			return err
		}

		switch response.AuthResult {
		case models.AuthResult_SUCCESS:
			ue.UnauthenticatedSupi = false
			ue.Kseaf = response.KSeaf
			ue.Supi = response.Supi
			ue.DerivateKamf()
			return GmmFSM.SendEvent(ue.State[accessType], SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      ue,
				ArgAccessType: accessType,
				ArgEAPSuccess: true,
				ArgEAPMessage: response.EapPayload,
			})
		case models.AuthResult_FAILURE:
			if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
				err := gmm_message.SendAuthenticationResult(ue.RanUe[accessType], false, response.EapPayload)
				if err != nil {
					return fmt.Errorf("send authentication result error: %s", err)
				}
				ue.GmmLog.Infof("sent authentication result")
				err = gmm_message.SendIdentityRequest(ue.RanUe[accessType], nasMessage.MobileIdentity5GSTypeSuci)
				if err != nil {
					return fmt.Errorf("send identity request error: %s", err)
				}
				ue.GmmLog.Infof("sent identity request")
				return nil
			} else {
				err := gmm_message.SendAuthenticationReject(ue.RanUe[accessType], response.EapPayload)
				if err != nil {
					return fmt.Errorf("error sending authentication reject: %v", err)
				}
				ue.GmmLog.Infof("sent authentication reject")
				return GmmFSM.SendEvent(ue.State[accessType], AuthFailEvent, fsm.ArgsType{
					ArgAmfUe:      ue,
					ArgAccessType: accessType,
				})
			}
		case models.AuthResult_ONGOING:
			ue.AuthenticationCtx.Var5gAuthData = response.EapPayload
			err := gmm_message.SendAuthenticationRequest(ue.RanUe[accessType])
			if err != nil {
				return fmt.Errorf("send authentication request error: %s", err)
			}
			ue.GmmLog.Infof("sent authentication request")
		}
	}

	return nil
}

func HandleAuthenticationFailure(ue *context.AmfUe, anType models.AccessType, authenticationFailure *nasMessage.AuthenticationFailure) error {
	ue.GmmLog.Info("Handle Authentication Failure")

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause5GMM := authenticationFailure.Cause5GMM.GetCauseValue()

	if ue.AuthenticationCtx.AuthType == models.AuthType__5_G_AKA {
		switch cause5GMM {
		case nasMessage.Cause5GMMMACFailure:
			ue.GmmLog.Warnln("Authentication Failure Cause: Mac Failure")
			err := gmm_message.SendAuthenticationReject(ue.RanUe[anType], "")
			if err != nil {
				return fmt.Errorf("error sending authentication reject: %v", err)
			}
			ue.GmmLog.Infof("sent authentication reject")
			return GmmFSM.SendEvent(ue.State[anType], AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue, ArgAccessType: anType})
		case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
			ue.GmmLog.Warnln("Authentication Failure Cause: Non-5G Authentication Unacceptable")
			err := gmm_message.SendAuthenticationReject(ue.RanUe[anType], "")
			if err != nil {
				return fmt.Errorf("error sending authentication reject: %v", err)
			}
			ue.GmmLog.Infof("sent authentication reject")
			return GmmFSM.SendEvent(ue.State[anType], AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue, ArgAccessType: anType})
		case nasMessage.Cause5GMMngKSIAlreadyInUse:
			ue.GmmLog.Warnln("Authentication Failure Cause: NgKSI Already In Use")
			ue.AuthFailureCauseSynchFailureTimes = 0
			ue.GmmLog.Warnln("Select new NgKsi")
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
			ue.GmmLog.Infof("sent authentication request")
		case nasMessage.Cause5GMMSynchFailure: // TS 24.501 5.4.1.3.7 case f
			ue.GmmLog.Warn("Authentication Failure 5GMM Cause: Synch Failure")

			ue.AuthFailureCauseSynchFailureTimes++
			if ue.AuthFailureCauseSynchFailureTimes >= 2 {
				ue.GmmLog.Warnf("2 consecutive Synch Failure, terminate authentication procedure")
				err := gmm_message.SendAuthenticationReject(ue.RanUe[anType], "")
				if err != nil {
					return fmt.Errorf("error sending authentication reject: %v", err)
				}
				ue.GmmLog.Infof("sent authentication reject")
				return GmmFSM.SendEvent(ue.State[anType], AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue, ArgAccessType: anType})
			}

			auts := authenticationFailure.AuthenticationFailureParameter.GetAuthenticationFailureParameter()
			resynchronizationInfo := &models.ResynchronizationInfo{
				Auts: hex.EncodeToString(auts[:]),
			}

			response, err := consumer.SendUEAuthenticationAuthenticateRequest(ue, resynchronizationInfo)
			if err != nil {
				return fmt.Errorf("send UE Authentication Authenticate Request Error: %s", err.Error())
			}
			ue.AuthenticationCtx = response
			ue.ABBA = []uint8{0x00, 0x00}

			err = gmm_message.SendAuthenticationRequest(ue.RanUe[anType])
			if err != nil {
				return fmt.Errorf("send authentication request error: %s", err)
			}
			ue.GmmLog.Infof("sent authentication request")
		}
	} else if ue.AuthenticationCtx.AuthType == models.AuthType_EAP_AKA_PRIME {
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
			ue.GmmLog.Infof("sent authentication request")
		}
	}

	return nil
}

func HandleRegistrationComplete(ue *context.AmfUe, accessType models.AccessType, registrationComplete *nasMessage.RegistrationComplete) error {
	ue.GmmLog.Info("Handle Registration Complete")

	if ue.T3550 != nil {
		ue.T3550.Stop()
		ue.T3550 = nil // clear the timer
	}

	if ue.RegistrationRequest.UplinkDataStatus == nil &&
		ue.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestNoPending {
		err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[accessType], context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		ue.GmmLog.Infof("sent ue context release command")
	}

	return GmmFSM.SendEvent(ue.State[accessType], ContextSetupSuccessEvent, fsm.ArgsType{
		ArgAmfUe:      ue,
		ArgAccessType: accessType,
	})
}

// TS 33.501 6.7.2
func HandleSecurityModeComplete(ue *context.AmfUe, anType models.AccessType, procedureCode int64,
	securityModeComplete *nasMessage.SecurityModeComplete,
) error {
	ue.GmmLog.Info("Handle Security Mode Complete")

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
		ue.GmmLog.Debugln("receieve IMEISV")
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
			ue.GmmLog.Errorln("nas message container Iei type error")
			return errors.New("nas message container Iei type error")
		} else {
			return GmmFSM.SendEvent(ue.State[anType], SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:         ue,
				ArgAccessType:    anType,
				ArgProcedureCode: procedureCode,
				ArgNASMessage:    m.GmmMessage.RegistrationRequest,
			})
		}
	}
	return GmmFSM.SendEvent(ue.State[anType], SecurityModeSuccessEvent, fsm.ArgsType{
		ArgAmfUe:         ue,
		ArgAccessType:    anType,
		ArgProcedureCode: procedureCode,
		ArgNASMessage:    ue.RegistrationRequest,
	})
}

func HandleSecurityModeReject(ue *context.AmfUe, anType models.AccessType,
	securityModeReject *nasMessage.SecurityModeReject,
) error {
	ue.GmmLog.Info("Handle Security Mode Reject")

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause := securityModeReject.Cause5GMM.GetCauseValue()
	ue.GmmLog.Warnf("Reject Cause: %s", nasMessage.Cause5GMMToString(cause))
	ue.GmmLog.Error("UE reject the security mode command, abort the ongoing procedure")

	ue.SecurityContextAvailable = false

	err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[anType], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}
	ue.GmmLog.Infof("sent ue context release command")
	return nil
}

// TS 23.502 4.2.2.3
func HandleDeregistrationRequest(ue *context.AmfUe, anType models.AccessType,
	deregistrationRequest *nasMessage.DeregistrationRequestUEOriginatingDeregistration,
) error {
	ue.GmmLog.Info("Handle Deregistration Request(UE Originating)")

	targetDeregistrationAccessType := deregistrationRequest.GetAccessType()
	ue.SmContextList.Range(func(key, value interface{}) bool {
		smContext := value.(*context.SmContext)

		if smContext.AccessType() == anType ||
			targetDeregistrationAccessType == nasMessage.AccessTypeBoth {
			err := consumer.SendReleaseSmContextRequest(smContext)
			if err != nil {
				ue.GmmLog.Errorf("Release SmContext Error[%v]", err.Error())
			}
		}
		return true
	})

	if ue.AmPolicyAssociation != nil {
		terminateAmPolicyAssocaition := true
		switch anType {
		case models.AccessType__3_GPP_ACCESS:
			terminateAmPolicyAssocaition = ue.State[models.AccessType_NON_3_GPP_ACCESS].Is(context.Deregistered)
		case models.AccessType_NON_3_GPP_ACCESS:
			terminateAmPolicyAssocaition = ue.State[models.AccessType__3_GPP_ACCESS].Is(context.Deregistered)
		}

		if terminateAmPolicyAssocaition {
			err := consumer.AMPolicyControlDelete(ue)
			if err != nil {
				ue.GmmLog.Errorf("AM Policy Control Delete Error[%v]", err.Error())
			}
		}
	}

	// if Deregistration type is not switch-off, send Deregistration Accept
	if deregistrationRequest.GetSwitchOff() == 0 && ue.RanUe[anType] != nil {
		err := gmm_message.SendDeregistrationAccept(ue.RanUe[anType])
		if err != nil {
			return fmt.Errorf("error sending deregistration accept: %v", err)
		}
		ue.GmmLog.Infof("sent deregistration accept")
	}

	// TS 23.502 4.2.6, 4.12.3
	switch targetDeregistrationAccessType {
	case nasMessage.AccessType3GPP:
		if ue.RanUe[models.AccessType__3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType__3_GPP_ACCESS], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
		return GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
	case nasMessage.AccessTypeNon3GPP:
		if ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType_NON_3_GPP_ACCESS], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
		return GmmFSM.SendEvent(ue.State[models.AccessType_NON_3_GPP_ACCESS], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
	case nasMessage.AccessTypeBoth:
		if ue.RanUe[models.AccessType__3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType__3_GPP_ACCESS], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
		if ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType_NON_3_GPP_ACCESS], context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}

		err := GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
		if err != nil {
			ue.GmmLog.Errorln(err)
		}
		return GmmFSM.SendEvent(ue.State[models.AccessType_NON_3_GPP_ACCESS], DeregistrationAcceptEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgAccessType: anType,
		})
	}

	return nil
}

// TS 23.502 4.2.2.3
func HandleDeregistrationAccept(ue *context.AmfUe, anType models.AccessType,
	deregistrationAccept *nasMessage.DeregistrationAcceptUETerminatedDeregistration,
) error {
	ue.GmmLog.Info("Handle Deregistration Accept(UE Terminated)")

	if ue.T3522 != nil {
		ue.T3522.Stop()
		ue.T3522 = nil // clear the timer
	}

	switch ue.DeregistrationTargetAccessType {
	case nasMessage.AccessType3GPP:
		if ue.RanUe[models.AccessType__3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType__3_GPP_ACCESS], context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
	case nasMessage.AccessTypeNon3GPP:
		if ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType_NON_3_GPP_ACCESS],
				context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
	case nasMessage.AccessTypeBoth:
		if ue.RanUe[models.AccessType__3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType__3_GPP_ACCESS],
				context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
		if ue.RanUe[models.AccessType_NON_3_GPP_ACCESS] != nil {
			err := ngap_message.SendUEContextReleaseCommand(ue.RanUe[models.AccessType_NON_3_GPP_ACCESS],
				context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
			if err != nil {
				return fmt.Errorf("error sending ue context release command: %v", err)
			}
			ue.GmmLog.Infof("sent ue context release command")
		}
	}

	ue.DeregistrationTargetAccessType = 0

	return GmmFSM.SendEvent(ue.State[models.AccessType__3_GPP_ACCESS], DeregistrationAcceptEvent, fsm.ArgsType{
		ArgAmfUe:      ue,
		ArgAccessType: anType,
	})
}

func HandleStatus5GMM(ue *context.AmfUe, anType models.AccessType, status5GMM *nasMessage.Status5GMM) error {
	ue.GmmLog.Info("Handle Staus 5GMM")
	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	cause := status5GMM.Cause5GMM.GetCauseValue()
	ue.GmmLog.Errorf("Error condition [Cause Value: %s]", nasMessage.Cause5GMMToString(cause))
	return nil
}

func HandleAuthenticationError(ue *context.AmfUe, anType models.AccessType) error {
	ue.GmmLog.Error("Handle Authentication Error")
	if ue.RegistrationRequest != nil {
		err := gmm_message.SendRegistrationReject(ue.RanUe[anType], nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork, "")
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Infof("sent registration reject")
	}
	return nil
}
