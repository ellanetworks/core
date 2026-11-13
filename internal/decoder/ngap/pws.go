// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type CellIDList struct {
	EUTRACGIs []EUTRACGI `json:"eutra_cgi_list,omitempty"`
	NRCGIs    []NRCGI    `json:"nr_cgi_list,omitempty"`
}

type WarningAreaList struct {
	CellIDList
	TAIs             []TAI    `json:"tai_list,omitempty"`
	EmergencyAreaIDs []string `json:"emergency_area_id_list,omitempty"`
}

type BroadcastCell struct {
	EUTRACGI           *EUTRACGI `json:"eutra_cgi,omitempty"`
	NRCGI              *NRCGI    `json:"nr_cgi,omitempty"`
	NumberOfBroadcasts *int64    `json:"number_of_broadcasts,omitempty"`
}

type BroadcastTAI struct {
	TAI   TAI             `json:"tai"`
	Cells []BroadcastCell `json:"cells"`
}

type BroadcastEmergencyArea struct {
	EmergencyAreaID string          `json:"emergency_area_id"`
	Cells           []BroadcastCell `json:"cells"`
}

type BroadcastAreaList struct {
	CellIDEUTRA          []BroadcastCell          `json:"cell_id_eutra,omitempty"`
	TAIEUTRA             []BroadcastTAI           `json:"tai_eutra,omitempty"`
	EmergencyAreaIDEUTRA []BroadcastEmergencyArea `json:"emergency_area_id_eutra,omitempty"`
	CellIDNR             []BroadcastCell          `json:"cell_id_nr,omitempty"`
	TAINR                []BroadcastTAI           `json:"tai_nr,omitempty"`
	EmergencyAreaIDNR    []BroadcastEmergencyArea `json:"emergency_area_id_nr,omitempty"`
}

func eutraCGI(c ngap.EUTRACGI) EUTRACGI {
	return EUTRACGI{
		PLMNID:            plmnIDToDecoder(c.PLMNIdentity),
		EUTRACellIdentity: bitsHex(uint64(c.EUTRACellIdentity), ngap.EUTRACellIdentityBits),
	}
}

func nrCGI(c ngap.NRCGI) NRCGI {
	return NRCGI{
		PLMNID:         plmnIDToDecoder(c.PLMNIdentity),
		NRCellIdentity: bitsHex(uint64(c.NRCellIdentity), ngap.NRCellIdentityBits),
	}
}

func cellIDList(eutra []ngap.EUTRACGI, nr []ngap.NRCGI) CellIDList {
	var out CellIDList

	for _, c := range eutra {
		out.EUTRACGIs = append(out.EUTRACGIs, eutraCGI(c))
	}

	for _, c := range nr {
		out.NRCGIs = append(out.NRCGIs, nrCGI(c))
	}

	return out
}

func taiList(list []ngap.TAI) []TAI {
	out := make([]TAI, 0, len(list))
	for _, t := range list {
		out = append(out, tai(t))
	}

	return out
}

func emergencyAreaIDs(list []ngap.EmergencyAreaID) []string {
	out := make([]string, 0, len(list))
	for _, e := range list {
		out = append(out, hex.EncodeToString(e[:]))
	}

	return out
}

func warningAreaList(l ngap.WarningAreaList) WarningAreaList {
	out := WarningAreaList{CellIDList: cellIDList(l.EUTRACGIs, l.NRCGIs)}

	if len(l.TAIs) > 0 {
		out.TAIs = taiList(l.TAIs)
	}

	if len(l.EmergencyAreaIDs) > 0 {
		out.EmergencyAreaIDs = emergencyAreaIDs(l.EmergencyAreaIDs)
	}

	return out
}

func eutraCell(c ngap.EUTRACGI, n *uint16) BroadcastCell {
	cgi := eutraCGI(c)
	out := BroadcastCell{EUTRACGI: &cgi}

	if n != nil {
		v := int64(*n)
		out.NumberOfBroadcasts = &v
	}

	return out
}

func nrCell(c ngap.NRCGI, n *uint16) BroadcastCell {
	cgi := nrCGI(c)
	out := BroadcastCell{NRCGI: &cgi}

	if n != nil {
		v := int64(*n)
		out.NumberOfBroadcasts = &v
	}

	return out
}

