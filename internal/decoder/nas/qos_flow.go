// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

func qfdOpcodeToEnum(op fgs.QoSFlowOperation) utils.EnumField {
	switch op {
	case fgs.QoSFlowOpCreate:
		return utils.MakeEnum(uint8(op), "Create", false)
	case fgs.QoSFlowOpDelete:
		return utils.MakeEnum(uint8(op), "Delete", false)
	case fgs.QoSFlowOpModify:
		return utils.MakeEnum(uint8(op), "Modify", false)
	default:
		return utils.MakeEnum(uint8(op), "", true)
	}
}

func qfdParamIDToEnum(id fgs.QoSFlowParameterID) utils.EnumField {
	switch id {
	case fgs.QoSFlowParam5QI:
		return utils.MakeEnum(uint8(id), "5QI", false)
	case fgs.QoSFlowParamGFBRUplink:
		return utils.MakeEnum(uint8(id), "GFBR UL", false)
	case fgs.QoSFlowParamGFBRDownlink:
		return utils.MakeEnum(uint8(id), "GFBR DL", false)
	case fgs.QoSFlowParamMFBRUplink:
		return utils.MakeEnum(uint8(id), "MFBR UL", false)
	case fgs.QoSFlowParamMFBRDownlink:
		return utils.MakeEnum(uint8(id), "MFBR DL", false)
	case fgs.QoSFlowParamAveragingWindow:
		return utils.MakeEnum(uint8(id), "Averaging Window", false)
	case fgs.QoSFlowParamEPSBearerID:
		return utils.MakeEnum(uint8(id), "EPS Bearer ID", false)
	default:
		return utils.MakeEnum(uint8(id), fmt.Sprintf("Unknown(0x%02X)", uint8(id)), true)
	}
}

type QosFlowParameter struct {
	ParamID  utils.EnumField `json:"identifier"`
	ParamLen uint8           `json:"length"`

	FiveQI      *uint8  `json:"five_qi,omitempty"`
	GfbrUlKbps  *uint64 `json:"gfbr_ul_kbps,omitempty"`
	GfbrDlKbps  *uint64 `json:"gfbr_dl_kbps,omitempty"`
	MfbrUlKbps  *uint64 `json:"mfbr_ul_kbps,omitempty"`
	MfbrDlKbps  *uint64 `json:"mfbr_dl_kbps,omitempty"`
	AvgWindowMs *uint16 `json:"averaging_window_ms,omitempty"`
	EpsBearerID *uint8  `json:"eps_bearer_id,omitempty"`
}

type QoSFlowDescription struct {
	ParamList []QosFlowParameter `json:"param_list"`
	Qfi       uint8              `json:"qfi"`
	OpCode    utils.EnumField    `json:"op_code"`
	EBit      bool               `json:"e_bit"`
}

// QosFlowDescriptionsFromNAS renders decoded authorized QoS flow descriptions as
// the observability view.
func QosFlowDescriptionsFromNAS(descs fgs.QoSFlowDescriptions) []QoSFlowDescription {
	out := make([]QoSFlowDescription, 0, len(descs))

	for _, d := range descs {
		desc := QoSFlowDescription{
			Qfi:    d.QFI,
			OpCode: qfdOpcodeToEnum(d.OperationCode),
			EBit:   d.EBit,
		}

		for _, p := range d.Parameters {
			desc.ParamList = append(desc.ParamList, qosFlowParameter(p))
		}

		out = append(out, desc)
	}

	return out
}

func qosFlowParameter(p fgs.QoSFlowParameter) QosFlowParameter {
	param := QosFlowParameter{
		ParamID:  qfdParamIDToEnum(p.ID),
		ParamLen: uint8(len(p.Value)),
	}

	switch p.ID {
	case fgs.QoSFlowParam5QI:
		if len(p.Value) == 1 {
			v := p.Value[0]
			param.FiveQI = &v
		}
	case fgs.QoSFlowParamGFBRUplink, fgs.QoSFlowParamGFBRDownlink,
		fgs.QoSFlowParamMFBRUplink, fgs.QoSFlowParamMFBRDownlink:
		if kbps, ok := p.Kbps(); ok {
			switch p.ID {
			case fgs.QoSFlowParamGFBRUplink:
				param.GfbrUlKbps = &kbps
			case fgs.QoSFlowParamGFBRDownlink:
				param.GfbrDlKbps = &kbps
			case fgs.QoSFlowParamMFBRUplink:
				param.MfbrUlKbps = &kbps
			case fgs.QoSFlowParamMFBRDownlink:
				param.MfbrDlKbps = &kbps
			}
		}
	case fgs.QoSFlowParamAveragingWindow:
		if len(p.Value) == 2 {
			ms := binary.BigEndian.Uint16(p.Value)
			param.AvgWindowMs = &ms
		}
	case fgs.QoSFlowParamEPSBearerID:
		if len(p.Value) == 1 {
			ebi := p.Value[0]
			param.EpsBearerID = &ebi
		}
	}

	return param
}
