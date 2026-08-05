// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

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
		ies = append(ies, ie(idUEPagingIdentity, ngap.CriticalityIgnore,
			UEPagingIdentity{FiveGSTMSI: buildFiveGSTMSI(*m.FiveGSTMSI)}))
	}

	if m.TAIListForPaging != nil {
		tais := make([]TAI, 0, len(m.TAIListForPaging))
		for _, item := range m.TAIListForPaging {
			tais = append(tais, tai(item))
		}

		ies = append(ies, ie(idTAIListForPaging, ngap.CriticalityIgnore, tais))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
