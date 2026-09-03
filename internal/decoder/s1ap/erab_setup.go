// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

// ERABToBeSetupBearer is a decoded E-RAB the MME asks the eNB to add for an
// additional PDN connection (TS 36.413 §9.1.3.1). Unlike the context-setup
// variant its NAS-PDU is mandatory: it carries the ACTIVATE DEFAULT EPS BEARER
// CONTEXT REQUEST.
type ERABToBeSetupBearer struct {
	ERABID                uint8  `json:"erab_id"`
	QCI                   uint8  `json:"qci"`
	ARP                   ARP    `json:"arp"`
	TransportLayerAddress string `json:"transport_layer_address"`
	GTPTEID               uint32 `json:"gtp_teid"`
	NASPDU                NASPDU `json:"nas_pdu"`
}

func buildERABSetupRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABSetupRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Setup Request: %v", err)}, ""
	}

	erabs := make([]ERABToBeSetupBearer, 0, len(m.ERABToBeSetup))
	for _, it := range m.ERABToBeSetup {
		erabs = append(erabs, ERABToBeSetupBearer{
			ERABID: uint8(it.ERABID),
			QCI:    uint8(it.QoS.QCI),
			ARP: ARP{
				PriorityLevel:           it.QoS.ARP.PriorityLevel,
				PreemptionCapability:    uint8(it.QoS.ARP.PreemptionCapability),
				PreemptionVulnerability: uint8(it.QoS.ARP.PreemptionVulnerability),
			},
			TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
			GTPTEID:               uint32(it.GTPTEID),
			NASPDU:                nasPDU(it.NASPDU),
		})
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if ambr := m.UEAggregateMaximumBitRate; ambr != nil {
		ies = append(ies, ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityReject, AMBR{DL: uint64(ambr.DL), UL: uint64(ambr.UL)}))
	}

	ies = append(ies, ie(s1ap.IDERABToBeSetupListBearerSUReq, s1ap.CriticalityReject, erabs))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Setup Request (MME-UE %d, eNB-UE %d, %d E-RAB)", m.MMEUES1APID, m.ENBUES1APID, len(m.ERABToBeSetup))
}

func buildERABSetupResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseERABSetupResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse E-RAB Setup Response: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if len(m.ERABSetup) > 0 {
		setup := make([]ERABSetupItem, 0, len(m.ERABSetup))
		for _, it := range m.ERABSetup {
			setup = append(setup, ERABSetupItem{
				ERABID:                uint8(it.ERABID),
				TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
				GTPTEID:               uint32(it.GTPTEID),
			})
		}

		ies = append(ies, ie(s1ap.IDERABSetupListBearerSURes, s1ap.CriticalityIgnore, setup))
	}

	if len(m.ERABFailedToSetup) > 0 {
		ies = append(ies, ie(s1ap.IDERABFailedToSetupListBearerSURes, s1ap.CriticalityIgnore, erabFailedItems(m.ERABFailedToSetup)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(s1ap.IDUserLocationInformation, s1ap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("E-RAB Setup Response (MME-UE %s, eNB-UE %s, %d E-RAB)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID), len(m.ERABSetup))
}

func erabFailedItems(items []s1ap.ERABItem) []ERABFailedItem {
	out := make([]ERABFailedItem, 0, len(items))
	for _, it := range items {
		out = append(out, ERABFailedItem{ERABID: uint8(it.ERABID), Cause: cause(it.Cause)})
	}

	return out
}
