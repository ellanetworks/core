// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

type ERABQoSExtensions struct {
	DownlinkPacketLossRate *uint16 `json:"downlink_packet_loss_rate,omitempty"`
	UplinkPacketLossRate   *uint16 `json:"uplink_packet_loss_rate,omitempty"`
}

type ERABSetupExtensions struct {
	CorrelationID                 string           `json:"correlation_id,omitempty"`
	SIPTOCorrelationID            string           `json:"sipto_correlation_id,omitempty"`
	BearerType                    *utils.EnumField `json:"bearer_type,omitempty"`
	EthernetType                  *utils.EnumField `json:"ethernet_type,omitempty"`
	IntegrityProtectionIndication *utils.EnumField `json:"integrity_protection_indication,omitempty"`
	DataForwardingNotPossible     *bool            `json:"data_forwarding_not_possible,omitempty"`
}

func qosExtensions(q s1ap.ERABLevelQoSParameters) ERABQoSExtensions {
	var out ERABQoSExtensions

	if q.DownlinkPacketLossRate != nil {
		v := uint16(*q.DownlinkPacketLossRate)
		out.DownlinkPacketLossRate = &v
	}

	if q.UplinkPacketLossRate != nil {
		v := uint16(*q.UplinkPacketLossRate)
		out.UplinkPacketLossRate = &v
	}

	return out
}

func correlationID(c *s1ap.CorrelationID) string {
	if c == nil {
		return ""
	}

	return hex.EncodeToString(c[:])
}

func integrityProtectionIndication(s *s1ap.SecurityIndication) *utils.EnumField {
	if s == nil {
		return nil
	}

	v := utils.NamedEnum(uint8(s.IntegrityProtectionIndication), s.IntegrityProtectionIndication.Name())

	return &v
}

func setupExtensions(c, sipto *s1ap.CorrelationID, bearer *s1ap.BearerType, ethernet *s1ap.EthernetType, sec *s1ap.SecurityIndication) ERABSetupExtensions {
	out := ERABSetupExtensions{
		CorrelationID:                 correlationID(c),
		SIPTOCorrelationID:            correlationID(sipto),
		IntegrityProtectionIndication: integrityProtectionIndication(sec),
	}

	if bearer != nil {
		v := utils.NamedEnum(uint8(*bearer), bearer.Name())
		out.BearerType = &v
	}

	if ethernet != nil {
		v := utils.NamedEnum(uint8(*ethernet), ethernet.Name())
		out.EthernetType = &v
	}

	return out
}

func extendedBitRate(b *s1ap.ExtendedBitRate) *uint64 {
	if b == nil {
		return nil
	}

	v := uint64(*b)

	return &v
}
