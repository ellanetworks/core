// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildENBConfigurationUpdate(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseENBConfigurationUpdate(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse eNB Configuration Update: %v", err)}, ""
	}

	var ies []IE

	name := ""
	if m.ENBName != nil {
		name = *m.ENBName
		ies = append(ies, ie(s1ap.IDENBname, s1ap.CriticalityIgnore, name))
	}

	if len(m.SupportedTAs) > 0 {
		ies = append(ies, ie(s1ap.IDSupportedTAs, s1ap.CriticalityReject, supportedTAs(m.SupportedTAs)))
	}

	if m.DefaultPagingDRX != nil {
		ies = append(ies, ie(s1ap.IDDefaultPagingDRX, s1ap.CriticalityIgnore, pagingDRXToEnum(*m.DefaultPagingDRX)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	if name == "" {
		return S1APMessageValue{IEs: ies}, "eNB Configuration Update"
	}

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("eNB Configuration Update (%s)", name)
}

func buildENBConfigurationUpdateAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseENBConfigurationUpdateAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse eNB Configuration Update Acknowledge: %v", err)}, ""
	}

	var ies []IE

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "eNB Configuration Update Acknowledge"
}

func buildENBConfigurationUpdateFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseENBConfigurationUpdateFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse eNB Configuration Update Failure: %v", err)}, ""
	}

	var ies []IE

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.TimeToWait != nil {
		ies = append(ies, ie(s1ap.IDTimeToWait, s1ap.CriticalityIgnore, timeToWaitToEnum(*m.TimeToWait)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "eNB Configuration Update Failure"
}
