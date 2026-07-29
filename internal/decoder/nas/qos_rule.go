// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type PacketFilterComponent struct {
	ComponentValue []byte          `json:"component_value"`
	ComponentType  utils.EnumField `json:"component_type"`
}

type PacketFilter struct {
	Content    []PacketFilterComponent `json:"content"`
	Direction  utils.EnumField         `json:"direction"`
	Identifier uint8                   `json:"identifier"` // only 0-15
}

type QosRule struct {
	PacketFilterList []PacketFilter  `json:"packet_filter_list"`
	Identifier       uint8           `json:"identifier"`
	OperationCode    uint8           `json:"operation_code"`
	DQR              utils.EnumField `json:"dqr"`
	Segregation      *uint8          `json:"segregation,omitempty"`
	Precedence       *uint8          `json:"precedence,omitempty"`
	QFI              *uint8          `json:"qfi,omitempty"`
}

func dqrToEnum(dqr uint8) utils.EnumField {
	switch dqr & 0x01 {
	case 1:
		return utils.MakeEnum(dqr&0x01, "default", false)
	case 0:
		return utils.MakeEnum(dqr&0x01, "non-default", false)
	default:
		return utils.MakeEnum(dqr&0x01, "", true)
	}
}

func buildPFComponentTypeString(ct uint8) utils.EnumField {
	switch ct {
	case 0x01:
		return utils.MakeEnum(ct, "MatchAll", false)
	case 0x10:
		return utils.MakeEnum(ct, "IPv4RemoteAddress", false)
	case 0x11:
		return utils.MakeEnum(ct, "IPv4LocalAddress", false)
	case 0x21:
		return utils.MakeEnum(ct, "IPv6RemoteAddress", false)
	case 0x23:
		return utils.MakeEnum(ct, "IPv6LocalAddress", false)
	case 0x30:
		return utils.MakeEnum(ct, "ProtocolIdentifierOrNextHeader", false)
	case 0x40:
		return utils.MakeEnum(ct, "SingleLocalPort", false)
	case 0x41:
		return utils.MakeEnum(ct, "LocalPortRange", false)
	case 0x50:
		return utils.MakeEnum(ct, "SingleRemotePort", false)
	case 0x51:
		return utils.MakeEnum(ct, "RemotePortRange", false)
	case 0x60:
		return utils.MakeEnum(ct, "SecurityParameterIndex", false)
	case 0x70:
		return utils.MakeEnum(ct, "TypeOfServiceOrTrafficClass", false)
	case 0x80:
		return utils.MakeEnum(ct, "FlowLabel", false)
	case 0x81:
		return utils.MakeEnum(ct, "DestinationMACAddress", false)
	case 0x82:
		return utils.MakeEnum(ct, "SourceMACAddress", false)
	case 0x87:
		return utils.MakeEnum(ct, "Ethertype", false)
	default:
		return utils.MakeEnum(ct, fmt.Sprintf("Unknown(0x%02X)", ct), true)
	}
}

func buildPFDirectionString(n uint8) utils.EnumField {
	switch n & 0x0F {
	case 0x01:
		return utils.MakeEnum(n&0x0F, "downlink", false)
	case 0x02:
		return utils.MakeEnum(n&0x0F, "uplink", false)
	case 0x03:
		return utils.MakeEnum(n&0x0F, "bidirectional", false)
	default:
		return utils.MakeEnum(n&0x0F, "", true)
	}
}

// QosRulesFromNAS renders decoded authorized QoS rules as the observability
// view.
func QosRulesFromNAS(rules fgs.QoSRules) []QosRule {
	out := make([]QosRule, 0, len(rules))

	for _, r := range rules {
		qr := QosRule{
			Identifier:    r.Identifier,
			OperationCode: uint8(r.OperationCode),
			DQR:           dqrToEnum(r.DQR),
		}

		if p := r.Parameters; p != nil {
			qr.Segregation, qr.Precedence, qr.QFI = &p.Segregation, &p.Precedence, &p.QFI
		}

		for _, f := range r.Filters {
			pf := PacketFilter{
				Direction:  buildPFDirectionString(uint8(f.Direction)),
				Identifier: f.Identifier,
			}

			for _, c := range f.Components {
				pf.Content = append(pf.Content, PacketFilterComponent{
					ComponentType:  buildPFComponentTypeString(uint8(c.Type)),
					ComponentValue: c.Value,
				})
			}

			qr.PacketFilterList = append(qr.PacketFilterList, pf)
		}

		out = append(out, qr)
	}

	return out
}
