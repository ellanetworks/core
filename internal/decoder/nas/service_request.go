package nas

import (
	"fmt"

	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
)

type TMSI5GS struct {
	TypeOfIdentity string   `json:"type_of_identity"`
	AMFSetID       uint16   `json:"amf_set_id"`
	AMFPointer     uint8    `json:"amf_pointer"`
	TMSI5G         [4]uint8 `json:"tmsi_5g"`
}

func buildTMSI5GS(tmsi5gs nasType.TMSI5GS) TMSI5GS {
	var typeOfIdentity string
	switch tmsi5gs.GetTypeOfIdentity() {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		typeOfIdentity = "NoIdentity"
	case nasMessage.MobileIdentity5GSTypeSuci:
		typeOfIdentity = "Suci"
	case nasMessage.MobileIdentity5GSType5gGuti:
		typeOfIdentity = "5gGuti"
	case nasMessage.MobileIdentity5GSTypeImei:
		typeOfIdentity = "Imei"
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		typeOfIdentity = "5gSTmsi"
	case nasMessage.MobileIdentity5GSTypeImeisv:
		typeOfIdentity = "Imeisv"
	default:
		typeOfIdentity = fmt.Sprintf("Unknown(%d)", tmsi5gs.GetTypeOfIdentity())
	}

	return TMSI5GS{
		TypeOfIdentity: typeOfIdentity,
		AMFSetID:       tmsi5gs.GetAMFSetID(),
		AMFPointer:     tmsi5gs.GetAMFPointer(),
		TMSI5G:         tmsi5gs.GetTMSI5G(),
	}
}

type UplinkDataStatusPDU struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type PDUSessionStatusPDU struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type AllowedPDUSessionStatus struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type ServiceRequest struct {
	ExtendedProtocolDiscriminator       uint8                     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                     `json:"spare_half_octet_and_security_header_type"`
	ServiceRequestMessageIdentity       string                    `json:"service_request_message_identity"`
	ServiceTypeAndNgksi                 string                    `json:"service_type_and_ngksi"`
	TMSI5GS                             TMSI5GS                   `json:"tmsi_5gs,omitempty"`
	UplinkDataStatus                    []UplinkDataStatusPDU     `json:"uplink_data_status,omitempty"`
	PDUSessionStatus                    []PDUSessionStatusPDU     `json:"pdu_session_status,omitempty"`
	AllowedPDUSessionStatus             []AllowedPDUSessionStatus `json:"allowed_pdu_session_status,omitempty"`
	NASMessageContainer                 []byte                    `json:"nas_message_container,omitempty"`
}

func buildServiceRequest(msg *nasMessage.ServiceRequest) *ServiceRequest {
	if msg == nil {
		return nil
	}

	serviceRequest := &ServiceRequest{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		ServiceRequestMessageIdentity:       nas.MessageName(msg.ServiceRequestMessageIdentity.Octet),
		ServiceTypeAndNgksi:                 nas.MessageName(msg.ServiceTypeAndNgksi.Octet),
		TMSI5GS:                             buildTMSI5GS(msg.TMSI5GS),
	}

	if msg.UplinkDataStatus != nil {
		uplinkDataStatus := []UplinkDataStatusPDU{}
		uplinkDataPsi := nasConvert.PSIToBooleanArray(msg.UplinkDataStatus.Buffer)
		for pduSessionID, hasUplinkData := range uplinkDataPsi {
			uplinkDataStatus = append(uplinkDataStatus, UplinkDataStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       hasUplinkData,
			})
		}
		serviceRequest.UplinkDataStatus = uplinkDataStatus
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
		serviceRequest.PDUSessionStatus = pduSessionStatus
	}

	if msg.AllowedPDUSessionStatus != nil {
		allowedPduSessionStatus := []AllowedPDUSessionStatus{}
		allowedPsis := nasConvert.PSIToBooleanArray(msg.AllowedPDUSessionStatus.Buffer)
		for pduSessionID, isAllowed := range allowedPsis {
			allowedPduSessionStatus = append(allowedPduSessionStatus, AllowedPDUSessionStatus{
				PDUSessionID: pduSessionID,
				Active:       isAllowed,
			})
		}
		serviceRequest.AllowedPDUSessionStatus = allowedPduSessionStatus
	}

	if msg.NASMessageContainer != nil {
		serviceRequest.NASMessageContainer = msg.NASMessageContainer.GetNASMessageContainerContents()
	}

	return serviceRequest
}
