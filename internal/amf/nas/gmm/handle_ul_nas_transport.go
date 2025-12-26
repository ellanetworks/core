package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func forward5GSMMessageToSMF(
	ctx ctxt.Context,
	ue *context.AmfUe,
	pduSessionID uint8,
	smContextRef string,
	smMessage []byte,
) error {
	response, err := pdusession.UpdateSmContextN1Msg(ctx, smContextRef, smMessage)
	if err != nil {
		return fmt.Errorf("couldn't send update sm context request: %s", err)
	}

	if response == nil {
		ue.Log.Warn("SMF did not return any N1/N2 message", zap.Uint8("pduSessionID", pduSessionID))
		return nil
	}

	var n1Msg []byte

	if response.BinaryDataN1SmMessage != nil {
		ue.Log.Debug("Receive N1 SM Message from SMF")
		n1Msg, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, response.BinaryDataN1SmMessage, pduSessionID, nil)
		if err != nil {
			return fmt.Errorf("error building DL NAS Transport: %s", err)
		}
	}

	if response.BinaryDataN2SmInformation != nil {
		ue.Log.Debug("Receive N2 SM Information from SMF")
		if !response.N2SmInfoTypePduResRel {
			ue.Log.Debug("AMF forward N2 SM Information to UE")
			return nil
		}

		list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
		send.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, response.BinaryDataN2SmInformation)
		err := ue.RanUe.Ran.NGAPSender.SendPDUSessionResourceReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, n1Msg, list)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource release command: %s", err)
		}

		ue.Log.Info("sent pdu session resource release command to UE")

		return nil
	}

	if n1Msg != nil {
		err := ue.RanUe.Ran.NGAPSender.SendDownlinkNasTransport(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, n1Msg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink nas transport: %s", err)
		}

		ue.Log.Info("sent downlink nas transport to UE")
	}

	return nil
}

