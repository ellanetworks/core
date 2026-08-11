// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

// MappedEPSBearerContext is the observability view of one mapped EPS bearer
// context (TS 24.501 §9.11.4.8): the EPS bearer a PDU session's QoS flows map to
// on a handover to EPS.
type MappedEPSBearerContext struct {
	EPSBearerID uint8           `json:"eps_bearer_id"`
	OpCode      utils.EnumField `json:"op_code"`
	EBit        bool            `json:"e_bit"`
	ParamList   []EPSParameter  `json:"param_list,omitempty"`
}

// EPSParameter is one entry of a mapped EPS bearer context's parameters list.
// The contents are rendered as hex: each identifier is coded by a different EPS
// clause (TS 24.301 §9.9.4.2, §9.9.4.3, TS 24.008 figure 10.5.144), and the
// observability view has no reason to re-implement those.
type EPSParameter struct {
	ParamID  utils.EnumField `json:"identifier"`
	ParamLen uint8           `json:"length"`
	Contents string          `json:"contents"`
}

func mappedEPSOpcodeToEnum(op fgs.MappedEPSBearerOperation) utils.EnumField {
	switch op {
	case fgs.MappedEPSBearerOpCreate:
		return utils.MakeEnum(uint8(op), "Create", false)
	case fgs.MappedEPSBearerOpDelete:
		return utils.MakeEnum(uint8(op), "Delete", false)
	case fgs.MappedEPSBearerOpModify:
		return utils.MakeEnum(uint8(op), "Modify", false)
	default:
		return utils.MakeEnum(uint8(op), "", true)
	}
}

func epsParameterIDToEnum(id fgs.EPSParameterIdentifier) utils.EnumField {
	switch id {
	case fgs.EPSParameterMappedEPSQoS:
		return utils.MakeEnum(uint8(id), "Mapped EPS QoS", false)
	case fgs.EPSParameterMappedExtendedEPSQoS:
		return utils.MakeEnum(uint8(id), "Mapped Extended EPS QoS", false)
	case fgs.EPSParameterTrafficFlowTemplate:
		return utils.MakeEnum(uint8(id), "TFT", false)
	case fgs.EPSParameterAPNAMBR:
		return utils.MakeEnum(uint8(id), "APN-AMBR", false)
	case fgs.EPSParameterExtendedAPNAMBR:
		return utils.MakeEnum(uint8(id), "Extended APN-AMBR", false)
	default:
		return utils.MakeEnum(uint8(id), fmt.Sprintf("Unknown(0x%02X)", uint8(id)), true)
	}
}

// MappedEPSBearerContextsFromNAS renders decoded mapped EPS bearer contexts as
// the observability view.
func MappedEPSBearerContextsFromNAS(contexts fgs.MappedEPSBearerContexts) []MappedEPSBearerContext {
	out := make([]MappedEPSBearerContext, 0, len(contexts))

	for _, c := range contexts {
		ctx := MappedEPSBearerContext{
			EPSBearerID: c.EPSBearerIdentity,
			OpCode:      mappedEPSOpcodeToEnum(c.Operation),
			EBit:        c.EBit,
		}

		for _, p := range c.Parameters {
			ctx.ParamList = append(ctx.ParamList, EPSParameter{
				ParamID:  epsParameterIDToEnum(p.Identifier),
				ParamLen: uint8(len(p.Contents)),
				Contents: strings.ToUpper(hex.EncodeToString(p.Contents)),
			})
		}

		out = append(out, ctx)
	}

	return out
}
