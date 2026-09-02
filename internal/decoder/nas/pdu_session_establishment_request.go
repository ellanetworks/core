// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	epsdec "github.com/ellanetworks/core/internal/decoder/eps"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type IntegrityProtectionMaximumDataRate struct {
	Uplink   uint8 `json:"uplink"`
	Downlink uint8 `json:"downlink"`
}

type Capability5GSM struct {
	RqoS   uint8 `json:"rqo_s"`
	MH6PDU uint8 `json:"mh_6_pdu"`
}

type PDUSessionEstablishmentRequest struct {
	IntegrityProtectionMaximumDataRate   IntegrityProtectionMaximumDataRate           `json:"integrity_protection_maximum_data_rate"`
	PDUSessionType                       *utils.EnumField                             `json:"pdu_session_type,omitempty"`
	SSCMode                              *uint8                                       `json:"ssc_mode,omitempty"`
	Capability5GSM                       *Capability5GSM                              `json:"capability_5g_s_m,omitempty"`
	ExtendedProtocolConfigurationOptions *epsdec.ExtendedProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`

	MaximumNumberOfSupportedPacketFilters *uint16 `json:"maximum_number_of_supported_packet_filters,omitempty"`
	AlwaysonPDUSessionRequested           *bool   `json:"alwayson_pdu_session_requested,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildPDUSessionEstablishmentRequest(msg *fgs.PDUSessionEstablishmentRequest) *PDUSessionEstablishmentRequest {
	out := &PDUSessionEstablishmentRequest{
		IntegrityProtectionMaximumDataRate: IntegrityProtectionMaximumDataRate{
			Uplink:   msg.IntegrityProtMaxDataRate[0],
			Downlink: msg.IntegrityProtMaxDataRate[1],
		},
	}

	if msg.PDUSessionType != nil {
		sessionType := buildPDUSessionType(uint8(*msg.PDUSessionType))
		out.PDUSessionType = &sessionType
	}

	if msg.SSCMode != nil {
		mode := uint8(*msg.SSCMode)
		out.SSCMode = &mode
	}

	if msg.GSMCapability != nil {
		out.Capability5GSM = &Capability5GSM{
			RqoS:   b2u(msg.GSMCapability.RqoS),
			MH6PDU: b2u(msg.GSMCapability.MH6PDU),
		}
	}

	out.AlwaysonPDUSessionRequested = msg.AlwaysOnRequested

	out.ExtendedProtocolConfigurationOptions = epsdec.ExtendedPCO(msg.ExtendedPCO)

	for _, ie := range msg.Unrecognized {
		switch ie.IEI {
		case ieiMaxPacketFilters:
			out.MaximumNumberOfSupportedPacketFilters = maxSupportedPacketFilters(ie.Value)
		case ieiExtendedPCO:
			// The element reached Unrecognized because its content did not decode.
			out.ExtendedProtocolConfigurationOptions = &epsdec.ExtendedProtocolConfigurationOptions{
				Error: "failed to parse extended protocol configuration options content",
			}
		}
	}

	out.UnrecognizedIEs = utils.RawIEsExcept(msg.Unrecognized, ieiMaxPacketFilters, ieiExtendedPCO)

	return out
}

func buildPDUSessionType(sessType uint8) utils.EnumField {
	return utils.NamedEnum(sessType, fgs.PDUSessionType(sessType).Name())
}

// maxSupportedPacketFilters reads the 11-bit count TS 24.501 §9.11.4.9 spreads
// over two octets: bit 8 of the first is the most significant bit and bit 6 of
// the second the least, leaving the low five bits of the second spare.
func maxSupportedPacketFilters(v []byte) *uint16 {
	if len(v) != 2 {
		return nil
	}

	n := uint16(v[0])<<3 | uint16(v[1])>>5

	return &n
}
