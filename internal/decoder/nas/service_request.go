// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type TMSI5GS struct {
	TypeOfIdentity utils.EnumField `json:"type_of_identity"`
	AMFSetID       uint16          `json:"amf_set_id"`
	AMFPointer     uint8           `json:"amf_pointer"`
	TMSI5G         [4]uint8        `json:"tmsi_5g"`
}

// buildTMSI5GS renders a decoded 5G-S-TMSI mobile identity (TS 24.501 §9.11.3.4).
func buildTMSI5GS(id fgs.MobileIdentity) TMSI5GS {
	s := id.STMSI
	if s == nil {
		return TMSI5GS{}
	}

	return TMSI5GS{
		TypeOfIdentity: buildTypeOfIdentityEnum(uint8(fgs.IdentitySTMSI)),
		AMFSetID:       s.AMFSetID,
		AMFPointer:     s.AMFPointer,
		TMSI5G:         s.TMSI,
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
	ServiceType          utils.EnumField `json:"service_type"`
	TSC                  utils.EnumField `json:"tsc"`
	NasKeySetIdentifiler uint8           `json:"nas_key_set_identifier"`
}

type ServiceRequest struct {
	ServiceTypeAndNgksi     ServiceTypeAndNgksi       `json:"service_type_and_ngksi"`
	TMSI5GS                 TMSI5GS                   `json:"tmsi_5gs,omitempty"`
	UplinkDataStatus        []UplinkDataStatusPDU     `json:"uplink_data_status,omitempty"`
	PDUSessionStatus        []PDUSessionStatusPDU     `json:"pdu_session_status,omitempty"`
	AllowedPDUSessionStatus []AllowedPDUSessionStatus `json:"allowed_pdu_session_status,omitempty"`
	NASMessageContainer     []byte                    `json:"nas_message_container,omitempty"`
}

func buildServiceRequest(msg *fgs.ServiceRequest) *ServiceRequest {
	out := &ServiceRequest{
		ServiceTypeAndNgksi: ServiceTypeAndNgksi{
			ServiceType:          buildServiceTypeEnum(uint8(msg.ServiceType)),
			TSC:                  buildTSCEnum(msg.NgKSI.Mapped),
			NasKeySetIdentifiler: msg.NgKSI.Value,
		},
		TMSI5GS:             buildTMSI5GS(msg.MobileIdentity),
		NASMessageContainer: msg.NASMessageContainer,
	}

	if msg.UplinkDataStatus != nil {
		psi := msg.UplinkDataStatus.PSI
		uds := []UplinkDataStatusPDU{}

		for i := range 16 {
			uds = append(uds, UplinkDataStatusPDU{PDUSessionID: i, Active: psi[i]})
		}

		out.UplinkDataStatus = uds
	}

	if msg.PDUSessionStatus != nil {
		out.PDUSessionStatus = decodePDUSessionStatus(msg.PDUSessionStatus)
	}

	if msg.AllowedPDUSessionStatus != nil {
		psi := msg.AllowedPDUSessionStatus.PSI
		aps := []AllowedPDUSessionStatus{}

		for i := range 16 {
			aps = append(aps, AllowedPDUSessionStatus{PDUSessionID: i, Active: psi[i]})
		}

		out.AllowedPDUSessionStatus = aps
	}

	return out
}

func buildServiceTypeEnum(serviceType uint8) utils.EnumField {
	return utils.NamedEnum(serviceType, fgs.ServiceType(serviceType).Name())
}

func buildTSCEnum(mapped bool) utils.EnumField {
	switch mapped {
	case false:
		return utils.MakeEnum(uint8(0), "Native", false)
	default:
		return utils.MakeEnum(uint8(1), "Mapped", false)
	}
}
