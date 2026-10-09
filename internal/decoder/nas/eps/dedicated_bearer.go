// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"

	nasie "github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

type ActivateDedicatedBearer struct {
	LinkedEPSBearerIdentity              uint8                               `json:"linked_eps_bearer_identity"`
	EPSQoS                               *EPSQoS                             `json:"eps_qos,omitempty"`
	TFT                                  *TFT                                `json:"tft,omitempty"`
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	UnrecognizedIEs                      []utils.RawIE                       `json:"unrecognized_ies,omitempty"`
}

type ActivateDedicatedBearerAccept struct {
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	UnrecognizedIEs                      []utils.RawIE                       `json:"unrecognized_ies,omitempty"`
}

type ActivateDedicatedBearerReject struct {
	ESMCause                             utils.EnumField                     `json:"esm_cause"`
	ProtocolConfigurationOptions         *nasie.ProtocolConfigurationOptions `json:"protocol_configuration_options,omitempty"`
	ExtendedProtocolConfigurationOptions *nasie.ProtocolConfigurationOptions `json:"extended_protocol_configuration_options,omitempty"`
	UnrecognizedIEs                      []utils.RawIE                       `json:"unrecognized_ies,omitempty"`
}

type TFT struct {
	Operation         utils.EnumField     `json:"operation"`
	PacketFilters     []TFTPacketFilter   `json:"packet_filters,omitempty"`
	DeleteIdentifiers []uint8             `json:"delete_identifiers,omitempty"`
	Parameters        []TFTParameterField `json:"parameters,omitempty"`
}

type TFTPacketFilter struct {
	Identifier uint8           `json:"identifier"`
	Direction  utils.EnumField `json:"direction"`
	Precedence uint8           `json:"precedence"`
	Components []TFTComponent  `json:"components"`
}

type TFTComponent struct {
	Type  utils.EnumField `json:"type"`
	Value string          `json:"value"`
}

type TFTParameterField struct {
	Identifier  uint8  `json:"identifier"`
	ContentsHex string `json:"contents_hex"`
}

var tftOperationNames = map[eps.TFTOperation]string{
	eps.TFTIgnore:         "Ignore this IE",
	eps.TFTCreate:         "Create new TFT",
	eps.TFTDeleteExisting: "Delete existing TFT",
	eps.TFTAddFilters:     "Add packet filters to existing TFT",
	eps.TFTReplaceFilters: "Replace packet filters in existing TFT",
	eps.TFTDeleteFilters:  "Delete packet filters from existing TFT",
	eps.TFTNoOperation:    "No TFT operation",
}

var tftDirectionNames = map[eps.TFTDirection]string{
	eps.TFTPreRel7:       "pre Rel-7 TFT filter",
	eps.TFTDownlink:      "downlink only",
	eps.TFTUplink:        "uplink only",
	eps.TFTBidirectional: "bidirectional",
}

var tftComponentNames = map[eps.TFTComponentType]string{
	eps.TFTIPv4RemoteAddress:       "IPv4 remote address type",
	eps.TFTIPv4LocalAddress:        "IPv4 local address type",
	eps.TFTIPv6RemoteAddress:       "IPv6 remote address type",
	eps.TFTIPv6RemoteAddressPrefix: "IPv6 remote address/prefix length type",
	eps.TFTIPv6LocalAddressPrefix:  "IPv6 local address/prefix length type",
	eps.TFTProtocolIdentifier:      "Protocol identifier/Next header type",
	eps.TFTSingleLocalPort:         "Single local port type",
	eps.TFTLocalPortRange:          "Local port range type",
	eps.TFTSingleRemotePort:        "Single remote port type",
	eps.TFTRemotePortRange:         "Remote port range type",
	eps.TFTSecurityParameterIndex:  "Security parameter index type",
	eps.TFTTypeOfService:           "Type of service/Traffic class type",
	eps.TFTFlowLabel:               "Flow label type",
	eps.TFTDestinationMACAddress:   "Destination MAC address type",
	eps.TFTSourceMACAddress:        "Source MAC address type",
	eps.TFTCTagVID:                 "802.1Q C-TAG VID type",
	eps.TFTSTagVID:                 "802.1Q S-TAG VID type",
	eps.TFTCTagPCPDEI:              "802.1Q C-TAG PCP/DEI type",
	eps.TFTSTagPCPDEI:              "802.1Q S-TAG PCP/DEI type",
	eps.TFTEthertype:               "Ethertype type",
}

