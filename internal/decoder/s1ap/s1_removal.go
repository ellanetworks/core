// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildS1RemovalRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseS1RemovalRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Removal Request: %v", err)}, ""
	}

	ies := []IE{ie(s1ap.IDGlobalENBID, s1ap.CriticalityReject, globalENBID(m.GlobalENBID))}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "S1 Removal Request"
}

func buildS1RemovalResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseS1RemovalResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Removal Response: %v", err)}, ""
	}

	var ies []IE

	if m.MMEName != nil {
		ies = append(ies, ie(s1ap.IDMMEname, s1ap.CriticalityIgnore, *m.MMEName))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "S1 Removal Response"
}

func buildS1RemovalFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseS1RemovalFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse S1 Removal Failure: %v", err)}, ""
	}

	var ies []IE

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, "S1 Removal Failure"
}
