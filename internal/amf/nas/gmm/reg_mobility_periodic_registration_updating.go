package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleMobilityAndPeriodicRegistrationUpdating(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe) error {
	ue.Log.Debug("Handle MobilityAndPeriodicRegistrationUpdating")

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	conn := ue.NasConn()
	if conn == nil {
		return fmt.Errorf("no active NAS connection")
	}

	err := ue.DerivateAnKey()
	if err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	if conn.RegistrationRequest.UpdateType5GS != nil {
		if conn.RegistrationRequest.GetNGRanRcu() == nasMessage.NGRanRadioCapabilityUpdateNeeded {
			ue.Current().UeRadioCapability = ""
			ue.Current().UeRadioCapabilityForPaging = nil
		}
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		return fmt.Errorf("error getting operator info: %v", err)
	}

	subscriberProfile, err := amfInstance.GetSubscriberProfile(ctx, ue.Supi)
	if err != nil {
		return fmt.Errorf("error getting subscriber profile: %v", err)
	}

	if len(subscriberProfile.AllowedNssai) == 0 {
		UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationReject).Inc()

		err = message.SendRegistrationReject(ctx, ranUe, nasMessage.Cause5GMM5GSServicesNotAllowed)
		if err != nil {
			return fmt.Errorf("error sending registration reject: %v", err)
		}

		return fmt.Errorf("registration Reject [No allowed S-NSSAI in subscription]")
	}

	ue.Current().AllowedNssai = subscriberProfile.AllowedNssai

	// The 5GMM capability IE is optional (TS 24.501 Table 8.2.6.1.1) and is
	// re-sent only when it changes (§5.5.1.3.2). Its absence is not an error:
	// per §7.7.1 the receiver treats a missing optional IE as not present and
	// proceeds.

	if conn.RegistrationRequest.MICOIndication != nil {
		ue.Log.Warn("Receive MICO Indication Not Supported", zap.Uint8("RAAI", conn.RegistrationRequest.GetRAAI()))
	}

	if conn.RegistrationRequest.RequestedDRXParameters != nil {
		drx := conn.RegistrationRequest.GetDRXValue()
		if drx > nasMessage.DRXcycleParameterT256 {
			ue.Log.Warn("UE requested reserved DRX value, treating as not specified", zap.Uint8("drxValue", drx))
			drx = nasMessage.DRXValueNotSpecified
		}

		ue.Current().UESpecificDRX = drx
	}

	if len(ue.Pei) == 0 {
		ue.Log.Debug("The UE did not provide PEI")

		err := message.SendIdentityRequest(ctx, ranUe, nasMessage.MobileIdentity5GSTypeImei)
		if err != nil {
			return fmt.Errorf("error sending identity request: %v", err)
		}

		ue.Log.Info("sent identity request to UE")

		return nil
	}

	ue.Current().Ambr = subscriberProfile.Ambr

	var (
		reactivationResult        *[16]bool
		errPduSessionID, errCause []uint8
	)

	ctxList := ngapType.PDUSessionResourceSetupListCxtReq{}
	suList := ngapType.PDUSessionResourceSetupListSUReq{}

	if conn.RegistrationRequest.UplinkDataStatus != nil {
		uplinkDataPsi := nasConvert.PSIToBooleanArray(conn.RegistrationRequest.UplinkDataStatus.Buffer)
		reactivationResult = new([16]bool)

		for idx, hasUplinkData := range uplinkDataPsi {
			pduSessionID := uint8(idx)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				// uplink data are pending for the corresponding PDU session identity
				if hasUplinkData {
					binaryDataN2SmInformation, err := amfInstance.Smf.ActivateSmContext(ctx, smContext.Ref)
					if err != nil {
						ue.Log.Warn("SendActivateSmContextRequest Error", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
						reactivationResult[pduSessionID] = true
						errPduSessionID = append(errPduSessionID, pduSessionID)
						cause := nasMessage.Cause5GMMProtocolErrorUnspecified
						errCause = append(errCause, cause)
					} else {
						if ranUe.UeContextRequest {
							send.AppendPDUSessionResourceSetupListCxtReq(&ctxList, pduSessionID,
								smContext.Snssai, nil, binaryDataN2SmInformation)
						} else {
							send.AppendPDUSessionResourceSetupListSUReq(&suList, pduSessionID,
								smContext.Snssai, nil, binaryDataN2SmInformation)
						}
					}
				}
			}
		}
	}

	var pduSessionStatus *[16]bool
	if conn.RegistrationRequest.PDUSessionStatus != nil {
		pduSessionStatus = new([16]bool)
		psiArray := nasConvert.PSIToBooleanArray(conn.RegistrationRequest.PDUSessionStatus.Buffer)

		for psi := 1; psi <= 15; psi++ {
			pduSessionID := uint8(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
					err := amfInstance.Smf.ReleaseSmContext(ctx, smContext.Ref)
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

	err = amfInstance.ReAllocateGuti(ctx, ue, operatorInfo.Guami)
	if err != nil {
		return fmt.Errorf("error reallocating GUTI to UE: %v", err)
	}

	if conn.RegistrationRequest.AllowedPDUSessionStatus != nil {
		if conn.N1N2Message != nil {
			requestData := conn.N1N2Message
			n1Msg := conn.N1N2Message.BinaryDataN1Message
			n2Info := conn.N1N2Message.BinaryDataN2Information

			// downlink signalling
			if n2Info == nil {
				if len(suList.List) != 0 {
					nasPdu, err := message.BuildRegistrationAccept(amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
					if err != nil {
						return err
					}

					UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationAccept).Inc()

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

					ue.Log.Info("Sent NGAP pdu session resource setup request")
				} else {
					UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationAccept).Inc()

					err := message.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)
					if err != nil {
						return fmt.Errorf("error sending GMM registration accept: %v", err)
					}

					ue.Log.Info("Sent GMM registration accept")
				}

				err := message.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport message: %v", err)
				}

				conn.N1N2Message = nil

				return nil
			}

			_, exist := ue.SmContextFindByPDUSessionID(requestData.PduSessionID)
			if !exist {
				conn.N1N2Message = nil
				return fmt.Errorf("pdu Session Id does not Exists")
			}

			var (
				nasPdu []byte
				err    error
			)

			if n1Msg != nil {
				nasPdu, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, n1Msg, requestData.PduSessionID, nil)
				if err != nil {
					return err
				}
			}

			send.AppendPDUSessionResourceSetupListSUReq(&suList, requestData.PduSessionID, requestData.SNssai, nasPdu, n2Info)
		}
	}

	ue.AllocateRegistrationArea(operatorInfo.Tais)

	if ranUe.UeContextRequest {
		UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationAccept).Inc()

		err := message.SendRegistrationAccept(ctx, amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, &ctxList, *operatorInfo.Guami.PlmnID, operatorInfo.Guami)
		if err != nil {
			return fmt.Errorf("error sending GMM registration accept: %v", err)
		}

		ue.Log.Info("Sent GMM registration accept")

		return nil
	} else {
		nasPdu, err := message.BuildRegistrationAccept(amfInstance, ue, pduSessionStatus, reactivationResult, errPduSessionID, errCause, *operatorInfo.Guami.PlmnID)
		if err != nil {
			return fmt.Errorf("error building registration accept: %v", err)
		}

		if len(suList.List) != 0 {
			UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationAccept).Inc()

			err := ranUe.SendPDUSessionResourceSetupRequest(
				ctx,
				ue.Current().Ambr.Uplink,
				ue.Current().Ambr.Downlink,
				nasPdu,
				suList,
			)
			if err != nil {
				return fmt.Errorf("error sending pdu session resource setup request: %v", err)
			}

			ue.Log.Info("Sent NGAP pdu session resource setup request")
		} else {
			UERegistrationAttempts.WithLabelValues(getRegistrationType5GSName(conn.RegistrationType5GS), RegistrationAccept).Inc()

			err := ranUe.SendDownlinkNasTransport(ctx, nasPdu, nil)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %v", err)
			}

			ue.Log.Info("sent downlink nas transport message")
		}

		return nil
	}
}
