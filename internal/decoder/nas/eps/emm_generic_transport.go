// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/lpp"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type GenericMessageContainer struct {
	RawHex     string   `json:"raw_hex"`
	LppMessage *lpp.PDU `json:"lpp_message,omitempty"`

	Error string `json:"error,omitempty"`
}

type GenericNASTransport struct {
	GenericMessageContainerType utils.EnumField         `json:"generic_message_container_type"`
	GenericMessageContainer     GenericMessageContainer `json:"generic_message_container"`
	AdditionalInformation       *utils.RawOctets        `json:"additional_information,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDownlinkGenericNASTransport(msg *eps.DownlinkGenericNASTransport) *GenericNASTransport {
	return buildGenericNASTransport(msg.ContainerType, msg.Container, msg.AdditionalInformation, msg.Unrecognized)
}

func buildUplinkGenericNASTransport(msg *eps.UplinkGenericNASTransport) *GenericNASTransport {
	return buildGenericNASTransport(msg.ContainerType, msg.Container, msg.AdditionalInformation, msg.Unrecognized)
}

func buildGenericNASTransport(containerType eps.GenericMessageContainerType, container, additionalInformation []byte, unrecognized []nas.RawIE) *GenericNASTransport {
	out := &GenericNASTransport{
		GenericMessageContainerType: utils.NamedEnum(uint8(containerType), containerType.Name()),
		GenericMessageContainer:     GenericMessageContainer{RawHex: hex.EncodeToString(container)},
		AdditionalInformation:       utils.NewRawOctets(additionalInformation),
		UnrecognizedIEs:             utils.RawIEs(unrecognized),
	}

	switch containerType {
	case eps.GenericMessageContainerTypeLPP:
		out.GenericMessageContainer.LppMessage = lpp.Decode(container)
	default:
		out.GenericMessageContainer.Error = "Generic message container type not yet implemented"
	}

	return out
}
