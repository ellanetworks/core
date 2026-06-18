// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildPaging(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParsePaging(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Paging: %v", err)}, ""
	}

	tais := make([]TAI, 0, len(m.TAIList))
	for _, t := range m.TAIList {
		tais = append(tais, tai(t))
	}

	ies := []IE{
		ie(idUEIdentityIndexValue, s1ap.CriticalityReject, m.UEIdentityIndexValue),
		ie(idSTMSI, s1ap.CriticalityReject, stmsi(m.STMSI)),
		ie(idCNDomain, s1ap.CriticalityReject, cnDomainToEnum(m.CNDomain)),
		ie(idTAIList, s1ap.CriticalityReject, tais),
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Paging (M-TMSI %d, %d TAI)", m.STMSI.MTMSI, len(m.TAIList))
}
