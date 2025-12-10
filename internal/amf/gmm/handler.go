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
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/nas/handler")

func PlmnIDStringToModels(plmnIDStr string) models.PlmnID {
	var plmnID models.PlmnID
	plmnID.Mcc = plmnIDStr[:3]
	plmnID.Mnc = plmnIDStr[3:]
	return plmnID
}

func HandleULNASTransport(ctx ctxt.Context, ue *context.AmfUe, ulNasTransport *nasMessage.ULNASTransport) error {
	ctx, span := tracer.Start(ctx, "AMF HandleULNASTransport")
	defer span.End()

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	switch ulNasTransport.GetPayloadContainerType() {
	// TS 24.501 5.4.5.2.3 case a)
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return transport5GSMMessage(ctx, ue, ulNasTransport)
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

func transport5GSMMessage(ctx ctxt.Context, ue *context.AmfUe, ulNasTransport *nasMessage.ULNASTransport) error {
	var pduSessionID int32
	smMessage := ulNasTransport.PayloadContainer.GetPayloadContainerContents()

	id := ulNasTransport.PduSessionID2Value
	if id == nil {
		return fmt.Errorf("pdu session id is nil")
	}
	pduSessionID = int32(id.GetPduSessionID2Value())

	if ulNasTransport.OldPDUSessionID != nil {
		return fmt.Errorf("old pdu session id is not supported")
	}
	// case 1): looks up a PDU session routing context for the UE and the PDU session ID IE in case the Old PDU
	// session ID IE is not included
	smContext, smContextExist := ue.SmContextFindByPDUSessionID(pduSessionID)
	requestType := ulNasTransport.RequestType

	if requestType != nil {
		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest:
			fallthrough
		case nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
			ue.GmmLog.Warn("Emergency PDU Session is not supported")
			err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.GmmLog.Info("sent downlink nas transport to UE")
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
			return forward5GSMMessageToSMF(ctx, ue, pduSessionID, smContext, smMessage)
		}

		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialRequest:
			//  perform a local release of the PDU session identified by the PDU session ID and shall request
			// the SMF to perform a local release of the PDU session
			updateData := models.SmContextUpdateData{
				Cause: models.CauseRelDueToDuplicateSessionID,
			}
			ue.GmmLog.Warn("Duplicated PDU session ID", zap.Int32("pduSessionID", pduSessionID))
			response, err := consumer.SendUpdateSmContextRequest(ctx, smContext, updateData, nil, nil)
			if err != nil {
				return err
			}
			if response == nil {
				ue.GmmLog.Error("PDU Session can't be released in DUPLICATE_SESSION_ID case", zap.Int32("pduSessionID", pduSessionID))
				err = gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}
				ue.GmmLog.Info("sent downlink nas transport to UE")
			} else {
				responseData := response.JSONData
				n2Info := response.BinaryDataN2SmInformation
				if n2Info != nil {
					switch responseData.N2SmInfoType {
					case models.N2SmInfoTypePduResRelCmd:
						ue.GmmLog.Debug("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
						list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
						ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2Info)
						err := ngap_message.SendPDUSessionResourceReleaseCommand(ctx, ue.RanUe, nil, list)
						if err != nil {
							return fmt.Errorf("error sending pdu session resource release command: %s", err)
						}
						ue.GmmLog.Info("sent pdu session resource release command to UE")
					}
				}
			}

		// case ii) AMF has a PDU session routing context, and Request type is "existing PDU session"
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			if ue.InAllowedNssai(smContext.Snssai()) {
				return forward5GSMMessageToSMF(ctx, ue, pduSessionID, smContext, smMessage)
			} else {
				ue.GmmLog.Error("S-NSSAI is not allowed for access type", zap.Any("snssai", smContext.Snssai()), zap.Int32("pduSessionID", pduSessionID))
				err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}
				ue.GmmLog.Info("sent downlink nas transport to UE")
			}
		// other requestType: AMF forward the 5GSM message, and the PDU session ID IE towards the SMF identified
		// by the SMF ID of the PDU session routing context
		default:
			return forward5GSMMessageToSMF(ctx, ue, pduSessionID, smContext, smMessage)
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
				if ue.AllowedNssai == nil {
					return fmt.Errorf("allowed nssai is nil in UE context")
				}
				snssai = *ue.AllowedNssai
			}

			if ulNasTransport.DNN != nil && ulNasTransport.DNN.GetLen() > 0 {
				dnn = ulNasTransport.DNN.GetDNN()
			} else {
				// if user's subscription context obtained from UDM does not contain the default DNN for the,
				// S-NSSAI, the AMF shall use a locally configured DNN as the DNN

				_, dnnResp, err := context.GetSubscriberData(ctx, ue.Supi)
				if err != nil {
					return fmt.Errorf("failed to get subscriber data: %v", err)
				}

				dnn = dnnResp
			}

			newSmContext := consumer.SelectSmf(pduSessionID, snssai, dnn)

			smContextRef, errResponse, err := consumer.SendCreateSmContextRequest(ctx, ue, newSmContext, smMessage)
			if err != nil {
				ue.GmmLog.Error("couldn't send create sm context request", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
			}

			if errResponse != nil {
				err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, errResponse.BinaryDataN1SmMessage, pduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}

				return fmt.Errorf("pdu session establishment request was rejected by SMF for pdu session id %d", pduSessionID)
			}

			newSmContext.SetSmContextRef(smContextRef)

			ue.StoreSmContext(pduSessionID, newSmContext)
			ue.GmmLog.Debug("Created sm context for pdu session", zap.Int32("pduSessionID", pduSessionID))

		case nasMessage.ULNASTransportRequestTypeModificationRequest:
			fallthrough
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.GmmLog.Info("sent downlink nas transport to UE")
		default:
		}
	}
	return nil
}

