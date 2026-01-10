package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleMobilityAndPeriodicRegistrationUpdating(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe) error {
	ue.Log.Debug("Handle MobilityAndPeriodicRegistrationUpdating")

	err := ue.DerivateAnKey()
	if err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	if ue.RegistrationRequest.UpdateType5GS != nil {
		if ue.RegistrationRequest.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.UeRadioCapability = ""
			ue.UeRadioCapabilityForPaging = nil
		}
	}

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	ue.AllowedNssai = operatorInfo.SupportedPLMN.SNssai

	if ue.RegistrationRequest.Capability5GMM == nil {
		if ue.RegistrationType5GS != nasMessage.RegistrationType5GSPeriodicRegistrationUpdating {
			err := message.SendRegistrationReject(ctx, ue.RanUe, nasMessage.Cause5GMMProtocolErrorUnspecified)
			if err != nil {
				return fmt.Errorf("error sending registration reject: %v", err)
			}

			return fmt.Errorf("Capability5GMM is nil")
		}
	}

	if ue.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", ue.RegistrationRequest.GetRAAI()))
	}

	if ue.RegistrationRequest.RequestedDRXParameters != nil {
		ue.UESpecificDRX = ue.RegistrationRequest.GetDRXValue()
	}

	if len(ue.Pei) == 0 {
		ue.Log.Debug("The UE did not provide PEI")

		err := message.SendIdentityRequest(ctx, ue.RanUe, nasMessage.MobileIdentity5GSTypeImei)
		if err != nil {
			return fmt.Errorf("error sending identity request: %v", err)
		}

		ue.Log.Info("sent identity request to UE")

		return nil
	}

	bitRate, dnn, err := amf.GetSubscriberData(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("failed to get subscriber data: %v", err)
	}

	ue.Dnn = dnn
	ue.Ambr = bitRate

	var (
		reactivationResult        *[16]bool
		errPduSessionID, errCause []uint8
	)

	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}
	suList := ngapType.PDUSessionResourceSetupListSUReq{}

	if ue.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(ue.RegistrationRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)

		for idx, hasUplinkData := range uplinkDataPsi {
			if !hasUplinkData {
				continue
			}

			pduSessionID := uint8(idx)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				// uplink data are pending for the corresponding PDU session identity
				binaryDataN2SmInformation, err := pdusession.ActivateSmContext(smContext.Ref)
				if err != nil {
					ue.Log.Error("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
					reactivationResult[pduSessionID] = true
					errPduSessionID = append(errPduSessionID, pduSessionID)
					errCause = append(errCause, nasMessage.Cause5GMMProtocolErrorUnspecified)
				} else {
					if ue.RanUe.UeContextRequest {
						send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
					} else {
						send.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionID, smContext.Snssai, nil, binaryDataN2SmInformation)
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
			pduSessionID := uint8(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
					err := pdusession.ReleaseSmContext(ctx, smContext.Ref)
					if err != nil {
						return fmt.Errorf("failed to release sm context: %s", err)
					}

					pduSessionStatus[psi] = false
				} else {
					pduSessionStatus[psi] = true
				}
			}
		}
	}

	err = ue.ReAllocateGuti(operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	// check in specs if we need to wait for confirmation before freeing old GUTI
	ue.FreeOldGuti()

	if ue.RegistrationRequest.AllowedPDUSessionStatus != nil {
		if ue.N1N2Message != nil {
			requestData := ue.N1N2Message
			n1Msg := ue.N1N2Message.BinaryDataN1Message
			n2Info := ue.N1N2Message.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := message.BuildRegistrationAccept(
						ue,
						amf.NetworkFeatureSupport5GS,
						ue.Guti,
						ue.RegistrationArea,
						ue.AllowedNssai,
						ue.T3512Value,
						ue.UESpecificDRX,
						pduSessionStatus,
						reactivationResult,
						errPduSessionID,
						errCause,
						operatorInfo.SupportedPLMN,
					)
					if err != nil {
						return err
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

					ue.Log.Info("Sent NGAP pdu session resource setup request")
				} else {
					err := message.SendRegistrationAccept(ctx, amf, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, operatorInfo.SupportedPLMN, operatorInfo.Guami)
					if err != nil {
						return fmt.Errorf("error sending GMM registration accept: %v", err)
					}

					ue.Log.Info("Sent GMM registration accept")
				}

				err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport message: %v", err)
				}

				ue.N1N2Message = nil

				return nil
			}

			_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				ue.N1N2Message = nil
				return fmt.Errorf("pdu Session Id does not Exists")
			}

			var nasPdu []byte

			if n1Msg != nil {
				nasPdu, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
				if err != nil {
					return fmt.Errorf("build DL NAS Transport error: %v", err)
				}
			}

			send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
		}
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	if ue.RanUe.UeContextRequest {
		err := message.SendRegistrationAccept(ctx, amf, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, operatorInfo.SupportedPLMN, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error sending GMM registration accept: %v", err)
		}

		ue.Log.Info("Sent GMM registration accept")

		return nil
	}

	nasPdu, err := message.BuildRegistrationAccept(
		ue,
		amf.NetworkFeatureSupport5GS,
		ue.Guti,
		ue.RegistrationArea,
		ue.AllowedNssai,
		ue.T3512Value,
		ue.UESpecificDRX,
		pduSessionStatus,
		reactivationResult,
		errPduSessionID,
		errCause,
		operatorInfo.SupportedPLMN,
	)
	if err != nil {
		return fmt.Errorf("error building registration accept: %v", err)
	}

	if len(suList.List) != 0 {
		err := ue.RanUe.Radio.NGAPSender.SendPDUSessionResourceSetupRequest(
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

		ue.Log.Info("Sent NGAP pdu session resource setup request")

		return nil
	}

	err = ue.RanUe.Radio.NGAPSender.SendDownlinkNasTransport(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, nasPdu, nil)
	if err != nil {
		return fmt.Errorf("error sending downlink nas transport: %v", err)
	}

	ue.Log.Info("sent downlink nas transport message")

	return nil
}
