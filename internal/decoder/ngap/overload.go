// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type OverloadStartNSSAIItem struct {
	SliceOverloadList                   []SNSSAI         `json:"slice_overload_list"`
	SliceOverloadResponse               *utils.EnumField `json:"slice_overload_response,omitempty"`
	SliceTrafficLoadReductionIndication *int64           `json:"slice_traffic_load_reduction_indication,omitempty"`
}

func overloadResponse(o ngap.OverloadResponse) utils.EnumField {
	return utils.NamedEnum(uint8(o.OverloadAction), o.OverloadAction.Name())
}

func overloadStartNSSAIList(list ngap.OverloadStartNSSAIList) []OverloadStartNSSAIItem {
	out := make([]OverloadStartNSSAIItem, 0, len(list))

	for _, item := range list {
		slices := make([]SNSSAI, 0, len(item.SliceOverloadList))
		for _, s := range item.SliceOverloadList {
			slices = append(slices, buildSNSSAIValue(s.SNSSAI))
		}

		o := OverloadStartNSSAIItem{SliceOverloadList: slices}

		if item.SliceOverloadResponse != nil {
			r := overloadResponse(*item.SliceOverloadResponse)
			o.SliceOverloadResponse = &r
		}

		if item.SliceTrafficLoadReductionIndication != nil {
			v := int64(*item.SliceTrafficLoadReductionIndication)
			o.SliceTrafficLoadReductionIndication = &v
		}

		out = append(out, o)
	}

	return out
}

func buildOverloadStart(value []byte) NGAPMessageValue {
	m, err := ngap.ParseOverloadStart(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Overload Start: %v", err)}
	}

	var ies []IE

	if m.AMFOverloadResponse != nil {
		ies = append(ies, ie(ngap.IDAMFOverloadResponse, ngap.CriticalityReject, overloadResponse(*m.AMFOverloadResponse)))
	}

	if m.AMFTrafficLoadReductionIndication != nil {
		ies = append(ies, ie(ngap.IDAMFTrafficLoadReductionIndication, ngap.CriticalityIgnore, int64(*m.AMFTrafficLoadReductionIndication)))
	}

	if len(m.OverloadStartNSSAIList) > 0 {
		ies = append(ies, ie(ngap.IDOverloadStartNSSAIList, ngap.CriticalityIgnore, overloadStartNSSAIList(m.OverloadStartNSSAIList)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildOverloadStop(value []byte) NGAPMessageValue {
	m, err := ngap.ParseOverloadStop(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Overload Stop: %v", err)}
	}

	return NGAPMessageValue{IEs: unmodeledIEs(m.UnknownIEs())}
}
