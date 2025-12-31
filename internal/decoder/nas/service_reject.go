package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
)

type ServiceReject struct {
	ExtendedProtocolDiscriminator       uint8                  `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                  `json:"spare_half_octet_and_security_header_type"`
	Cause5GMM                           utils.EnumField[uint8] `json:"cause"`
	PDUSessionStatus                    []PDUSessionStatusPDU  `json:"pdu_session_status,omitempty"`
	T3346Value                          *uint8                 `json:"t3346_value,omitempty"`
	EAPMessage                          []byte                 `json:"eap_message,omitempty"`
}

func buildServiceReject(msg *nasMessage.ServiceReject) *ServiceReject {
	if msg == nil {
		return nil
	}

	serviceReject := &ServiceReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		Cause5GMM:                           cause5GMMToEnum(msg.Cause5GMM.GetCauseValue()),
	}

	if msg.PDUSessionStatus != nil {
		pduSessionStatus := []PDUSessionStatusPDU{}

		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionStatus.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionStatus = append(pduSessionStatus, PDUSessionStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}

		serviceReject.PDUSessionStatus = pduSessionStatus
	}

	if msg.T3346Value != nil {
		t3346Value := msg.T3346Value.GetGPRSTimer2Value()
		serviceReject.T3346Value = &t3346Value
	}

	if msg.EAPMessage != nil {
		serviceReject.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return serviceReject
}
