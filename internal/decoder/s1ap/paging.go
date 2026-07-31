// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"strconv"

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

	var ies []IE

	if m.UEIdentityIndexValue != nil {
		ies = append(ies, ie(idUEIdentityIndexValue, s1ap.CriticalityReject, *m.UEIdentityIndexValue))
	}

	if m.STMSI != nil {
		ies = append(ies, ie(idSTMSI, s1ap.CriticalityReject, stmsi(*m.STMSI)))
	}

	if m.CNDomain != nil {
		ies = append(ies, ie(idCNDomain, s1ap.CriticalityReject, cnDomainToEnum(*m.CNDomain)))
	}

	ies = append(ies, ie(idTAIList, s1ap.CriticalityReject, tais))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	mtmsi := "?"
	if m.STMSI != nil {
		mtmsi = strconv.FormatUint(uint64(m.STMSI.MTMSI), 10)
	}

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Paging (M-TMSI %s, %d TAI)", mtmsi, len(m.TAIList))
}
