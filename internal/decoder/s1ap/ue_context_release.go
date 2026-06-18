// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

func buildUEContextReleaseRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUEContextReleaseRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Context Release Request: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Context Release Request (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildUEContextReleaseCommand(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUEContextReleaseCommand(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Context Release Command: %v", err)}, ""
	}

	ies := []IE{
		ie(idUES1APIDs, s1ap.CriticalityReject, ues1apIDs(m.UES1APIDs)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Context Release Command (MME-UE %d, eNB-UE %d)", m.UES1APIDs.MMEUES1APID, m.UES1APIDs.ENBUES1APID)
}

func buildUEContextReleaseComplete(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseUEContextReleaseComplete(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse UE Context Release Complete: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityIgnore, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityIgnore, uint32(m.ENBUES1APID)),
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(idCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("UE Context Release Complete (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}
