// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/binary"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

func qfdOpcodeToEnum(op fgs.QoSFlowOperation) utils.EnumField {
	return utils.NamedEnum(uint8(op), op.Name())
}

func qfdParamIDToEnum(id fgs.QoSFlowParameterID) utils.EnumField {
	return utils.NamedEnum(uint8(id), id.Name())
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
