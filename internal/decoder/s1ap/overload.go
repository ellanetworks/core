// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

type OverloadResponse struct {
	OverloadAction uint8 `json:"overload_action"`
}

func gummeiList(list []s1ap.GUMMEI) []GUMMEI {
	out := make([]GUMMEI, 0, len(list))
	for _, g := range list {
		out = append(out, gummei(g))
	}

	return out
}

func buildOverloadStart(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseOverloadStart(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Overload Start: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDOverloadResponse, s1ap.CriticalityReject, OverloadResponse{OverloadAction: uint8(m.OverloadResponse.OverloadAction)}),
	}

	if len(m.GUMMEIList) > 0 {
		ies = append(ies, ie(s1ap.IDGUMMEIList, s1ap.CriticalityIgnore, gummeiList(m.GUMMEIList)))
	}

	if m.TrafficLoadReductionIndication != nil {
		ies = append(ies, ie(s1ap.IDTrafficLoadReductionIndication, s1ap.CriticalityIgnore, *m.TrafficLoadReductionIndication))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Overload Start (action %d)", uint8(m.OverloadResponse.OverloadAction))
}

func buildOverloadStop(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseOverloadStop(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Overload Stop: %v", err)}, ""
	}

	var ies []IE

	if len(m.GUMMEIList) > 0 {
		ies = append(ies, ie(s1ap.IDGUMMEIList, s1ap.CriticalityIgnore, gummeiList(m.GUMMEIList)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "Overload Stop"
}
