package nas

import (
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
)

type ServiceReject struct {
	ExtendedProtocolDiscriminator       uint8                 `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                 `json:"spare_half_octet_and_security_header_type"`
	ServiceRejectMessageIdentity        string                `json:"service_reject_message_identity"`
	Cause5GMM                           string                `json:"cause"`
	PDUSessionStatus                    []PDUSessionStatusPDU `json:"pdu_session_status,omitempty"`
	T3346Value                          *uint8                `json:"t3346_value,omitempty"`
	EAPMessage                          []byte                `json:"eap_message,omitempty"`
}

func buildServiceReject(msg *nasMessage.ServiceReject) *ServiceReject {
	if msg == nil {
		return nil
	}

	serviceReject := &ServiceReject{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ServiceRejectMessageIdentity:        nas.MessageName(msg.ServiceRejectMessageIdentity.Octet),
		Cause5GMM:                           nasMessage.Cause5GMMToString(msg.Cause5GMM.Octet),
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
