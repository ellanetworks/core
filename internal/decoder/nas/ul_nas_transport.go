package nas

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"go.uber.org/zap"
)

type PayloadContainer struct {
	Raw        []byte      `json:"raw"`
	GsmMessage *GsmMessage `json:"gsm_message,omitempty"`
}

type ULNASTransport struct {
	ExtendedProtocolDiscriminator         uint8            `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType   uint8            `json:"spare_half_octet_and_security_header_type"`
	ULNASTRANSPORTMessageIdentity         string           `json:"ul_nas_transport_message_identity"`
	SpareHalfOctetAndPayloadContainerType uint8            `json:"spare_half_octet_and_payload_container_type"`
	PayloadContainer                      PayloadContainer `json:"payload_container"`
	PduSessionID2Value                    *uint8           `json:"pdu_session_id_2_value,omitempty"`
	OldPDUSessionID                       *uint8           `json:"old_pdu_session_id,omitempty"`
	RequestType                           *string          `json:"request_type,omitempty"`
	SNSSAI                                *SNSSAI          `json:"snssai,omitempty"`
	DNN                                   *string          `json:"dnn,omitempty"`
}

func buildULNASTransport(msg *nasMessage.ULNASTransport) *ULNASTransport {
	if msg == nil {
		return nil
	}

	ulNasTransport := &ULNASTransport{
		ExtendedProtocolDiscriminator:         msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:   msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ULNASTRANSPORTMessageIdentity:         nas.MessageName(msg.ULNASTRANSPORTMessageIdentity.Octet),
		SpareHalfOctetAndPayloadContainerType: msg.SpareHalfOctetAndPayloadContainerType.Octet,
	}

	ulNasTransport.PayloadContainer = buildULNASPayloadContainer(msg)

	if msg.PduSessionID2Value != nil {
		value := msg.PduSessionID2Value.GetPduSessionID2Value()
		ulNasTransport.PduSessionID2Value = &value
	}

	if msg.OldPDUSessionID != nil {
		value := msg.OldPDUSessionID.GetOldPDUSessionID()
		ulNasTransport.OldPDUSessionID = &value
	}

	if msg.RequestType != nil {
		value := ""
		switch msg.RequestType.GetRequestTypeValue() {
		case nasMessage.ULNASTransportRequestTypeInitialRequest:
			value = "InitialRequest"
		case nasMessage.ULNASTransportRequestTypeExistingPduSession:
			value = "ExistingPduSession"
		case nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest:
			value = "InitialEmergencyRequest"
		case nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession:
			value = "ExistingEmergencyPduSession"
		case nasMessage.ULNASTransportRequestTypeModificationRequest:
			value = "ModificationRequest"
		case nasMessage.ULNASTransportRequestTypeReserved:
			value = "Reserved"
		}
		ulNasTransport.RequestType = &value
	}

	if msg.SNSSAI != nil {
		snssai := buildNSSAI(msg.SNSSAI)
		ulNasTransport.SNSSAI = &snssai
	}

	if msg.DNN != nil {
		dnn := string(msg.DNN.GetDNN())
		ulNasTransport.DNN = &dnn
	}

	if msg.AdditionalInformation != nil {
		logger.EllaLog.Warn("AdditionalInformation not yet implemented")
	}

	return ulNasTransport
}

func buildULNASPayloadContainer(msg *nasMessage.ULNASTransport) PayloadContainer {
	containerType := msg.GetPayloadContainerType()

	payloadContainer := PayloadContainer{
		Raw: msg.GetPayloadContainerContents(),
	}

	if containerType != nasMessage.PayloadContainerTypeN1SMInfo {
		logger.EllaLog.Warn("Payload container type not yet implemented", zap.Uint8("type", containerType))
		return payloadContainer
	}

	rawBytes := msg.GetPayloadContainerContents()

	gsmMessage, err := decodeGSMMessage(rawBytes)
	if err != nil {
		logger.EllaLog.Warn("Failed to decode N1 SM message in UL NAS Transport Payload Container", zap.Error(err))
		return payloadContainer
	}

	payloadContainer.GsmMessage = gsmMessage

	return payloadContainer
}

func buildNSSAI(n *nasType.SNSSAI) SNSSAI {
	var out SNSSAI
	out.SST = int32(n.GetSST())

	if n.Len >= 4 {
		sd := n.Octet[1:4] // 3 bytes following SST
		sdStr := strings.ToUpper(hex.EncodeToString(sd))
		out.SD = &sdStr
	} else {
		out.SD = nil
	}

	return out
}

func decodeGSMMessage(raw []byte) (*GsmMessage, error) {
	m := nas.NewMessage()

	err := m.GsmMessageDecode(&raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode N1 SM message in UL NAS Transport Payload Container: %w", err)
	}

	gsmMessage := &GsmMessage{
		GsmHeader: GsmHeader{
			MessageType: getGsmMessageType(m.GsmMessage),
		},
	}

	switch m.GsmMessage.GetMessageType() {
	case nas.MsgTypePDUSessionEstablishmentRequest:
		gsmMessage.PDUSessionEstablishmentRequest = buildPDUSessionEstablishmentRequest(m.GsmMessage.PDUSessionEstablishmentRequest)
	case nas.MsgTypePDUSessionEstablishmentAccept:
		gsmMessage.PDUSessionEstablishmentAccept = buildPDUSessionEstablishmentAccept(m.GsmMessage.PDUSessionEstablishmentAccept)
	default:
		logger.EllaLog.Warn("GSM message type not yet implemented", zap.String("message_type", gsmMessage.GsmHeader.MessageType))
	}

	return gsmMessage, nil
}
