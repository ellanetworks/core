package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

type TMSI5GS struct {
	TypeOfIdentity utils.EnumField[uint8] `json:"type_of_identity"`
	AMFSetID       uint16                 `json:"amf_set_id"`
	AMFPointer     uint8                  `json:"amf_pointer"`
	TMSI5G         [4]uint8               `json:"tmsi_5g"`
}

func buildTypeOfIdentityEnum(toi uint8) utils.EnumField[uint8] {
	switch toi {
	case nasMessage.MobileIdentity5GSTypeNoIdentity:
		return utils.MakeEnum(toi, "NoIdentity", false)
	case nasMessage.MobileIdentity5GSTypeSuci:
		return utils.MakeEnum(toi, "Suci", false)
	case nasMessage.MobileIdentity5GSType5gGuti:
		return utils.MakeEnum(toi, "5gGuti", false)
	case nasMessage.MobileIdentity5GSTypeImei:
		return utils.MakeEnum(toi, "Imei", false)
	case nasMessage.MobileIdentity5GSType5gSTmsi:
		return utils.MakeEnum(toi, "5gSTmsi", false)
	case nasMessage.MobileIdentity5GSTypeImeisv:
		return utils.MakeEnum(toi, "Imeisv", false)
	default:
		return utils.MakeEnum(toi, "", true)
	}
}

func buildTMSI5GS(tmsi5gs nasType.TMSI5GS) TMSI5GS {
	return TMSI5GS{
		TypeOfIdentity: buildTypeOfIdentityEnum(tmsi5gs.GetTypeOfIdentity()),
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

type ServiceTypeAndNgksi struct {
	ServiceType          utils.EnumField[uint8] `json:"service_type"`
	TSC                  utils.EnumField[uint8] `json:"tsc"`
	NasKeySetIdentifiler uint8                  `json:"nas_key_set_identifier"`
}

type ServiceRequest struct {
	ExtendedProtocolDiscriminator       uint8                     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                     `json:"spare_half_octet_and_security_header_type"`
	ServiceTypeAndNgksi                 ServiceTypeAndNgksi       `json:"service_type_and_ngksi"`
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
		ServiceTypeAndNgksi:                 buildServiceTypeAndNgksi(msg.ServiceTypeAndNgksi),
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

func buildServiceTypeAndNgksi(stng nasType.ServiceTypeAndNgksi) ServiceTypeAndNgksi {
	return ServiceTypeAndNgksi{
		ServiceType:          buildServiceTypeEnum(stng.GetServiceTypeValue()),
		TSC:                  buildTSCEnum(stng.GetTSC()),
		NasKeySetIdentifiler: stng.GetNasKeySetIdentifiler(),
	}
}

func buildServiceTypeEnum(serviceType uint8) utils.EnumField[uint8] {
	switch serviceType {
	case nasMessage.ServiceTypeSignalling:
		return utils.MakeEnum(serviceType, "Signalling", false)
	case nasMessage.ServiceTypeData:
		return utils.MakeEnum(serviceType, "Data", false)
	case nasMessage.ServiceTypeMobileTerminatedServices:
		return utils.MakeEnum(serviceType, "MobileTerminatedServices", false)
	case nasMessage.ServiceTypeEmergencyServices:
		return utils.MakeEnum(serviceType, "EmergencyServices", false)
	case nasMessage.ServiceTypeEmergencyServicesFallback:
		return utils.MakeEnum(serviceType, "EmergencyServicesFallback", false)
	case nasMessage.ServiceTypeHighPriorityAccess:
		return utils.MakeEnum(serviceType, "HighPriorityAccess", false)
	default:
		return utils.MakeEnum(serviceType, "", true)
	}
}

func buildTSCEnum(tsc uint8) utils.EnumField[uint8] {
	switch tsc {
	case nasMessage.TypeOfSecurityContextFlagNative:
		return utils.MakeEnum(tsc, "Native", false)
	case nasMessage.TypeOfSecurityContextFlagMapped:
		return utils.MakeEnum(tsc, "Mapped", false)
	default:
		return utils.MakeEnum(tsc, "", true)
	}
}
