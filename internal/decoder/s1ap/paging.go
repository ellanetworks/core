// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/ellanetworks/core/internal/decoder/utils"
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
		ies = append(ies, ie(s1ap.IDUEIdentityIndexValue, s1ap.CriticalityIgnore, *m.UEIdentityIndexValue))
	}

	if m.STMSI != nil {
		ies = append(ies, ie(s1ap.IDUEPagingID, s1ap.CriticalityIgnore, stmsi(*m.STMSI)))
	}

	if m.PagingDRX != nil {
		ies = append(ies, ie(s1ap.IDPagingDRX, s1ap.CriticalityIgnore, pagingDRXToEnum(*m.PagingDRX)))
	}

	if m.CNDomain != nil {
		ies = append(ies, ie(s1ap.IDCNDomain, s1ap.CriticalityIgnore, cnDomainToEnum(*m.CNDomain)))
	}

	ies = append(ies, ie(s1ap.IDTAIList, s1ap.CriticalityIgnore, tais))

	if m.PagingPriority != nil {
		pr := *m.PagingPriority
		ies = append(ies, ie(s1ap.IDPagingPriority, s1ap.CriticalityIgnore, utils.NamedEnum(uint8(pr), pr.Name())))
	}

	if len(m.UERadioCapabilityForPaging) > 0 {
		ies = append(ies, ie(s1ap.IDUERadioCapabilityForPaging, s1ap.CriticalityIgnore, hex.EncodeToString(m.UERadioCapabilityForPaging)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	mtmsi := "?"
	if m.STMSI != nil {
		mtmsi = strconv.FormatUint(uint64(m.STMSI.MTMSI), 10)
	}

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Paging (M-TMSI %s, %d TAI)", mtmsi, len(m.TAIList))
}