func forward5GSMMessageToSMF(
	ctx ctxt.Context,
	ue *context.AmfUe,
	pduSessionID int32,
	smContext *context.SmContext,
	smMessage []byte,
) error {
	smContextUpdateData := models.SmContextUpdateData{}

	response, err := consumer.SendUpdateSmContextRequest(ctx, smContext, smContextUpdateData, smMessage, nil)
	if err != nil {
		ue.GmmLog.Error("couldn't send update sm context request", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
		return nil
	} else if response != nil {
		// update SmContext in AMF

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
				err := ngap_message.SendPDUSessionResourceModifyRequest(ctx, ue.RanUe, list)
				if err != nil {
					return fmt.Errorf("error sending pdu session resource modify request: %s", err)
				}
				ue.GmmLog.Info("sent pdu session resource modify request to UE")
			case models.N2SmInfoTypePduResRelCmd:
				list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
				ngap_message.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2SmInfo)
				err := ngap_message.SendPDUSessionResourceReleaseCommand(ctx, ue.RanUe, n1Msg, list)
				if err != nil {
					return fmt.Errorf("error sending pdu session resource release command: %s", err)
				}
				ue.GmmLog.Info("sent pdu session resource release command to UE")
			default:
				return fmt.Errorf("error N2 SM information type[%s]", responseData.N2SmInfoType)
			}
		} else if n1Msg != nil {
			ue.GmmLog.Debug("AMF forward Only N1 SM Message to UE")
			err := ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe, n1Msg, nil)
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
func HandleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, registrationRequest *nasMessage.RegistrationRequest) error {
	ctx, span := tracer.Start(ctx, "AMF HandleRegistrationRequest")
	defer span.End()

	var guamiFromUeGuti models.Guami

	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	if ue.RanUe == nil {
		return fmt.Errorf("RanUe is nil")
	}

	// MacFailed is set if plain Registration Request message received with GUTI/SUCI or
	// integrity protected Registration Reguest message received but mac verification Failed
	if ue.MacFailed {
		ue.SecurityContextAvailable = false
	}

	ue.SetOnGoing(&context.OnGoingProcedureWithPrio{
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

	ue.RegistrationRequest = registrationRequest
	ue.RegistrationType5GS = registrationRequest.NgksiAndRegistrationType5GS.GetRegistrationType5GS()
	regName := getRegistrationType5GSName(ue.RegistrationType5GS)
	ue.GmmLog.Debug("Received Registration Request", zap.String("registrationType", regName))

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSReserved {
		ue.RegistrationType5GS = nasMessage.RegistrationType5GSInitialRegistration
	}

	mobileIdentity5GSContents := registrationRequest.MobileIdentity5GS.GetMobileIdentity5GSContents()
	if len(mobileIdentity5GSContents) == 0 {
		return errors.New("mobile identity 5GS is empty")
	}

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	ue.IdentityTypeUsedForRegistration = nasConvert.GetTypeOfIdentity(mobileIdentity5GSContents[0])
	switch ue.IdentityTypeUsedForRegistration { // get type of identity
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		ue.GmmLog.Debug("No Identity used for registration")
	case nasMessage.MobileIdentity5GSTypeSuci:
		ue.GmmLog.Debug("UE used SUCI identity for registration")
		var plmnID string
		ue.Suci, plmnID = nasConvert.SuciToString(mobileIdentity5GSContents)
		ue.PlmnID = PlmnIDStringToModels(plmnID)
	case nasMessage.MobileIdentity5GSType5gGuti:
		guamiFromUeGutiTmp, guti := util.GutiToString(mobileIdentity5GSContents)
		guamiFromUeGuti = guamiFromUeGutiTmp
		ue.Guti = guti
		ue.GmmLog.Debug("UE used GUTI identity for registration", zap.String("guti", guti))

		if reflect.DeepEqual(guamiFromUeGuti, operatorInfo.Guami) {
			ue.ServingAmfChanged = false
		} else {
			ue.GmmLog.Debug("Serving AMF has changed but 5G-Core is not supporting for now")
			ue.ServingAmfChanged = false
		}
	case nasMessage.MobileIdentity5GSTypeImei:
		imei := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imei
		ue.GmmLog.Debug("UE used IMEI identity for registration", zap.String("imei", imei))
	case nasMessage.MobileIdentity5GSTypeImeisv:
		imeisv := nasConvert.PeiToString(mobileIdentity5GSContents)
		ue.Pei = imeisv
		ue.GmmLog.Debug("UE used IMEISV identity for registration", zap.String("imeisv", imeisv))
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
	ue.Location = ue.RanUe.Location
	ue.Tai = ue.RanUe.Tai

	// Check TAI
	taiList := make([]models.Tai, len(operatorInfo.Tais))
	copy(taiList, operatorInfo.Tais)
	for i := range taiList {
		tac, err := util.TACConfigToModels(taiList[i].Tac)
		if err != nil {
			logger.AmfLog.Warn("failed to convert TAC to models.Tac", zap.Error(err), zap.String("tac", taiList[i].Tac))
			continue
		}
		taiList[i].Tac = tac
	}
	if !context.InTaiList(ue.Tai, taiList) {
		err := gmm_message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMTrackingAreaNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return fmt.Errorf("registration Reject[Tracking area not allowed]")
	}

	if ue.RegistrationType5GS == nasMessage.RegistrationType5GSInitialRegistration && registrationRequest.UESecurityCapability == nil {
		err := gmm_message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return fmt.Errorf("registration request does not contain UE security capability for initial registration")
	}

	if registrationRequest.UESecurityCapability != nil {
		ue.UESecurityCapability = registrationRequest.UESecurityCapability
	}

	if ue.ServingAmfChanged {
		logger.AmfLog.Debug("Serving AMF has changed - Unsupported")
	}

	return nil
}

func IdentityVerification(ue *context.AmfUe) bool {
	return ue.Supi != "" || len(ue.Suci) != 0
}

func HandleInitialRegistration(ctx ctxt.Context, ue *context.AmfUe) error {
	amfSelf := context.AMFSelf()

	ue.ClearRegistrationData()

	// update Kgnb/Kn3iwf
	ue.UpdateSecurityContext()

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if ue.SubscribedNssai == nil {
		ue.SubscribedNssai = &operatorInfo.SupportedPLMN.SNssai
	}

	if err := handleRequestedNssai(ctx, ue, operatorInfo.SupportedPLMN); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	}

	if ue.AllowedNssai == nil {
		err := gmm_message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			ue.GmmLog.Error("error sending registration reject", zap.Error(err))
		}
		err = ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
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

	if ue.ServingAmfChanged || !ue.SubscriptionDataValid {
		if err := getAndSetSubscriberData(ctx, ue); err != nil {
			return err
		}
	}

	if !context.SubscriberExists(ctx, ue.Supi) {
		ue.GmmLog.Error("Subscriber does not exist", zap.Error(err))
		err := gmm_message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}
		ue.GmmLog.Info("sent registration reject to UE")
		return fmt.Errorf("ue not found in database: %s", ue.Supi)
	}

	amfSelf.AllocateRegistrationArea(ctx, ue, operatorInfo.Tais)
	ue.GmmLog.Debug("use original GUTI", zap.String("guti", ue.Guti))

	amfSelf.AddAmfUeToUePool(ue, ue.Supi)
	ue.T3502Value = amfSelf.T3502Value
	ue.T3512Value = amfSelf.T3512Value

	amfSelf.ReAllocateGutiToUe(ctx, ue, operatorInfo.Guami)
	// check in specs if we need to wait for confirmation before freeing old GUTI
	amfSelf.FreeOldGuti(ue)

	err = gmm_message.SendRegistrationAccept(ctx, ue, nil, nil, nil, nil, nil, operatorInfo.SupportedPLMN, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error sending GMM registration accept: %v", err)
	}

	return nil
}

func HandleMobilityAndPeriodicRegistrationUpdating(ctx ctxt.Context, ue *context.AmfUe) error {
	ue.GmmLog.Debug("Handle MobilityAndPeriodicRegistrationUpdating")

	ue.DerivateAnKey()

	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.UpdateType5GS != nil {
		if ue.RegistrationRequest.UpdateType5GS.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.UeRadioCapability = ""
			ue.UeRadioCapabilityForPaging = nil
		}
	}

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	// Registration with AMF re-allocation (TS 23.502 4.2.2.2.3)
	if ue.SubscribedNssai == nil {
		ue.SubscribedNssai = &operatorInfo.SupportedPLMN.SNssai
	}

	if err := handleRequestedNssai(ctx, ue, operatorInfo.SupportedPLMN); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	} else {
		if ue.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
			err := gmm_message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
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
		ue.GmmLog.Debug("The UE did not provide PEI")
		err := gmm_message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeImei)
		if err != nil {
			return fmt.Errorf("error sending identity request: %v", err)
		}
		ue.GmmLog.Info("sent identity request to UE")
		return nil
	}

	if ue.ServingAmfChanged ||
		!ue.SubscriptionDataValid {
		if err := getAndSetSubscriberData(ctx, ue); err != nil {
			return err
		}
	}

	var reactivationResult *[16]bool
	var errPduSessionID, errCause []uint8
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}
	suList := ngapType.PDUSessionResourceSetupListSUReq{}

	if ue.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)
		allowReEstablishPduSession := true

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
					if hasUplinkData {
						response, err := consumer.SendUpdateSmContextActivateUpCnxState(ctx, ue, smContext)
						if response == nil {
							reactivationResult[pduSessionID] = true
							errPduSessionID = append(errPduSessionID, uint8(pduSessionID))
							cause := nasMessage.Cause5GMMProtocolErrorUnspecified
							errCause = append(errCause, cause)

							if err != nil {
								ue.GmmLog.Error("Update SmContext Error", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
							}
						} else {
							if ue.RanUe.UeContextRequest {
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
		for psi := 1; psi <= 15; psi++ {
			pduSessionID := int32(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
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

	amfSelf.ReAllocateGutiToUe(ctx, ue, operatorInfo.Guami)
	// check in specs if we need to wait for confirmation before freeing old GUTI
	amfSelf.FreeOldGuti(ue)

	if ue.RegistrationRequest.AllowedPDUSessionStatus != nil {
		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message.Request.JSONData
			n1Msg := ue.N1N2Message.Request.BinaryDataN1Message
			n2Info := ue.N1N2Message.Request.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := gmm_message.BuildRegistrationAccept(ctx, ue, pduSessionStatus,
						reactivationResult, errPduSessionID, errCause, operatorInfo.SupportedPLMN)
					if err != nil {
						return err
					}
					err = ngap_message.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe, nasPdu, suList)
					if err != nil {
						return fmt.Errorf("error sending pdu session resource setup request: %v", err)
					}
					ue.GmmLog.Info("Sent NGAP pdu session resource setup request")
				} else {
					err := gmm_message.SendRegistrationAccept(ctx, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, operatorInfo.SupportedPLMN, operatorInfo.Guami)
					if err != nil {
						return fmt.Errorf("error sending GMM registration accept: %v", err)
					}
					ue.GmmLog.Info("Sent GMM registration accept")
				}
				switch requestData.N1MessageClass {
				case models.N1MessageClassSM:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassLPP:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassSMS:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassUPDP:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				}
				ue.N1N2Message = nil
				return nil
			}

			smInfo := requestData.N2InfoContainer.SmInfo
			_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				ue.N1N2Message = nil
				return fmt.Errorf("pdu Session Id does not Exists")
			}

			if smInfo.NgapIeType == models.NgapIeTypePduResSetupReq {
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

	amfSelf.AllocateRegistrationArea(ctx, ue, operatorInfo.Tais)

	if ue.RanUe.UeContextRequest {
		err := gmm_message.SendRegistrationAccept(ctx, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, operatorInfo.SupportedPLMN, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error sending GMM registration accept: %v", err)
		}
		ue.GmmLog.Info("Sent GMM registration accept")
		return nil
	} else {
		nasPdu, err := gmm_message.BuildRegistrationAccept(ctx, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, operatorInfo.SupportedPLMN)
		if err != nil {
			return fmt.Errorf("error building registration accept: %v", err)
		}
		if len(suList.List) != 0 {
			err := ngap_message.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe, nasPdu, suList)
			if err != nil {
				return fmt.Errorf("error sending pdu session resource setup request: %v", err)
			}
			ue.GmmLog.Info("Sent NGAP pdu session resource setup request")
		} else {
			err := ngap_message.SendDownlinkNasTransport(ctx, ue.RanUe, nasPdu, nil)
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

func getAndSetSubscriberData(ctx ctxt.Context, ue *context.AmfUe) error {
	bitRate, dnn, err := context.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate
	ue.SubscriptionDataValid = true

	return nil
}

// TS 23.502 4.2.2.2.3 Registration with AMF Re-allocation
func handleRequestedNssai(ctx ctxt.Context, ue *context.AmfUe, supportedPLMN *context.PlmnSupportItem) error {
	amfSelf := context.AMFSelf()

	if ue.RegistrationRequest.RequestedNSSAI != nil {
		requestedNssai, err := util.RequestedNssaiToModels(ue.RegistrationRequest.RequestedNSSAI)
		if err != nil {
			return fmt.Errorf("failed to decode requested NSSAI[%s]", err)
		}

		needSliceSelection := false
		var newAllowed *models.Snssai

		for _, requestedSnssai := range requestedNssai {
			if ue.InSubscribedNssai(requestedSnssai) {
				newAllowed = &models.Snssai{
					Sst: requestedSnssai.Sst,
					Sd:  requestedSnssai.Sd,
				}
			} else {
				needSliceSelection = true
				break
			}
		}

		ue.AllowedNssai = newAllowed

		if needSliceSelection {
			// Step 4
			ue.AllowedNssai = ue.SubscribedNssai

			var n1Message bytes.Buffer
			err = ue.RegistrationRequest.EncodeRegistrationRequest(&n1Message)
			if err != nil {
				return fmt.Errorf("failed to encode registration request: %s", err)
			}
			return nil
		}
	}

	// if registration request has no requested nssai, or non of snssai in requested nssai is permitted by nssf
	// then use ue subscribed snssai which is marked as default as allowed nssai
	if ue.AllowedNssai == nil {
		var newAllowed *models.Snssai
		if amfSelf.InPlmnSupport(ctx, *ue.SubscribedNssai, supportedPLMN) {
			newAllowed = ue.SubscribedNssai
		}
		ue.AllowedNssai = newAllowed
	}
	return nil
}

func HandleIdentityResponse(ue *context.AmfUe, identityResponse *nasMessage.IdentityResponse) error {
	if ue == nil {
		return fmt.Errorf("AmfUe is nil")
	}

	mobileIdentityContents := identityResponse.MobileIdentity.GetMobileIdentityContents()
	if len(mobileIdentityContents) == 0 {
		return fmt.Errorf("mobile identity is empty")
	}

	switch nasConvert.GetTypeOfIdentity(mobileIdentityContents[0]) {
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
	if ue.T3555 != nil {
		ue.T3555.Stop()
		ue.T3555 = nil // clear the timer
	}
	amfSelf := context.AMFSelf()
	amfSelf.FreeOldGuti(ue)

	return nil
}

func AuthenticationProcedure(ctx ctxt.Context, ue *context.AmfUe) (bool, error) {
	ctx, span := tracer.Start(ctx, "AuthenticationProcedure")
	defer span.End()

	// Check whether UE has SUCI and SUPI
	if IdentityVerification(ue) {
		ue.GmmLog.Debug("UE has SUCI / SUPI")
		if ue.SecurityContextIsValid() {
			ue.GmmLog.Debug("UE has a valid security context - skip the authentication procedure")
			return true, nil
		} else {
			ue.GmmLog.Debug("UE has no valid security context - continue with the authentication procedure")
		}
	} else {
		// Request UE's SUCI by sending identity request
		ue.GmmLog.Debug("UE has no SUCI / SUPI - send identity request to UE")
		err := gmm_message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
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

	err = gmm_message.SendAuthenticationRequest(ctx, ue.RanUe)
	if err != nil {
		return false, fmt.Errorf("error sending authentication request: %v", err)
	}

	return false, nil
}

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

// TS 24501 5.6.1
func HandleServiceRequest(ctx ctxt.Context, ue *context.AmfUe, serviceRequest *nasMessage.ServiceRequest) error {
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
		ue.GmmLog.Warn("UE should not in OnGoing", zap.Any("procedure", procedure))
	}

	// Send Authtication / Security Procedure not support
	// Rejecting ServiceRequest if it is received in Deregistered State
	if !ue.SecurityContextIsValid() || ue.State.Current() == context.Deregistered {
		ue.GmmLog.Warn("No security context", zap.String("supi", ue.Supi))
		err := gmm_message.SendServiceReject(ctx, ue.RanUe, nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.GmmLog.Info("sent service reject")
		err = ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
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
				return fmt.Errorf("expected service request message, got %d", messageType)
			}
			// TS 24.501 4.4.6: The AMF shall consider the NAS message that is obtained from the NAS message container
			// IE as the initial NAS message that triggered the procedure
			serviceRequest = m.ServiceRequest
		}
		// TS 33.501 6.4.6 step 3: if the initial NAS message was protected but did not pass the integrity check
		ue.RetransmissionOfInitialNASMsg = ue.MacFailed
	}

	serviceType := serviceRequest.GetServiceTypeValue()

	logger.AmfLog.Debug("Handle Service Request", zap.String("supi", ue.Supi), zap.String("serviceType", serviceTypeToString(serviceType)))

	var reactivationResult, acceptPduSessionPsi *[16]bool
	var errPduSessionID, errCause []uint8
	var targetPduSessionID int32
	suList := ngapType.PDUSessionResourceSetupListSUReq{}
	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}

	if serviceType == nasMessage.ServiceTypeEmergencyServices ||
		serviceType == nasMessage.ServiceTypeEmergencyServicesFallback {
		ue.GmmLog.Warn("emergency service is not supported")
	}

	if ue.MacFailed {
		ue.SecurityContextAvailable = false
		ue.GmmLog.Warn("Security Context Exist, But Integrity Check Failed with existing Context", zap.String("supi", ue.Supi))
		err := gmm_message.SendServiceReject(ctx, ue.RanUe, nil, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending service reject: %v", err)
		}
		ue.GmmLog.Info("sent service reject")
		err = ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
		return nil
	}

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	if serviceType == nasMessage.ServiceTypeSignalling {
		err := sendServiceAccept(ctx, ue, ctxList, suList, nil, nil, nil, nil, operatorInfo.Guami)
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
		uplinkDataPsi := nasConvert.PSIToBooleanArray(serviceRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)
		ue.SmContextList.Range(func(key, value any) bool {
			pduSessionID := key.(int32)
			smContext := value.(*context.SmContext)

			if pduSessionID != targetPduSessionID {
				if uplinkDataPsi[pduSessionID] {
					response, err := consumer.SendUpdateSmContextActivateUpCnxState(ctx, ue, smContext)
					if err != nil {
						ue.GmmLog.Error("SendUpdateSmContextActivateUpCnxState Error", zap.Error(err), zap.Int32("pduSessionID", pduSessionID))
					} else if response == nil {
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, uint8(pduSessionID))
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else if ue.RanUe.UeContextRequest {
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
			if !psiArray[pduSessionID] {
				err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
				if err != nil {
					ue.GmmLog.Error("Release SmContext Error", zap.Error(err))
				}
			} else {
				acceptPduSessionPsi[pduSessionID] = true
			}
			return true
		})
	}
	switch serviceType {
	case nasMessage.ServiceTypeMobileTerminatedServices: // Triggered by Network
		// TS 24.501 5.4.4.1 - We need to assign a new GUTI after a successful Service Request
		// triggered by a paging request.
		ue.ConfigurationUpdateCommandFlags = &context.ConfigurationUpdateCommandFlags{NeedGUTI: true}

		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message.Request.JSONData
			n1Msg := ue.N1N2Message.Request.BinaryDataN1Message
			n2Info := ue.N1N2Message.Request.BinaryDataN2Information

			// Paging was triggered for downlink signaling only
			if n2Info == nil && n1Msg != nil {
				err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
				if err != nil {
					return err
				}
				switch requestData.N1MessageClass {
				case models.N1MessageClassSM:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				case models.N1MessageClassLPP:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				case models.N1MessageClassSMS:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				case models.N1MessageClassUPDP:
					err := gmm_message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
					ue.GmmLog.Info("sent downlink nas transport message")
				}
				ue.N1N2Message = nil
			} else {
				smInfo := requestData.N2InfoContainer.SmInfo

				_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
				if !exist {
					ue.N1N2Message = nil
					return fmt.Errorf("service Request triggered by Network for pduSessionID that does not exist")
				}

				if smInfo.NgapIeType == models.NgapIeTypePduResSetupReq {
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
					if ue.RanUe.UeContextRequest {
						ngap_message.AppendPDUSessionResourceSetupListCxtReq(&ctxList, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
					} else {
						ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, smInfo.PduSessionID, omecSnssai, nasPdu, n2Info)
					}
				}
				ue.GmmLog.Debug("sending service accept")
				err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
				if err != nil {
					return err
				}
			}
		} else {
			err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
			if err != nil {
				return err
			}
		}
		if ue.ConfigurationUpdateCommandFlags != nil {
			// Allocate a new GUTI after successful network triggered Service Request
			amfSelf := context.AMFSelf()
			amfSelf.ReAllocateGutiToUe(ctx, ue, operatorInfo.Guami)

			gmm_message.SendConfigurationUpdateCommand(ctx, ue)
			ue.ConfigurationUpdateCommandFlags = nil
		}
	case nasMessage.ServiceTypeData:
		err := sendServiceAccept(ctx, ue, ctxList, suList, acceptPduSessionPsi, reactivationResult, errPduSessionID, errCause, operatorInfo.Guami)
		if err != nil {
			return err
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

func sendServiceAccept(ctx ctxt.Context, ue *context.AmfUe, ctxList ngapType.PDUSessionResourceSetupListCxtReq,
	suList ngapType.PDUSessionResourceSetupListSUReq, pDUSessionStatus *[16]bool,
	reactivationResult *[16]bool, errPduSessionID, errCause []uint8, supportedGUAMI *models.Guami,
) error {
	if ue.RanUe.UeContextRequest {
		// update Kgnb/Kn3iwf
		ue.UpdateSecurityContext()

		nasPdu, err := gmm_message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionID, errCause)
		if err != nil {
			return err
		}
		if len(ctxList.List) != 0 {
			err := ngap_message.SendInitialContextSetupRequest(ctx, ue, nasPdu, &ctxList, nil, nil, nil, supportedGUAMI)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Info("sent service accept with context list", zap.Int("len", len(ctxList.List)))
		} else {
			err := ngap_message.SendInitialContextSetupRequest(ctx, ue, nasPdu, nil, nil, nil, nil, supportedGUAMI)
			if err != nil {
				return fmt.Errorf("error sending initial context setup request: %v", err)
			}
			ue.GmmLog.Info("sent service accept")
		}
	} else if len(suList.List) != 0 {
		nasPdu, err := gmm_message.BuildServiceAccept(ue, pDUSessionStatus, reactivationResult,
			errPduSessionID, errCause)
		if err != nil {
			return err
		}
		err = ngap_message.SendPDUSessionResourceSetupRequest(ctx, ue.RanUe, nasPdu, suList)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource setup request: %v", err)
		}
		ue.GmmLog.Info("sent service accept")
	} else {
		err := gmm_message.SendServiceAccept(ctx, ue.RanUe, pDUSessionStatus, reactivationResult, errPduSessionID, errCause)
		if err != nil {
			return fmt.Errorf("error sending service accept: %v", err)
		}
		ue.GmmLog.Info("sent service accept")
	}
	return nil
}

// TS 24.501 5.4.1
func HandleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, authenticationResponse *nasMessage.AuthenticationResponse) error {
	logger.AmfLog.Debug("Handle Authentication Response", zap.String("supi", ue.Supi))

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.AuthenticationCtx == nil {
		return fmt.Errorf("ue Authentication Context is nil")
	}

	resStar := authenticationResponse.AuthenticationResponseParameter.GetRES()

	// Calculate HRES* (TS 33.501 Annex A.5)
	p0, err := hex.DecodeString(ue.AuthenticationCtx.Var5gAuthData.Rand)
	if err != nil {
		return err
	}

	p1 := resStar[:]
	concat := append(p0, p1...)
	hResStarBytes := sha256.Sum256(concat)
	hResStar := hex.EncodeToString(hResStarBytes[16:])

	if hResStar != ue.AuthenticationCtx.Var5gAuthData.HxresStar {
		ue.GmmLog.Error("HRES* Validation Failure", zap.String("received", hResStar), zap.String("expected", ue.AuthenticationCtx.Var5gAuthData.HxresStar))

		if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
			err := gmm_message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.GmmLog.Info("sent identity request")
			return nil
		} else {
			err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return GmmFSM.SendEvent(ctx, ue.State, AuthFailEvent, fsm.ArgsType{
				ArgAmfUe: ue,
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
		return GmmFSM.SendEvent(ctx, ue.State, AuthSuccessEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
	case models.AuthResultFailure:
		if ue.IdentityTypeUsedForRegistration == nasMessage.MobileIdentity5GSType5gGuti {
			err := gmm_message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeSuci)
			if err != nil {
				return fmt.Errorf("send identity request error: %s", err)
			}
			ue.GmmLog.Info("sent identity request")
			return nil
		} else {
			err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return GmmFSM.SendEvent(ctx, ue.State, AuthFailEvent, fsm.ArgsType{
				ArgAmfUe: ue,
			})
		}
	}

	return nil
}

func HandleAuthenticationFailure(ctx ctxt.Context, ue *context.AmfUe, authenticationFailure *nasMessage.AuthenticationFailure) error {
	logger.AmfLog.Debug("Handle Authentication Failure", zap.String("supi", ue.Supi))

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause5GMM := authenticationFailure.Cause5GMM.GetCauseValue()

	switch cause5GMM {
	case nasMessage.Cause5GMMMACFailure:
		ue.GmmLog.Warn("Authentication Failure Cause: Mac Failure")
		err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return GmmFSM.SendEvent(ctx, ue.State, AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue})
	case nasMessage.Cause5GMMNon5GAuthenticationUnacceptable:
		ue.GmmLog.Warn("Authentication Failure Cause: Non-5G Authentication Unacceptable")
		err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending GMM authentication reject: %v", err)
		}

		return GmmFSM.SendEvent(ctx, ue.State, AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue})
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

		err := gmm_message.SendAuthenticationRequest(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("send authentication request error: %s", err)
		}

		ue.GmmLog.Info("Sent authentication request")
	case nasMessage.Cause5GMMSynchFailure: // TS 24.501 5.4.1.3.7 case f
		ue.GmmLog.Warn("Authentication Failure 5GMM Cause: Synch Failure")

		ue.AuthFailureCauseSynchFailureTimes++
		if ue.AuthFailureCauseSynchFailureTimes >= 2 {
			ue.GmmLog.Warn("2 consecutive Synch Failure, terminate authentication procedure")
			err := gmm_message.SendAuthenticationReject(ctx, ue.RanUe)
			if err != nil {
				return fmt.Errorf("error sending GMM authentication reject: %v", err)
			}

			return GmmFSM.SendEvent(ctx, ue.State, AuthFailEvent, fsm.ArgsType{ArgAmfUe: ue})
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

		err = gmm_message.SendAuthenticationRequest(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("send authentication request error: %s", err)
		}

		ue.GmmLog.Info("Sent authentication request")
	}

	return nil
}

func HandleRegistrationComplete(ctx ctxt.Context, ue *context.AmfUe, registrationComplete *nasMessage.RegistrationComplete) error {
	logger.AmfLog.Debug("Handle Registration Complete", zap.String("supi", ue.Supi))

	if ue.T3550 != nil {
		ue.T3550.Stop()
		ue.T3550 = nil // clear the timer
	}

	forPending := ue.RegistrationRequest.GetFOR() == nasMessage.FollowOnRequestPending

	uds := ue.RegistrationRequest.UplinkDataStatus

	udsHasPending := uds != nil

	hasActiveSessions := ue.HasActivePduSessions()

	shouldRelease := !(forPending || udsHasPending || hasActiveSessions)

	if shouldRelease {
		err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextN2NormalRelease, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	return GmmFSM.SendEvent(ctx, ue.State, ContextSetupSuccessEvent, fsm.ArgsType{
		ArgAmfUe: ue,
	})
}

// TS 33.501 6.7.2
func HandleSecurityModeComplete(ctx ctxt.Context, ue *context.AmfUe, securityModeComplete *nasMessage.SecurityModeComplete) error {
	logger.AmfLog.Debug("Handle Security Mode Complete", zap.String("supi", ue.Supi))

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	if ue.SecurityContextIsValid() {
		// update Kgnb/Kn3iwf
		ue.UpdateSecurityContext()
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
			return GmmFSM.SendEvent(ctx, ue.State, SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      ue,
				ArgNASMessage: m.GmmMessage.RegistrationRequest,
			})
		}
	}
	return GmmFSM.SendEvent(ctx, ue.State, SecurityModeSuccessEvent, fsm.ArgsType{
		ArgAmfUe:      ue,
		ArgNASMessage: ue.RegistrationRequest,
	})
}

func HandleSecurityModeReject(ctx ctxt.Context, ue *context.AmfUe, securityModeReject *nasMessage.SecurityModeReject) error {
	logger.AmfLog.Debug("Handle Security Mode Reject", zap.String("supi", ue.Supi))

	if ue.T3560 != nil {
		ue.T3560.Stop()
		ue.T3560 = nil // clear the timer
	}

	cause := securityModeReject.Cause5GMM.GetCauseValue()
	ue.GmmLog.Warn("Reject", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
	ue.GmmLog.Error("UE reject the security mode command, abort the ongoing procedure")

	ue.SecurityContextAvailable = false

	err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		return fmt.Errorf("error sending ue context release command: %v", err)
	}
	return nil
}

// TS 23.502 4.2.2.3
func HandleDeregistrationRequest(ctx ctxt.Context, ue *context.AmfUe, deregistrationRequest *nasMessage.DeregistrationRequestUEOriginatingDeregistration) error {
	logger.AmfLog.Debug("Handle Deregistration Request", zap.String("supi", ue.Supi))

	targetDeregistrationAccessType := deregistrationRequest.GetAccessType()
	ue.SmContextList.Range(func(key, value any) bool {
		smContext := value.(*context.SmContext)

		err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
		if err != nil {
			ue.GmmLog.Error("Release SmContext Error", zap.Error(err))
		}

		return true
	})

	// if Deregistration type is not switch-off, send Deregistration Accept
	if deregistrationRequest.GetSwitchOff() == 0 && ue.RanUe != nil {
		err := gmm_message.SendDeregistrationAccept(ctx, ue.RanUe)
		if err != nil {
			return fmt.Errorf("error sending deregistration accept: %v", err)
		}
		ue.GmmLog.Info("sent deregistration accept")
	}

	// TS 23.502 4.2.6, 4.12.3
	if targetDeregistrationAccessType != nasMessage.AccessType3GPP {
		return fmt.Errorf("only 3gpp access type is supported")
	}

	if ue.RanUe != nil {
		err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextReleaseUeContext, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	return GmmFSM.SendEvent(ctx, ue.State, DeregistrationAcceptEvent, fsm.ArgsType{
		ArgAmfUe: ue,
	})
}

// TS 23.502 4.2.2.3
func HandleDeregistrationAccept(ctx ctxt.Context, ue *context.AmfUe, deregistrationAccept *nasMessage.DeregistrationAcceptUETerminatedDeregistration) error {
	logger.AmfLog.Debug("Handle Deregistration Accept", zap.String("supi", ue.Supi))

	if ue.T3522 != nil {
		ue.T3522.Stop()
		ue.T3522 = nil // clear the timer
	}

	if ue.RanUe != nil {
		err := ngap_message.SendUEContextReleaseCommand(ctx, ue.RanUe, context.UeContextReleaseDueToNwInitiatedDeregistraion, ngapType.CausePresentNas, ngapType.CauseNasPresentDeregister)
		if err != nil {
			return fmt.Errorf("error sending ue context release command: %v", err)
		}
	}

	return GmmFSM.SendEvent(ctx, ue.State, DeregistrationAcceptEvent, fsm.ArgsType{
		ArgAmfUe: ue,
	})
}

func HandleStatus5GMM(ue *context.AmfUe, status5GMM *nasMessage.Status5GMM) error {
	logger.AmfLog.Debug("Handle 5GMM Status", zap.String("supi", ue.Supi))

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	cause := status5GMM.Cause5GMM.GetCauseValue()
	ue.GmmLog.Error("Error condition", zap.String("Cause", nasMessage.Cause5GMMToString(cause)))
	return nil
}

func HandleAuthenticationError(ctx ctxt.Context, ue *context.AmfUe) error {
	logger.AmfLog.Debug("Handle Authentication Error", zap.String("supi", ue.Supi))

	if ue.RegistrationRequest != nil {
		err := gmm_message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		ue.GmmLog.Info("sent registration reject")
	}
	return nil
}