func transport5GSMMessage(ctx ctxt.Context, ue *context.AmfUe, ulNasTransport *nasMessage.ULNASTransport) error {
	smMessage := ulNasTransport.PayloadContainer.GetPayloadContainerContents()

	id := ulNasTransport.PduSessionID2Value
	if id == nil {
		return fmt.Errorf("pdu session id is nil")
	}

	pduSessionID := id.GetPduSessionID2Value()

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
			ue.Log.Warn("Emergency PDU Session is not supported")
			err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.Log.Info("sent downlink nas transport to UE")
			return nil
		}
	}

	if smContextExist && requestType != nil {
		/* AMF releases context locally as this is duplicate pdu session */
		if requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeInitialRequest {
			delete(ue.SmContextList, pduSessionID)
			smContextExist = false
		}
	}

	if !smContextExist {
		msg := new(nas.Message)
		if err := msg.PlainNasDecode(&smMessage); err != nil {
			ue.Log.Error("Could not decode Nas message", zap.Error(err))
		}
		if msg.GsmMessage != nil && msg.GsmMessage.Status5GSM != nil {
			ue.Log.Warn("SmContext doesn't exist, 5GSM Status message received from UE", zap.Any("cause", msg.GsmMessage.Status5GSM.Cause5GSM))
			return nil
		}
	}
	// AMF has a PDU session routing context for the PDU session ID and the UE
	if smContextExist {
		// case i) Request type IE is either not included
		if requestType == nil {
			return forward5GSMMessageToSMF(ctx, ue, pduSessionID, smContext.SmContextRef(), smMessage)
		}

		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialRequest:
			//  perform a local release of the PDU session identified by the PDU session ID and shall request
			// the SMF to perform a local release of the PDU session
			ue.Log.Warn("Duplicated PDU session ID", zap.Uint8("pduSessionID", pduSessionID))
			n2Rsp, err := pdusession.UpdateSmContextCauseDuplicatePDUSessionID(ctx, smContext.SmContextRef())
			if err != nil {
				return fmt.Errorf("couldn't send update sm context request for duplicate pdu session id: %s", err)
			}
			ue.Log.Debug("AMF Transfer NGAP PDU Session Resource Release Command from SMF")
			list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
			send.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, n2Rsp)
			err = ue.RanUe.Ran.NGAPSender.SendPDUSessionResourceReleaseCommand(ctx, ue.RanUe.AmfUeNgapID, ue.RanUe.RanUeNgapID, nil, list)
			if err != nil {
				return fmt.Errorf("error sending pdu session resource release command: %s", err)
			}
			ue.Log.Info("sent pdu session resource release command to UE")

		// case ii) AMF has a PDU session routing context, and Request type is "existing PDU session"
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			if ue.InAllowedNssai(smContext.Snssai()) {
				return forward5GSMMessageToSMF(ctx, ue, pduSessionID, smContext.SmContextRef(), smMessage)
			}

			ue.Log.Error("S-NSSAI is not allowed for access type", zap.Any("snssai", smContext.Snssai()), zap.Uint8("pduSessionID", pduSessionID))
			err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.Log.Info("sent downlink nas transport to UE")
		// other requestType: AMF forward the 5GSM message, and the PDU session ID IE towards the SMF identified
		// by the SMF ID of the PDU session routing context
		default:
			return forward5GSMMessageToSMF(ctx, ue, pduSessionID, smContext.SmContextRef(), smMessage)
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

			newSmContext := context.NewSmContext()

			newSmContext.SetSnssai(snssai)

			smContextRef, errResponse, err := pdusession.CreateSmContext(ctx, ue.Supi, pduSessionID, dnn, &snssai, smMessage)
			if err != nil {
				ue.Log.Error("couldn't send create sm context request", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
			}

			if errResponse != nil {
				err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, errResponse, pduSessionID, 0)
				if err != nil {
					return fmt.Errorf("error sending downlink nas transport: %s", err)
				}

				return fmt.Errorf("pdu session establishment request was rejected by SMF for pdu session id %d", pduSessionID)
			}

			newSmContext.SetSmContextRef(smContextRef)

			ue.StoreSmContext(pduSessionID, newSmContext)
			ue.Log.Debug("Created sm context for pdu session", zap.Uint8("pduSessionID", pduSessionID))

		case nasMessage.ULNASTransportRequestTypeModificationRequest:
			fallthrough
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			err := message.SendDLNASTransport(ctx, ue.RanUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded)
			if err != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", err)
			}
			ue.Log.Info("sent downlink nas transport to UE")
		default:
		}
	}
	return nil
}

func handleULNASTransport(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle UL NAS Transport", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "AMF NAS HandleULNASTransport")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != context.Registered {
		return fmt.Errorf("expected UE to be in state %s during UL NAS Transport, instead it was %s", context.Registered, ue.State.Current())
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	switch msg.ULNASTransport.GetPayloadContainerType() {
	// TS 24.501 5.4.5.2.3 case a)
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return transport5GSMMessage(ctx, ue, msg.ULNASTransport)
	case nasMessage.PayloadContainerTypeSMS:
		return fmt.Errorf("PayloadContainerTypeSMS has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeLPP:
		return fmt.Errorf("PayloadContainerTypeLPP has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeSOR:
		return fmt.Errorf("PayloadContainerTypeSOR has not been implemented yet in UL NAS TRANSPORT")
	case nasMessage.PayloadContainerTypeUEPolicy:
		ue.Log.Info("AMF Transfer UEPolicy To PCF")
	case nasMessage.PayloadContainerTypeUEParameterUpdate:
		ue.Log.Info("AMF Transfer UEParameterUpdate To UDM")
		upuMac, err := nasConvert.UpuAckToModels(msg.ULNASTransport.PayloadContainer.GetPayloadContainerContents())
		if err != nil {
			return fmt.Errorf("failed to convert UPU ACK to models: %v", err)
		}
		ue.Log.Debug("UpuMac in UPU ACK NAS Msg", zap.String("UpuMac", upuMac))
	case nasMessage.PayloadContainerTypeMultiplePayload:
		return fmt.Errorf("PayloadContainerTypeMultiplePayload has not been implemented yet in UL NAS TRANSPORT")
	}
	return nil
}
