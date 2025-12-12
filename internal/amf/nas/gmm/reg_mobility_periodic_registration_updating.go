package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

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
		ue.SubscribedNssai = operatorInfo.SupportedPLMN.SNssai
	}

	if err := handleRequestedNssai(ctx, ue, operatorInfo.SupportedPLMN); err != nil {
		return err
	}

	if ue.RegistrationRequest.Capability5GMM != nil {
		ue.Capability5GMM = *ue.RegistrationRequest.Capability5GMM
	} else {
		if ue.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
			err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
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
		err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeImei)
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
			requestData := ue.N1N2Message.JSONData
			n1Msg := ue.N1N2Message.BinaryDataN1Message
			n2Info := ue.N1N2Message.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := message.BuildRegistrationAccept(ctx, ue, pduSessionStatus,
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
					err := message.SendRegistrationAccept(ctx, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, operatorInfo.SupportedPLMN, operatorInfo.Guami)
					if err != nil {
						return fmt.Errorf("error sending GMM registration accept: %v", err)
					}
					ue.GmmLog.Info("Sent GMM registration accept")
				}
				switch requestData.N1MessageClass {
				case models.N1MessageClassSM:
					err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassLPP:
					err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeLPP, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassSMS:
					err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeSMS, n1Msg, 0, 0)
					if err != nil {
						return fmt.Errorf("error sending downlink nas transport message: %v", err)
					}
				case models.N1MessageClassUPDP:
					err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeUEPolicy, n1Msg, 0, 0)
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
					nasPdu, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, pduSessionID, nil)
					if err != nil {
						return err
					}
				}
				ngap_message.AppendPDUSessionResourceSetupListSUReq(&suList, smInfo.PduSessionID, smInfo.SNssai, nasPdu, n2Info)
			}
		}
	}

	amfSelf.AllocateRegistrationArea(ctx, ue, operatorInfo.Tais)

	if ue.RanUe.UeContextRequest {
		err := message.SendRegistrationAccept(ctx, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, operatorInfo.SupportedPLMN, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error sending GMM registration accept: %v", err)
		}
		ue.GmmLog.Info("Sent GMM registration accept")
		return nil
	} else {
		nasPdu, err := message.BuildRegistrationAccept(ctx, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, operatorInfo.SupportedPLMN)
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
