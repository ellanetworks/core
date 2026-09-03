// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

// ERABToBeModified is an E-RAB whose QoS the MME asks the eNB to change, with
// the NAS message that tells the UE about it (TS 36.413 §9.1.3.3).
type ERABToBeModified struct {
	ERABID uint8  `json:"erab_id"`
	QCI    uint8  `json:"qci"`
	ARP    ARP    `json:"arp"`
	NASPDU NASPDU `json:"nas_pdu"`
}

// ERABModifiedItem is an E-RAB the eNB confirms modified (TS 36.413 §9.1.3.4).
type ERABModifiedItem struct {
	ERABID uint8 `json:"erab_id"`
}

// ERABModifiedTunnel is an E-RAB whose downlink S1-U endpoint the eNB reports,
// so the MME can move the tunnel (TS 36.413 §9.1.3.8).
type ERABModifiedTunnel struct {
	ERABID                uint8  `json:"erab_id"`
	TransportLayerAddress string `json:"transport_layer_address"`
	DLGTPTEID             uint32 `json:"dl_gtp_teid"`
}

func erabModifiedTunnels(items []s1ap.ERABToBeModifiedItemBearerModInd) []ERABModifiedTunnel {
	out := make([]ERABModifiedTunnel, 0, len(items))
	for _, it := range items {
		out = append(out, ERABModifiedTunnel{
			ERABID:                uint8(it.ERABID),
			TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
			DLGTPTEID:             uint32(it.DLGTPTEID),
		})
	}

	return out
}

func buildERABModifyRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABModifyRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Modify Request: %v", err)}, ""
	}

	erabs := make([]ERABToBeModified, 0, len(m.ERABToBeModified))
	for _, it := range m.ERABToBeModified {
		erabs = append(erabs, ERABToBeModified{
			ERABID: uint8(it.ERABID),
			QCI:    uint8(it.QoS.QCI),
			ARP: ARP{
				PriorityLevel:           it.QoS.ARP.PriorityLevel,
				PreemptionCapability:    uint8(it.QoS.ARP.PreemptionCapability),
				PreemptionVulnerability: uint8(it.QoS.ARP.PreemptionVulnerability),
			},
			NASPDU: nasPDU(it.NASPDU),
		})
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if ambr := m.UEAggregateMaximumBitRate; ambr != nil {
		ies = append(ies, ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityReject, AMBR{DL: uint64(ambr.DL), UL: uint64(ambr.UL)}))
	}

	ies = append(ies, ie(s1ap.IDERABToBeModifiedListBearerModReq, s1ap.CriticalityReject, erabs))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Modify Request (MME-UE %d, eNB-UE %d, %d E-RAB)", m.MMEUES1APID, m.ENBUES1APID, len(m.ERABToBeModified))
}

func buildERABModifyResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABModifyResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Modify Response: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if len(m.ERABModify) > 0 {
		modified := make([]ERABModifiedItem, 0, len(m.ERABModify))
		for _, it := range m.ERABModify {
			modified = append(modified, ERABModifiedItem{ERABID: uint8(it.ERABID)})
		}

		ies = append(ies, ie(s1ap.IDERABModifyListBearerModRes, s1ap.CriticalityIgnore, modified))
	}

	if len(m.ERABFailedToModify) > 0 {
		ies = append(ies, ie(s1ap.IDERABFailedToModifyList, s1ap.CriticalityIgnore, erabFailedItems(m.ERABFailedToModify)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(s1ap.IDUserLocationInformation, s1ap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Modify Response (MME-UE %s, eNB-UE %s, %d E-RAB)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID), len(m.ERABModify))
}

func buildERABModificationIndication(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABModificationIndication(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Modification Indication: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDERABToBeModifiedListBearerModInd, s1ap.CriticalityReject, erabModifiedTunnels(m.ToBeModified)),
	}

	if len(m.NotToBeModified) > 0 {
		ies = append(ies, ie(s1ap.IDERABNotToBeModifiedListBearerModInd, s1ap.CriticalityReject, erabModifiedTunnels(m.NotToBeModified)))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(s1ap.IDUserLocationInformation, s1ap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Modification Indication (MME-UE %d, eNB-UE %d, %d E-RAB)", m.MMEUES1APID, m.ENBUES1APID, len(m.ToBeModified))
}

func buildERABModificationConfirm(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABModificationConfirm(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Modification Confirm: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if len(m.ModifiedERABs) > 0 {
		modified := make([]ERABModifiedItem, 0, len(m.ModifiedERABs))
		for _, id := range m.ModifiedERABs {
			modified = append(modified, ERABModifiedItem{ERABID: uint8(id)})
		}

		ies = append(ies, ie(s1ap.IDERABModifyListBearerModConf, s1ap.CriticalityIgnore, modified))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Modification Confirm (MME-UE %s, eNB-UE %s, %d E-RAB)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID), len(m.ModifiedERABs))
}
