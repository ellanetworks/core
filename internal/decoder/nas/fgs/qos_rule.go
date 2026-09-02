// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
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
	return utils.NamedEnum(ct, fgs.PacketFilterComponentType(ct).Name())
}

func buildPFDirectionString(n uint8) utils.EnumField {
	d := fgs.PacketFilterDirection(n & 0x0F)

	return utils.NamedEnum(uint8(d), d.Name())
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
