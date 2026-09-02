// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"
	"time"

	"github.com/ellanetworks/core/internal/decoder/lpp"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type DLNASTransport struct {
	SpareHalfOctetAndPayloadContainerType uint8            `json:"spare_half_octet_and_payload_container_type"`
	PayloadContainer                      PayloadContainer `json:"payload_container"`
	PduSessionID2Value                    *uint8           `json:"pdu_session_id_2_value,omitempty"`
	Cause5GMM                             *utils.EnumField `json:"cause_5gmm,omitempty"`
	BackoffTimerSeconds                   *uint32          `json:"backoff_timer_seconds,omitempty"`

	AdditionalInformation *utils.RawOctets `json:"additional_information,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildDLNASTransport(msg *fgs.DLNASTransport) *DLNASTransport {
	out := &DLNASTransport{
		SpareHalfOctetAndPayloadContainerType: uint8(msg.PayloadContainerType),
		PayloadContainer:                      buildDLPayloadContainer(msg.PayloadContainerType, msg.PayloadContainer),
	}

	if msg.PDUSessionID != nil {
		out.PduSessionID2Value = new(uint8(*msg.PDUSessionID))
	}

	if msg.BackoffTimer != nil {
		if d, ok := msg.BackoffTimer.Duration(); ok {
			secs := uint32(d / time.Second)
			out.BackoffTimerSeconds = &secs
		}
	}

	if msg.Cause != nil {
		cause := cause5GMMToEnum(*msg.Cause)
		out.Cause5GMM = &cause
	}

	out.AdditionalInformation = utils.NewRawOctets(msg.AdditionalInfo)
	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

func buildDLPayloadContainer(containerType fgs.PayloadContainerType, contents []byte) PayloadContainer {
	payloadContainer := PayloadContainer{
		RawHex: hex.EncodeToString(contents),
	}

	switch containerType {
	case fgs.PayloadContainerTypeN1SMInfo:
		gsmMessage, err := decodeGSMMessage(contents)
		if err != nil {
			payloadContainer.Error = "Failed to decode N1 SM message: " + err.Error()
			return payloadContainer
		}

		payloadContainer.GsmMessage = gsmMessage

	case fgs.PayloadContainerTypeLPP:
		payloadContainer.LppMessage = lpp.Decode(contents)

	default:
		payloadContainer.Error = "Payload container type not yet implemented"
	}

	return payloadContainer
}
