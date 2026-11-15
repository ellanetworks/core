// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
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
		tais = append(tais, tai(t.TAI))
	}

	var ies []IE

	if m.UEIdentityIndexValue != nil {
		ies = append(ies, ie(s1ap.IDUEIdentityIndexValue, s1ap.CriticalityIgnore, *m.UEIdentityIndexValue))
	}

	if p := m.UEPagingID; p != nil {
		switch {
		case p.STMSI != nil:
			ies = append(ies, ie(s1ap.IDUEPagingID, s1ap.CriticalityIgnore, stmsi(*p.STMSI)))
		case p.IMSI != nil:
			ies = append(ies, ie(s1ap.IDUEPagingID, s1ap.CriticalityIgnore, hex.EncodeToString(*p.IMSI)))
		case p.Ext != nil:
			ies = append(ies, ie(s1ap.IDUEPagingID, s1ap.CriticalityIgnore, rawIEValue{Hex: hex.EncodeToString([]byte(p.Ext.Value))}))
		}
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
		ies = append(ies, ie(s1ap.IDPagingPriority, s1ap.CriticalityIgnore, namedEnum(uint8(pr), pr.Name())))
	}

	if len(m.UERadioCapabilityForPaging) > 0 {
		ies = append(ies, ie(s1ap.IDUERadioCapabilityForPaging, s1ap.CriticalityIgnore, hex.EncodeToString(m.UERadioCapabilityForPaging)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	mtmsi := "?"
	if m.UEPagingID != nil && m.UEPagingID.STMSI != nil {
		mtmsi = strconv.FormatUint(uint64(m.UEPagingID.STMSI.MTMSI), 10)
	}

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Paging (M-TMSI %s, %d TAI)", mtmsi, len(m.TAIList))
}
