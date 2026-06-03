package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func forward5GSMMessageToSMF(
	ctx context.Context,
	amfInstance *amf.AMF,
	ue *amf.AmfUe,
	pduSessionID uint8,
	smContextRef string,
	smMessage []byte,
) error {
	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("RAN UE context is nil, cannot forward 5GSM message to SMF")
	}

	response, err := amfInstance.Smf.UpdateSmContextN1Msg(ctx, smContextRef, smMessage)
	if err != nil {
		return fmt.Errorf("couldn't send update sm context request: %s", err)
	}

	if response == nil {
		ue.Log.Warn("SMF did not return any N1/N2 message", zap.Uint8("pduSessionID", pduSessionID))
		return nil
	}

	var n1Msg []byte

	if response.N1Msg != nil {
		ue.Log.Debug("Receive N1 SM Message from SMF")

		n1Msg, err = message.BuildDLNASTransport(ue, nasMessage.PayloadContainerTypeN1SMInfo, response.N1Msg, pduSessionID, nil)
		if err != nil {
			return fmt.Errorf("error building DL NAS Transport: %s", err)
		}
	}

	if response.N2Msg != nil {
		ue.Log.Debug("Receive N2 SM Information from SMF")

		if !response.ReleaseN2 {
			ue.Log.Debug("AMF forward N2 SM Information to UE")
			return nil
		}

		list := ngapType.PDUSessionResourceToReleaseListRelCmd{}
		send.AppendPDUSessionResourceToReleaseListRelCmd(&list, pduSessionID, response.N2Msg)

		err := ranUe.SendPDUSessionResourceReleaseCommand(ctx, n1Msg, list)
		if err != nil {
			return fmt.Errorf("error sending pdu session resource release command: %s", err)
		}

		ue.Log.Info("sent pdu session resource release command to UE")

		return nil
	}

	if n1Msg != nil {
		err := ranUe.SendDownlinkNasTransport(ctx, n1Msg, nil)
		if err != nil {
			return fmt.Errorf("error sending downlink nas transport: %s", err)
		}

		ue.Log.Info("sent downlink nas transport to UE")
	}

	return nil
}

func transport5GSMMessage(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, ulNasTransport *nasMessage.ULNASTransport) error {
	smMessage := ulNasTransport.GetPayloadContainerContents()

	id := ulNasTransport.PduSessionID2Value
	if id == nil {
		return fmt.Errorf("pdu session id is nil")
	}

	pduSessionID := id.GetPduSessionID2Value()

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("RAN UE context is nil, cannot transport 5GSM message")
	}

	if ulNasTransport.OldPDUSessionID != nil {
		return fmt.Errorf("old pdu session id is not supported")
	}

	// Reserved or unassigned PDU session identity value (TS 24.501 §7.3.2 c)).
	if pduSessionID < 1 || pduSessionID > 15 {
		return sendPayloadNotForwarded(ctx, ranUe, pduSessionID, smMessage)
	}

	requestType := ulNasTransport.RequestType

	if requestType != nil {
		switch requestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest,
			nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
			ue.Log.Warn("Emergency PDU Session is not supported")
			return sendPayloadNotForwarded(ctx, ranUe, pduSessionID, smMessage)
		}
	}

	smContext, smContextExist := ue.SmContextFindByPDUSessionID(pduSessionID)

	isInitialRequest := requestType != nil &&
		requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeInitialRequest

	// Duplicate PDU session ID: an initial request for an active session locally
	// releases it and re-establishes (TS 24.501 §5.4.5.2.5 item 12).
	if smContextExist && isInitialRequest {
		ue.DeleteSmContext(pduSessionID)

		smContext, smContextExist = nil, false
	}

	if smContextExist {
		// Existing PDU session whose S-NSSAI is not allowed for the access type
		// (TS 24.501 §5.4.5.2.5 item 14).
		if requestType != nil &&
			requestType.GetRequestTypeValue() == nasMessage.ULNASTransportRequestTypeExistingPduSession &&
			!ue.IsAllowedNssai(smContext.Snssai) {
			ue.Log.Error("S-NSSAI is not allowed for access type", zap.Any("snssai", smContext.Snssai), zap.Uint8("pduSessionID", pduSessionID))
			return sendPayloadNotForwarded(ctx, ranUe, pduSessionID, smMessage)
		}

		// Forward to the SMF of the routing context (TS 24.501 §5.4.5.2.3 i/ii).
		return forward5GSMMessageToSMF(ctx, amfInstance, ue, pduSessionID, smContext.Ref, smMessage)
	}

	if isInitialRequest {
		return establishPDUSession(ctx, amfInstance, ue, ranUe, ulNasTransport, pduSessionID, smMessage)
	}

	// A 5GSM STATUS for a PDU session with no context is ignored (TS 24.501 §7.3.2 d)).
	if isStatus5GSM(smMessage) {
		ue.Log.Warn("5GSM STATUS for unknown PDU session, ignoring", zap.Uint8("pduSessionID", pduSessionID))
		return nil
	}

	// No routing context and not an initial request (TS 24.501 §5.4.5.2.5 item 7).
	return sendPayloadNotForwarded(ctx, ranUe, pduSessionID, smMessage)
}

