// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type UEPagingIdentity struct {
	FiveGSTMSI FiveGSTMSI `json:"five_gs_tmsi"`
}

// Paging asks the NG-RAN node to page a UE in the tracking areas listed
// (TS 38.413 §9.2.4.1).
func buildPaging(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePaging(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Paging: %v", err)}
	}

	var ies []IE

	if m.FiveGSTMSI != nil {
		ies = append(ies, ie(ngap.IDUEPagingIdentity, ngap.CriticalityIgnore,
			UEPagingIdentity{FiveGSTMSI: buildFiveGSTMSI(*m.FiveGSTMSI)}))
	}

	if m.PagingDRX != nil {
		ies = append(ies, ie(ngap.IDPagingDRX, ngap.CriticalityIgnore, buildPagingDRX(*m.PagingDRX)))
	}

	if m.TAIListForPaging != nil {
		tais := make([]TAI, 0, len(m.TAIListForPaging))
		for _, item := range m.TAIListForPaging {
			tais = append(tais, tai(item))
		}

		ies = append(ies, ie(ngap.IDTAIListForPaging, ngap.CriticalityIgnore, tais))
	}

	if m.PagingPriority != nil {
		p := *m.PagingPriority
		ies = append(ies, ie(ngap.IDPagingPriority, ngap.CriticalityIgnore, utils.NamedEnum(uint8(p), p.Name())))
	}

	if m.UERadioCapabilityForPaging != nil {
		ies = append(ies, ie(ngap.IDUERadioCapabilityForPaging, ngap.CriticalityIgnore,
			ueRadioCapabilityForPaging(*m.UERadioCapabilityForPaging)))
	}

	if m.PagingOrigin != nil {
		o := *m.PagingOrigin
		ies = append(ies, ie(ngap.IDPagingOrigin, ngap.CriticalityIgnore, utils.NamedEnum(uint8(o), o.Name())))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// UERadioCapabilityForPaging is the decoded UE Radio Capability for Paging IE
// (TS 38.413 §9.3.1.68): the paging-specific capabilities, per access.
type UERadioCapabilityForPaging struct {
	NR    string `json:"nr,omitempty"`
	EUTRA string `json:"eutra,omitempty"`
}

func ueRadioCapabilityForPaging(c ngap.UERadioCapabilityForPaging) UERadioCapabilityForPaging {
	var out UERadioCapabilityForPaging

	if c.NR != nil {
		out.NR = hex.EncodeToString(*c.NR)
	}

	if c.EUTRA != nil {
		out.EUTRA = hex.EncodeToString(*c.EUTRA)
	}

	return out
}
