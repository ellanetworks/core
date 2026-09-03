// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// RequestType is what the MME asked the eNB to report and over what area
// (TS 36.413 §9.2.1.35).
type RequestType struct {
	EventType  utils.EnumField `json:"event_type"`
	ReportArea utils.EnumField `json:"report_area"`
}

func requestType(r s1ap.RequestType) RequestType {
	return RequestType{
		EventType:  utils.NamedEnum(uint8(r.EventType), r.EventType.Name()),
		ReportArea: utils.NamedEnum(uint8(r.ReportArea), r.ReportArea.Name()),
	}
}

func buildLocationReport(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseLocationReport(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Location Report: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.TAI != nil {
		ies = append(ies, ie(s1ap.IDTAI, s1ap.CriticalityIgnore, tai(*m.TAI)))
	}

	if m.RequestType != nil {
		ies = append(ies, ie(s1ap.IDRequestType, s1ap.CriticalityIgnore, requestType(*m.RequestType)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Location Report (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}