// sendPayloadNotForwarded returns the 5GSM message to the UE in a DL NAS
// TRANSPORT with 5GMM cause #90 "payload was not forwarded" (TS 24.501
// §5.4.5.3.1 case e)/f)).
func sendPayloadNotForwarded(ctx context.Context, ranUe *amf.RanUe, pduSessionID uint8, smMessage []byte) error {
	if err := message.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, smMessage, pduSessionID, nasMessage.Cause5GMMPayloadWasNotForwarded); err != nil {
		return fmt.Errorf("error sending downlink nas transport: %s", err)
	}

	return nil
}

func isStatus5GSM(smMessage []byte) bool {
	m := new(nas.Message)
	if err := m.PlainNasDecode(&smMessage); err != nil {
		return false
	}

	return m.GsmMessage != nil && m.Status5GSM != nil
}

// establishPDUSession selects an SMF and creates the SM context for an initial
// request (TS 24.501 §5.4.5.2.3 iii). When the SMF rejects, its reject message
// is returned to the UE; when it produces none, the payload is reported as not
// forwarded.
func establishPDUSession(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, ranUe *amf.RanUe, ulNasTransport *nasMessage.ULNASTransport, pduSessionID uint8, smMessage []byte) error {
	var (
		snssai *models.Snssai
		dnn    string
	)

	if ulNasTransport.SNSSAI != nil {
		snssai = util.SnssaiToModels(ulNasTransport.SNSSAI)
	} else {
		if len(ue.Current().AllowedNssai) == 0 {
			return fmt.Errorf("allowed nssai is empty in UE context")
		}

		snssai = &ue.Current().AllowedNssai[0]
	}

	if ulNasTransport.DNN != nil && ulNasTransport.DNN.GetLen() > 0 {
		dnn = ulNasTransport.GetDNN()
	} else {
		dnnResp, err := amfInstance.GetSubscriberDnn(ctx, ue.Supi, snssai)
		if err != nil {
			return fmt.Errorf("failed to get subscriber data: %v", err)
		}

		dnn = dnnResp
	}

	smContextRef, errResponse, err := amfInstance.Smf.CreateSmContext(ctx, ue.Supi, pduSessionID, dnn, snssai, smMessage)
	if err != nil {
		ue.Log.Error("couldn't create sm context", zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))

		if errResponse != nil {
			if sendErr := message.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, errResponse, pduSessionID, 0); sendErr != nil {
				return fmt.Errorf("error sending downlink nas transport: %s", sendErr)
			}

			return fmt.Errorf("pdu session establishment request was rejected by SMF for pdu session id %d: %w", pduSessionID, err)
		}

		if sendErr := sendPayloadNotForwarded(ctx, ranUe, pduSessionID, smMessage); sendErr != nil {
			return sendErr
		}

		return fmt.Errorf("create sm context failed for pdu session id %d: %w", pduSessionID, err)
	}

	if errResponse != nil {
		if err := message.SendDLNASTransport(ctx, ranUe, nasMessage.PayloadContainerTypeN1SMInfo, errResponse, pduSessionID, 0); err != nil {
			return fmt.Errorf("error sending downlink nas transport: %s", err)
		}

		return fmt.Errorf("pdu session establishment request was rejected by SMF for pdu session id %d", pduSessionID)
	}

	// The SMF processed the message but produced no context and no response,
	// e.g. an establishment request with a reserved PTI it had to ignore
	// (TS 24.501 §7.3.1 d). Send nothing.
	if smContextRef == "" {
		ue.Log.Info("SMF ignored the PDU session establishment request, sending no response", zap.Uint8("pduSessionID", pduSessionID))
		return nil
	}

	if err := ue.CreateSmContext(pduSessionID, smContextRef, snssai); err != nil {
		return fmt.Errorf("error creating SM context: %w", err)
	}

	ue.Log.Debug("Created sm context for pdu session", zap.Uint8("pduSessionID", pduSessionID))

	return nil
}

func handleULNASTransport(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe, msg *nasMessage.ULNASTransport, macFailed bool) error {
	if ue.GetState() != amf.Registered {
		return fmt.Errorf("expected UE to be in state %s during UL NAS Transport, instead it was %s", amf.Registered, ue.GetState())
	}

	if macFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	switch msg.GetPayloadContainerType() {
	// TS 24.501 5.4.5.2.3 case a)
	case nasMessage.PayloadContainerTypeN1SMInfo:
		return transport5GSMMessage(ctx, amfInstance, ue, msg)
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

		upuMac, err := nasConvert.UpuAckToModels(msg.GetPayloadContainerContents())
		if err != nil {
			return fmt.Errorf("failed to convert UPU ACK to models: %v", err)
		}

		ue.Log.Debug("UpuMac in UPU ACK NAS Msg", zap.String("UpuMac", upuMac))
	case nasMessage.PayloadContainerTypeMultiplePayload:
		return fmt.Errorf("PayloadContainerTypeMultiplePayload has not been implemented yet in UL NAS TRANSPORT")
	}

	return nil
}
