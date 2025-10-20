package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
)

type DLNASTransport struct {
	ExtendedProtocolDiscriminator         uint8                   `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType   uint8                   `json:"spare_half_octet_and_security_header_type"`
	DLNASTRANSPORTMessageIdentity         string                  `json:"dl_nas_transport_message_identity"`
	SpareHalfOctetAndPayloadContainerType uint8                   `json:"spare_half_octet_and_payload_container_type"`
	PayloadContainer                      PayloadContainer        `json:"payload_container"`
	PduSessionID2Value                    *uint8                  `json:"pdu_session_id_2_value,omitempty"`
	Cause5GMM                             *utils.EnumField[uint8] `json:"cause_5gmm,omitempty"`
	BackoffTimerValue                     *uint8                  `json:"backoff_timer_value,omitempty"`
	Ipaddr                                string                  `json:"ip_addr,omitempty"`

	AdditionalInformation *UnsupportedIE `json:"additional_information,omitempty"`
}

func buildDLNASTransport(msg *nasMessage.DLNASTransport) *DLNASTransport {
	if msg == nil {
		return nil
	}

	dlNasTransport := &DLNASTransport{
		ExtendedProtocolDiscriminator:         msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType:   msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		DLNASTRANSPORTMessageIdentity:         nas.MessageName(msg.DLNASTRANSPORTMessageIdentity.Octet),
		SpareHalfOctetAndPayloadContainerType: msg.SpareHalfOctetAndPayloadContainerType.Octet,
		Ipaddr:                                msg.Ipaddr,
	}

	dlNasTransport.PayloadContainer = buildDLNASPayloadContainer(msg)

	if msg.PduSessionID2Value != nil {
		value := msg.PduSessionID2Value.GetPduSessionID2Value()
		dlNasTransport.PduSessionID2Value = &value
	}

	if msg.AdditionalInformation != nil {
		dlNasTransport.AdditionalInformation = makeUnsupportedIE()
	}

	if msg.BackoffTimerValue != nil {
		backoffTimerValue := msg.BackoffTimerValue.GetUnitTimerValue()
		dlNasTransport.BackoffTimerValue = &backoffTimerValue
	}

	if msg.Cause5GMM != nil {
		cause := cause5GMMToEnum(msg.Cause5GMM.GetCauseValue())
		dlNasTransport.Cause5GMM = &cause
	}

	return dlNasTransport
}

func buildDLNASPayloadContainer(msg *nasMessage.DLNASTransport) PayloadContainer {
	containerType := msg.GetPayloadContainerType()

	payloadContainer := PayloadContainer{
		Raw: msg.GetPayloadContainerContents(),
	}

	if containerType != nasMessage.PayloadContainerTypeN1SMInfo {
		payloadContainer.Error = "Payload container type not yet implemented"
		return payloadContainer
	}

	rawBytes := msg.GetPayloadContainerContents()

	gsmMessage, err := decodeGSMMessage(rawBytes)
	if err != nil {
		payloadContainer.Error = "Failed to decode N1 SM message: " + err.Error()
		return payloadContainer
	}

	payloadContainer.GsmMessage = gsmMessage

	return payloadContainer
}