func buildActivateDedicatedBearer(msg *eps.ActivateDedicatedEPSBearerContextRequest) *ActivateDedicatedBearer {
	return &ActivateDedicatedBearer{
		LinkedEPSBearerIdentity:              uint8(msg.LinkedEPSBearerIdentity),
		EPSQoS:                               epsQoS(msg.EPSQoS),
		TFT:                                  tft(msg.TFT),
		ProtocolConfigurationOptions:         nasie.ExtendedPCO(msg.ProtocolConfigurationOptions),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions),
		UnrecognizedIEs:                      utils.RawIEs(msg.Unrecognized),
	}
}

func buildActivateDedicatedBearerAccept(msg *eps.ActivateDedicatedEPSBearerContextAccept) *ActivateDedicatedBearerAccept {
	return &ActivateDedicatedBearerAccept{
		ProtocolConfigurationOptions:         nasie.ExtendedPCO(msg.ProtocolConfigurationOptions),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions),
		UnrecognizedIEs:                      utils.RawIEs(msg.Unrecognized),
	}
}

func buildActivateDedicatedBearerReject(msg *eps.ActivateDedicatedEPSBearerContextReject) *ActivateDedicatedBearerReject {
	return &ActivateDedicatedBearerReject{
		ESMCause:                             esmCauseToEnum(msg.Cause),
		ProtocolConfigurationOptions:         nasie.ExtendedPCO(msg.ProtocolConfigurationOptions),
		ExtendedProtocolConfigurationOptions: nasie.ExtendedPCO(msg.ExtendedProtocolConfigurationOptions),
		UnrecognizedIEs:                      utils.RawIEs(msg.Unrecognized),
	}
}

func tft(t eps.TrafficFlowTemplate) *TFT {
	out := &TFT{Operation: utils.NamedEnum(uint8(t.Operation), tftOperationNames[t.Operation]), DeleteIdentifiers: t.DeleteIdentifiers}

	for _, f := range t.Filters {
		pf := TFTPacketFilter{Identifier: f.Identifier, Direction: utils.NamedEnum(uint8(f.Direction), tftDirectionNames[f.Direction]), Precedence: f.Precedence}

		for _, c := range f.Components {
			pf.Components = append(pf.Components, TFTComponent{Type: utils.NamedEnum(uint8(c.Type), tftComponentNames[c.Type]), Value: tftComponentValue(c)})
		}

		out.PacketFilters = append(out.PacketFilters, pf)
	}

	for _, p := range t.Parameters {
		out.Parameters = append(out.Parameters, TFTParameterField{Identifier: p.Identifier, ContentsHex: hex.EncodeToString(p.Contents)})
	}

	return out
}

func tftComponentValue(c eps.TFTComponent) string {
	v := c.Value

	switch c.Type {
	case eps.TFTIPv4RemoteAddress, eps.TFTIPv4LocalAddress:
		return netip.PrefixFrom(netip.AddrFrom4([4]byte(v[:4])), maskBits(v[4:8])).String()
	case eps.TFTIPv6RemoteAddress:
		return netip.PrefixFrom(netip.AddrFrom16([16]byte(v[:16])), maskBits(v[16:32])).String()
	case eps.TFTIPv6RemoteAddressPrefix, eps.TFTIPv6LocalAddressPrefix:
		return netip.PrefixFrom(netip.AddrFrom16([16]byte(v[:16])), int(v[16])).String()
	case eps.TFTProtocolIdentifier:
		return fmt.Sprint(v[0])
	case eps.TFTSingleLocalPort, eps.TFTSingleRemotePort:
		return fmt.Sprint(binary.BigEndian.Uint16(v))
	case eps.TFTLocalPortRange, eps.TFTRemotePortRange:
		return fmt.Sprintf("%d-%d", binary.BigEndian.Uint16(v[:2]), binary.BigEndian.Uint16(v[2:4]))
	case eps.TFTDestinationMACAddress, eps.TFTSourceMACAddress:
		return net.HardwareAddr(v).String()
	default:
		return hex.EncodeToString(v)
	}
}

func maskBits(mask []byte) int {
	bits := 0

	for _, octet := range mask {
		for ; octet&0x80 != 0; octet <<= 1 {
			bits++
		}
	}

	return bits
}
