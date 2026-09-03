// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

// ERABToBeReleasedItem is a decoded E-RAB the MME asks the eNB to release, with
// the cause it releases it for (TS 36.413 §9.2.1.36).
type ERABToBeReleasedItem struct {
	ERABID uint8 `json:"erab_id"`
	Cause  Cause `json:"cause"`
}

// ERABReleasedItem is a decoded E-RAB the eNB confirms released
// (TS 36.413 §9.2.1.36).
type ERABReleasedItem struct {
	ERABID uint8 `json:"erab_id"`
}

func buildERABReleaseCommand(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABReleaseCommand(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Release Command: %v", err)}, ""
	}

	released := make([]ERABToBeReleasedItem, 0, len(m.ERABToBeReleased))
	for _, it := range m.ERABToBeReleased {
		released = append(released, ERABToBeReleasedItem{ERABID: uint8(it.ERABID), Cause: cause(it.Cause)})
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if ambr := m.UEAggregateMaximumBitRate; ambr != nil {
		ies = append(ies, ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityReject, AMBR{DL: uint64(ambr.DL), UL: uint64(ambr.UL)}))
	}

	ies = append(ies, ie(s1ap.IDERABToBeReleasedList, s1ap.CriticalityIgnore, released))

	if len(m.NASPDU) > 0 {
		ies = append(ies, ie(s1ap.IDNASPDU, s1ap.CriticalityIgnore, nasPDU(m.NASPDU)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Release Command (MME-UE %d, eNB-UE %d, %d E-RAB)", m.MMEUES1APID, m.ENBUES1APID, len(m.ERABToBeReleased))
}

func buildERABReleaseResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABReleaseResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Release Response: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if len(m.ERABReleased) > 0 {
		released := make([]ERABReleasedItem, 0, len(m.ERABReleased))
		for _, it := range m.ERABReleased {
			released = append(released, ERABReleasedItem{ERABID: uint8(it.ERABID)})
		}

		ies = append(ies, ie(s1ap.IDERABReleaseListBearerRelComp, s1ap.CriticalityIgnore, released))
	}

	if len(m.ERABFailedToRelease) > 0 {
		ies = append(ies, ie(s1ap.IDERABFailedToReleaseList, s1ap.CriticalityIgnore, erabFailedItems(m.ERABFailedToRelease)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(s1ap.IDUserLocationInformation, s1ap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Release Response (MME-UE %s, eNB-UE %s, %d E-RAB)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID), len(m.ERABReleased))
}
