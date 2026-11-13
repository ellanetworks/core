// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

type WarningAreaList struct {
	ECGIs            []EUTRANCGI `json:"ecgi_list,omitempty"`
	TAIs             []TAI       `json:"tai_list,omitempty"`
	EmergencyAreaIDs []string    `json:"emergency_area_id_list,omitempty"`
}

type TAIBroadcast struct {
	TAI            TAI         `json:"tai"`
	CompletedCells []EUTRANCGI `json:"completed_cells"`
}

type EmergencyAreaIDBroadcast struct {
	EmergencyAreaID string      `json:"emergency_area_id"`
	CompletedCells  []EUTRANCGI `json:"completed_cells"`
}

type BroadcastCompletedAreaList struct {
	CellIDs          []EUTRANCGI                `json:"cell_id_broadcast,omitempty"`
	TAIs             []TAIBroadcast             `json:"tai_broadcast,omitempty"`
	EmergencyAreaIDs []EmergencyAreaIDBroadcast `json:"emergency_area_id_broadcast,omitempty"`
}

type CancelledCell struct {
	ECGI               EUTRANCGI `json:"ecgi"`
	NumberOfBroadcasts uint16    `json:"number_of_broadcasts"`
}

type TAICancelled struct {
	TAI            TAI             `json:"tai"`
	CancelledCells []CancelledCell `json:"cancelled_cells"`
}

type EmergencyAreaIDCancelled struct {
	EmergencyAreaID string          `json:"emergency_area_id"`
	CancelledCells  []CancelledCell `json:"cancelled_cells"`
}

type BroadcastCancelledAreaList struct {
	CellIDs          []CancelledCell            `json:"cell_id_cancelled,omitempty"`
	TAIs             []TAICancelled             `json:"tai_cancelled,omitempty"`
	EmergencyAreaIDs []EmergencyAreaIDCancelled `json:"emergency_area_id_cancelled,omitempty"`
}

func emergencyAreaID(id s1ap.EmergencyAreaID) string { return hex.EncodeToString(id[:]) }

func emergencyAreaIDs(ids []s1ap.EmergencyAreaID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, emergencyAreaID(id))
	}

	return out
}

func ecgis(cells []s1ap.EUTRANCGI) []EUTRANCGI {
	out := make([]EUTRANCGI, 0, len(cells))
	for _, c := range cells {
		out = append(out, eutranCGI(c))
	}

	return out
}

func tais(list []s1ap.TAI) []TAI {
	out := make([]TAI, 0, len(list))
	for _, t := range list {
		out = append(out, tai(t))
	}

	return out
}

func warningAreaList(l s1ap.WarningAreaList) WarningAreaList {
	return WarningAreaList{ECGIs: ecgis(l.CellIDList), TAIs: tais(l.TAIList), EmergencyAreaIDs: emergencyAreaIDs(l.EmergencyAreaIDList)}
}

func broadcastCompletedAreaList(l s1ap.BroadcastCompletedAreaList) BroadcastCompletedAreaList {
	var out BroadcastCompletedAreaList

	for _, c := range l.CellIDBroadcast {
		out.CellIDs = append(out.CellIDs, eutranCGI(c.ECGI))
	}

	for _, t := range l.TAIBroadcast {
		b := TAIBroadcast{TAI: tai(t.TAI)}
		for _, c := range t.CompletedCellInTAI {
			b.CompletedCells = append(b.CompletedCells, eutranCGI(c.ECGI))
		}

		out.TAIs = append(out.TAIs, b)
	}

	for _, e := range l.EmergencyAreaIDBroadcast {
		b := EmergencyAreaIDBroadcast{EmergencyAreaID: emergencyAreaID(e.EmergencyAreaID)}
		for _, c := range e.CompletedCellInEAI {
			b.CompletedCells = append(b.CompletedCells, eutranCGI(c.ECGI))
		}

		out.EmergencyAreaIDs = append(out.EmergencyAreaIDs, b)
	}

	return out
}

func broadcastCancelledAreaList(l s1ap.BroadcastCancelledAreaList) BroadcastCancelledAreaList {
	var out BroadcastCancelledAreaList

	for _, c := range l.CellIDCancelled {
		out.CellIDs = append(out.CellIDs, CancelledCell{ECGI: eutranCGI(c.ECGI), NumberOfBroadcasts: c.NumberOfBroadcasts})
	}

	for _, t := range l.TAICancelled {
		b := TAICancelled{TAI: tai(t.TAI)}
		for _, c := range t.CancelledCellInTAI {
			b.CancelledCells = append(b.CancelledCells, CancelledCell{ECGI: eutranCGI(c.ECGI), NumberOfBroadcasts: c.NumberOfBroadcasts})
		}

		out.TAIs = append(out.TAIs, b)
	}

	for _, e := range l.EmergencyAreaIDCancelled {
		b := EmergencyAreaIDCancelled{EmergencyAreaID: emergencyAreaID(e.EmergencyAreaID)}
		for _, c := range e.CancelledCellInEAI {
			b.CancelledCells = append(b.CancelledCells, CancelledCell{ECGI: eutranCGI(c.ECGI), NumberOfBroadcasts: c.NumberOfBroadcasts})
		}

		out.EmergencyAreaIDs = append(out.EmergencyAreaIDs, b)
	}

	return out
}

func warningSummary(name string, id s1ap.MessageIdentifier, sn s1ap.SerialNumber) string {
	return fmt.Sprintf("%s (message %d, serial %d)", name, uint16(id), uint16(sn))
}

func buildWriteReplaceWarningRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseWriteReplaceWarningRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Write-Replace Warning Request: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMessageIdentifier, s1ap.CriticalityReject, uint16(m.MessageIdentifier)),
		ie(s1ap.IDSerialNumber, s1ap.CriticalityReject, uint16(m.SerialNumber)),
	}

	if m.WarningAreaList != nil {
		ies = append(ies, ie(s1ap.IDWarningAreaList, s1ap.CriticalityIgnore, warningAreaList(*m.WarningAreaList)))
	}

	ies = append(ies, ie(s1ap.IDRepetitionPeriod, s1ap.CriticalityReject, m.RepetitionPeriod))

	if m.ExtendedRepetitionPeriod != nil {
		ies = append(ies, ie(s1ap.IDExtendedRepetitionPeriod, s1ap.CriticalityReject, *m.ExtendedRepetitionPeriod))
	}

	ies = append(ies, ie(s1ap.IDNumberofBroadcastRequest, s1ap.CriticalityReject, m.NumberofBroadcastRequest))

	if m.WarningType != nil {
		ies = append(ies, ie(s1ap.IDWarningType, s1ap.CriticalityIgnore, rawIEValue{Hex: hex.EncodeToString(m.WarningType[:])}))
	}

	if m.WarningSecurityInfo != nil {
		ies = append(ies, ie(s1ap.IDWarningSecurityInfo, s1ap.CriticalityIgnore, rawIEValue{Hex: hex.EncodeToString(m.WarningSecurityInfo[:])}))
	}

	if m.DataCodingScheme != nil {
		ies = append(ies, ie(s1ap.IDDataCodingScheme, s1ap.CriticalityIgnore, uint8(*m.DataCodingScheme)))
	}

	if m.WarningMessageContents != nil {
		ies = append(ies, ie(s1ap.IDWarningMessageContents, s1ap.CriticalityIgnore, rawIEValue{Hex: hex.EncodeToString(*m.WarningMessageContents)}))
	}

	if m.ConcurrentWarningMessageIndicator != nil {
		ies = append(ies, ie(s1ap.IDConcurrentWarningMessageIndicator, s1ap.CriticalityReject, *m.ConcurrentWarningMessageIndicator == s1ap.ConcurrentWarningMessageIndicatorTrue))
	}

	if m.WarningAreaCoordinates != nil {
		ies = append(ies, ie(s1ap.IDWarningAreaCoordinates, s1ap.CriticalityIgnore, rawIEValue{Hex: hex.EncodeToString(*m.WarningAreaCoordinates)}))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, warningSummary("Write-Replace Warning Request", m.MessageIdentifier, m.SerialNumber)
}

func buildWriteReplaceWarningResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseWriteReplaceWarningResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Write-Replace Warning Response: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMessageIdentifier, s1ap.CriticalityReject, uint16(m.MessageIdentifier)),
		ie(s1ap.IDSerialNumber, s1ap.CriticalityReject, uint16(m.SerialNumber)),
	}

	if m.BroadcastCompletedAreaList != nil {
		ies = append(ies, ie(s1ap.IDBroadcastCompletedAreaList, s1ap.CriticalityIgnore, broadcastCompletedAreaList(*m.BroadcastCompletedAreaList)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, warningSummary("Write-Replace Warning Response", m.MessageIdentifier, m.SerialNumber)
}

func buildKillRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseKillRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Kill Request: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMessageIdentifier, s1ap.CriticalityReject, uint16(m.MessageIdentifier)),
		ie(s1ap.IDSerialNumber, s1ap.CriticalityReject, uint16(m.SerialNumber)),
	}

	if m.WarningAreaList != nil {
		ies = append(ies, ie(s1ap.IDWarningAreaList, s1ap.CriticalityIgnore, warningAreaList(*m.WarningAreaList)))
	}

	if m.KillAllWarningMessages != nil {
		ies = append(ies, ie(s1ap.IDKillAllWarningMessages, s1ap.CriticalityReject, *m.KillAllWarningMessages == s1ap.KillAllWarningMessagesTrue))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, warningSummary("Kill Request", m.MessageIdentifier, m.SerialNumber)
}

func buildKillResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseKillResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Kill Response: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMessageIdentifier, s1ap.CriticalityReject, uint16(m.MessageIdentifier)),
		ie(s1ap.IDSerialNumber, s1ap.CriticalityReject, uint16(m.SerialNumber)),
	}

	if m.BroadcastCancelledAreaList != nil {
		ies = append(ies, ie(s1ap.IDBroadcastCancelledAreaList, s1ap.CriticalityIgnore, broadcastCancelledAreaList(*m.BroadcastCancelledAreaList)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, warningSummary("Kill Response", m.MessageIdentifier, m.SerialNumber)
}

func buildPWSRestartIndication(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParsePWSRestartIndication(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse PWS Restart Indication: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDECGIListForRestart, s1ap.CriticalityReject, ecgis(m.ECGIListForRestart)),
		ie(s1ap.IDGlobalENBID, s1ap.CriticalityReject, globalENBID(m.GlobalENBID)),
		ie(s1ap.IDTAIListForRestart, s1ap.CriticalityReject, tais(m.TAIListForRestart)),
	}

	if len(m.EmergencyAreaIDListForRestart) > 0 {
		ies = append(ies, ie(s1ap.IDEmergencyAreaIDListForRestart, s1ap.CriticalityReject, emergencyAreaIDs(m.EmergencyAreaIDListForRestart)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("PWS Restart Indication (%d cell(s))", len(m.ECGIListForRestart))
}

func buildPWSFailureIndication(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParsePWSFailureIndication(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse PWS Failure Indication: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDPWSfailedECGIList, s1ap.CriticalityReject, ecgis(m.PWSFailedECGIList)),
		ie(s1ap.IDGlobalENBID, s1ap.CriticalityReject, globalENBID(m.GlobalENBID)),
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("PWS Failure Indication (%d cell(s))", len(m.PWSFailedECGIList))
}
