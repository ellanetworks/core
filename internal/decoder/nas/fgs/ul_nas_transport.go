// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/decoder/lpp"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type PayloadContainer struct {
	RawHex     string      `json:"raw_hex"`
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

	AdditionalInformation *utils.RawOctets `json:"additional_information,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
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

	out.AdditionalInformation = utils.NewRawOctets(msg.AdditionalInformation)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

func requestTypeEnum(rt uint8) utils.EnumField {
	return utils.NamedEnum(rt, fgs.RequestType(rt).Name())
}

func buildULPayloadContainer(containerType fgs.PayloadContainerType, contents []byte) PayloadContainer {
	payloadContainer := PayloadContainer{
		RawHex: hex.EncodeToString(contents),
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

	if s.MappedHPLMNSST != nil {
		sst := int32(*s.MappedHPLMNSST)
		out.MappedHPLMNSST = &sst
	}

	if s.MappedHPLMNSD != nil {
		sd := strings.ToUpper(hex.EncodeToString(s.MappedHPLMNSD[:]))
		out.MappedHPLMNSD = &sd
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