func broadcastCompletedAreaList(l ngap.BroadcastCompletedAreaList) BroadcastAreaList {
	var out BroadcastAreaList

	for _, it := range l.CellIDBroadcastEUTRA {
		out.CellIDEUTRA = append(out.CellIDEUTRA, eutraCell(it.EUTRACGI, nil))
	}

	for _, it := range l.TAIBroadcastEUTRA {
		b := BroadcastTAI{TAI: tai(it.TAI)}
		for _, c := range it.CompletedCellsInTAI {
			b.Cells = append(b.Cells, eutraCell(c.EUTRACGI, nil))
		}

		out.TAIEUTRA = append(out.TAIEUTRA, b)
	}

	for _, it := range l.EmergencyAreaIDBroadcastEUTRA {
		b := BroadcastEmergencyArea{EmergencyAreaID: hex.EncodeToString(it.EmergencyAreaID[:])}
		for _, c := range it.CompletedCellsInEAI {
			b.Cells = append(b.Cells, eutraCell(c.EUTRACGI, nil))
		}

		out.EmergencyAreaIDEUTRA = append(out.EmergencyAreaIDEUTRA, b)
	}

	for _, it := range l.CellIDBroadcastNR {
		out.CellIDNR = append(out.CellIDNR, nrCell(it.NRCGI, nil))
	}

	for _, it := range l.TAIBroadcastNR {
		b := BroadcastTAI{TAI: tai(it.TAI)}
		for _, c := range it.CompletedCellsInTAI {
			b.Cells = append(b.Cells, nrCell(c.NRCGI, nil))
		}

		out.TAINR = append(out.TAINR, b)
	}

	for _, it := range l.EmergencyAreaIDBroadcastNR {
		b := BroadcastEmergencyArea{EmergencyAreaID: hex.EncodeToString(it.EmergencyAreaID[:])}
		for _, c := range it.CompletedCellsInEAI {
			b.Cells = append(b.Cells, nrCell(c.NRCGI, nil))
		}

		out.EmergencyAreaIDNR = append(out.EmergencyAreaIDNR, b)
	}

	return out
}

func broadcastCancelledAreaList(l ngap.BroadcastCancelledAreaList) BroadcastAreaList {
	var out BroadcastAreaList

	for _, it := range l.CellIDCancelledEUTRA {
		out.CellIDEUTRA = append(out.CellIDEUTRA, eutraCell(it.EUTRACGI, &it.NumberOfBroadcasts))
	}

	for _, it := range l.TAICancelledEUTRA {
		b := BroadcastTAI{TAI: tai(it.TAI)}
		for _, c := range it.CancelledCellsInTAI {
			b.Cells = append(b.Cells, eutraCell(c.EUTRACGI, &c.NumberOfBroadcasts))
		}

		out.TAIEUTRA = append(out.TAIEUTRA, b)
	}

	for _, it := range l.EmergencyAreaIDCancelledEUTRA {
		b := BroadcastEmergencyArea{EmergencyAreaID: hex.EncodeToString(it.EmergencyAreaID[:])}
		for _, c := range it.CancelledCellsInEAI {
			b.Cells = append(b.Cells, eutraCell(c.EUTRACGI, &c.NumberOfBroadcasts))
		}

		out.EmergencyAreaIDEUTRA = append(out.EmergencyAreaIDEUTRA, b)
	}

	for _, it := range l.CellIDCancelledNR {
		out.CellIDNR = append(out.CellIDNR, nrCell(it.NRCGI, &it.NumberOfBroadcasts))
	}

	for _, it := range l.TAICancelledNR {
		b := BroadcastTAI{TAI: tai(it.TAI)}
		for _, c := range it.CancelledCellsInTAI {
			b.Cells = append(b.Cells, nrCell(c.NRCGI, &c.NumberOfBroadcasts))
		}

		out.TAINR = append(out.TAINR, b)
	}

	for _, it := range l.EmergencyAreaIDCancelledNR {
		b := BroadcastEmergencyArea{EmergencyAreaID: hex.EncodeToString(it.EmergencyAreaID[:])}
		for _, c := range it.CancelledCellsInEAI {
			b.Cells = append(b.Cells, nrCell(c.NRCGI, &c.NumberOfBroadcasts))
		}

		out.EmergencyAreaIDNR = append(out.EmergencyAreaIDNR, b)
	}

	return out
}

func warningIdentity(id ngap.MessageIdentifier, sn ngap.SerialNumber) []IE {
	return []IE{
		ie(ngap.IDMessageIdentifier, ngap.CriticalityReject, fmt.Sprintf("%04x", uint16(id))),
		ie(ngap.IDSerialNumber, ngap.CriticalityReject, fmt.Sprintf("%04x", uint16(sn))),
	}
}

func buildWriteReplaceWarningRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseWriteReplaceWarningRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Write-Replace Warning Request: %v", err)}
	}

	ies := warningIdentity(m.MessageIdentifier, m.SerialNumber)

	if m.WarningAreaList != nil {
		ies = append(ies, ie(ngap.IDWarningAreaList, ngap.CriticalityIgnore, warningAreaList(*m.WarningAreaList)))
	}

	ies = append(ies,
		ie(ngap.IDRepetitionPeriod, ngap.CriticalityReject, int64(m.RepetitionPeriod)),
		ie(ngap.IDNumberOfBroadcastsRequested, ngap.CriticalityReject, int64(m.NumberOfBroadcastsRequested)),
	)

	if m.WarningType != nil {
		ies = append(ies, ie(ngap.IDWarningType, ngap.CriticalityIgnore, utils.NewRawOctets(m.WarningType[:])))
	}

	if m.WarningSecurityInfo != nil {
		ies = append(ies, ie(ngap.IDWarningSecurityInfo, ngap.CriticalityIgnore, utils.NewRawOctets(m.WarningSecurityInfo[:])))
	}

	if m.DataCodingScheme != nil {
		ies = append(ies, ie(ngap.IDDataCodingScheme, ngap.CriticalityIgnore, fmt.Sprintf("%02x", uint8(*m.DataCodingScheme))))
	}

	if len(m.WarningMessageContents) > 0 {
		ies = append(ies, ie(ngap.IDWarningMessageContents, ngap.CriticalityIgnore, utils.NewRawOctets(m.WarningMessageContents)))
	}

	if m.ConcurrentWarningMessageInd != nil {
		ies = append(ies, ie(ngap.IDConcurrentWarningMessageInd, ngap.CriticalityReject,
			utils.NamedEnum(uint8(*m.ConcurrentWarningMessageInd), m.ConcurrentWarningMessageInd.Name())))
	}

	if len(m.WarningAreaCoordinates) > 0 {
		ies = append(ies, ie(ngap.IDWarningAreaCoordinates, ngap.CriticalityIgnore, utils.NewRawOctets(m.WarningAreaCoordinates)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildWriteReplaceWarningResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParseWriteReplaceWarningResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Write-Replace Warning Response: %v", err)}
	}

	ies := warningIdentity(m.MessageIdentifier, m.SerialNumber)

	if m.BroadcastCompletedAreaList != nil {
		ies = append(ies, ie(ngap.IDBroadcastCompletedAreaList, ngap.CriticalityIgnore, broadcastCompletedAreaList(*m.BroadcastCompletedAreaList)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPWSCancelRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePWSCancelRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PWS Cancel Request: %v", err)}
	}

	ies := warningIdentity(m.MessageIdentifier, m.SerialNumber)

	if m.WarningAreaList != nil {
		ies = append(ies, ie(ngap.IDWarningAreaList, ngap.CriticalityIgnore, warningAreaList(*m.WarningAreaList)))
	}

	if m.CancelAllWarningMessages != nil {
		ies = append(ies, ie(ngap.IDCancelAllWarningMessages, ngap.CriticalityReject,
			utils.NamedEnum(uint8(*m.CancelAllWarningMessages), m.CancelAllWarningMessages.Name())))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPWSCancelResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePWSCancelResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PWS Cancel Response: %v", err)}
	}

	ies := warningIdentity(m.MessageIdentifier, m.SerialNumber)

	if m.BroadcastCancelledAreaList != nil {
		ies = append(ies, ie(ngap.IDBroadcastCancelledAreaList, ngap.CriticalityIgnore, broadcastCancelledAreaList(*m.BroadcastCancelledAreaList)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPWSRestartIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePWSRestartIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PWS Restart Indication: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDCellIDListForRestart, ngap.CriticalityReject, cellIDList(m.CellIDListForRestart.EUTRACGIs, m.CellIDListForRestart.NRCGIs)),
		ie(ngap.IDGlobalRANNodeID, ngap.CriticalityReject, buildGlobalRANNodeID(m.GlobalRANNodeID)),
		ie(ngap.IDTAIListForRestart, ngap.CriticalityReject, taiList(m.TAIListForRestart)),
	}

	if len(m.EmergencyAreaIDListForRestart) > 0 {
		ies = append(ies, ie(ngap.IDEmergencyAreaIDListForRestart, ngap.CriticalityReject, emergencyAreaIDs(m.EmergencyAreaIDListForRestart)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPWSFailureIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePWSFailureIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PWS Failure Indication: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDPWSFailedCellIDList, ngap.CriticalityReject, cellIDList(m.PWSFailedCellIDList.EUTRACGIs, m.PWSFailedCellIDList.NRCGIs)),
		ie(ngap.IDGlobalRANNodeID, ngap.CriticalityReject, buildGlobalRANNodeID(m.GlobalRANNodeID)),
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
