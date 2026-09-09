// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildMMEConfigurationUpdate(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseMMEConfigurationUpdate(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse MME Configuration Update: %v", err)}, ""
	}

	var ies []IE

	if m.MMEName != nil {
		ies = append(ies, ie(s1ap.IDMMEname, s1ap.CriticalityIgnore, *m.MMEName))
	}

	if len(m.ServedGUMMEIs) > 0 {
		ies = append(ies, ie(s1ap.IDServedGUMMEIs, s1ap.CriticalityReject, servedGUMMEIs(m.ServedGUMMEIs)))
	}

	if m.RelativeMMECapacity != nil {
		ies = append(ies, ie(s1ap.IDRelativeMMECapacity, s1ap.CriticalityReject, *m.RelativeMMECapacity))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	if m.RelativeMMECapacity == nil {
		return S1APMessageValue{IEs: ies}, "MME Configuration Update"
	}

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("MME Configuration Update (capacity %d)", *m.RelativeMMECapacity)
}

func buildMMEConfigurationUpdateAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseMMEConfigurationUpdateAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse MME Configuration Update Acknowledge: %v", err)}, ""
	}

	var ies []IE

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "MME Configuration Update Acknowledge"
}

func buildMMEConfigurationUpdateFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseMMEConfigurationUpdateFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse MME Configuration Update Failure: %v", err)}, ""
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

	return S1APMessageValue{IEs: ies}, "MME Configuration Update Failure"
}
