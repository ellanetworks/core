// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/decoder/lpp"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type PayloadContainer struct {
	Raw        []byte      `json:"raw"`
	GsmMessage *GsmMessage `json:"gsm_message,omitempty"`
	LppMessage *lpp.PDU    `json:"lpp_message,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type ULNASTransport struct {
	SpareHalfOctetAndPayloadContainerType uint8            `json:"spare_half_octet_and_payload_container_type"`
	PayloadContainer                      PayloadContainer `json:"payload_container"`
	PduSessionID2Value                    *uint8           `json:"pdu_session_id_2_value,omitempty"`
	OldPDUSessionID                       *uint8           `json:"old_pdu_session_id,omitempty"`
	RequestType                           *utils.EnumField `json:"request_type,omitempty"`
	SNSSAI                                *SNSSAI          `json:"snssai,omitempty"`
	DNN                                   *string          `json:"dnn,omitempty"`

	AdditionalInformation *UnsupportedIE `json:"additional_information,omitempty"`
}

func buildULNASTransport(msg *fgs.ULNASTransport) *ULNASTransport {
	out := &ULNASTransport{
		SpareHalfOctetAndPayloadContainerType: uint8(msg.PayloadContainerType),
		PayloadContainer:                      buildULPayloadContainer(msg.PayloadContainerType, msg.PayloadContainer),
	}

	if msg.PDUSessionID != nil {
		out.PduSessionID2Value = sessionIDPtr(msg.PDUSessionID)
	}

	if msg.OldPDUSessionID != nil {
		out.OldPDUSessionID = sessionIDPtr(msg.OldPDUSessionID)
	}

	if msg.RequestType != nil {
		value := requestTypeEnum(uint8(*msg.RequestType))
		out.RequestType = &value
	}

	if msg.SNSSAI != nil {
		snssai := snssaiFromNAS(*msg.SNSSAI)
		out.SNSSAI = &snssai
	}

	if msg.DNN != nil {
		name := string(*msg.DNN)
		out.DNN = &name
	}

	if msg.AdditionalInformation != nil {
		out.AdditionalInformation = makeUnsupportedIE()
	}

	return out
}

// UL NAS transport request type values (TS 24.501 §9.11.3.47).
func requestTypeEnum(rt uint8) utils.EnumField {
	switch rt {
	case 1:
		return utils.MakeEnum(rt, "InitialRequest", false)
	case 2:
		return utils.MakeEnum(rt, "ExistingPduSession", false)
	case 3:
		return utils.MakeEnum(rt, "InitialEmergencyRequest", false)
	case 4:
		return utils.MakeEnum(rt, "ExistingEmergencyPduSession", false)
	case 5:
		return utils.MakeEnum(rt, "ModificationRequest", false)
	case 6:
		return utils.MakeEnum(rt, "Reserved", false)
	default:
		return utils.MakeEnum(rt, "", true)
	}
}

func buildULPayloadContainer(containerType fgs.PayloadContainerType, contents []byte) PayloadContainer {
	payloadContainer := PayloadContainer{
		Raw: contents,
	}

	switch containerType {
	case fgs.PayloadContainerTypeN1SMInfo:
		gsmMessage, err := decodeGSMMessage(contents)
		if err != nil {
			payloadContainer.Error = fmt.Sprintf("failed to decode N1 SM message in UL NAS Transport Payload Container: %v", err)
			return payloadContainer
		}

		payloadContainer.GsmMessage = gsmMessage

	case fgs.PayloadContainerTypeLPP:
		payloadContainer.LppMessage = lpp.Decode(contents)

	default:
		payloadContainer.Error = fmt.Sprintf("payload container type %d not yet implemented", containerType)
	}

	return payloadContainer
}

func snssaiFromNAS(s fgs.SNSSAI) SNSSAI {
	out := SNSSAI{SST: int32(s.SST)}
	if s.SD != nil {
		sd := strings.ToUpper(hex.EncodeToString(s.SD[:]))
		out.SD = &sd
	}

	return out
}

func decodeGSMMessage(raw []byte) (*GsmMessage, error) {
	gsm := buildGsmMessage(raw)
	if gsm == nil {
		return nil, fmt.Errorf("failed to decode N1 SM message in UL NAS Transport Payload Container: message too short")
	}

	return gsm, nil
}

// sessionIDPtr narrows an optional PDU session identity to the raw value the
// decoder's JSON shape carries.
func sessionIDPtr(id *fgs.PDUSessionID) *uint8 {
	if id == nil {
		return nil
	}

	v := uint8(*id)

	return &v
}
