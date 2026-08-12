// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// TargetID renders whichever alternative of TS 36.413 §9.2.1.24 the message
// carried. An eps-to-5gs HANDOVER REQUIRED names an NG-RAN node instead of an
// eNB, so rendering the eNB fields unconditionally would show an all-zero eNB
// target for exactly the message inter-system handover introduces.
type TargetID struct {
	GlobalENBID *GlobalENBID     `json:"global_enb_id,omitempty"`
	SelectedTAI *TAI             `json:"selected_tai,omitempty"`
	NgRanNode   *TargetNgRanNode `json:"target_ng_ran_node,omitempty"`
}

// TargetNgRanNode is the targetgNgRanNode-ID alternative: an NG-RAN node and the
// 5GS tracking area it serves, whose TAC is three octets where the EPS one is
// two.
type TargetNgRanNode struct {
	GlobalGNBID   *GlobalGNBID `json:"global_gnb_id,omitempty"`
	GlobalNgENBID *GlobalENBID `json:"global_ng_enb_id,omitempty"`
	SelectedTAI   FiveGSTAI    `json:"selected_tai"`
}

// GlobalGNBID is a gNB identity: a PLMN and a gNB ID whose width the sender
// chose within SIZE(22..32), so the width is rendered beside the value.
type GlobalGNBID struct {
	PLMNID PLMNID `json:"plmn_id"`
	GNBID  uint32 `json:"gnb_id"`
	Bits   int    `json:"gnb_id_bits"`
}

// FiveGSTAI is a 5GS tracking area identity (TS 36.413 §9.2.1.121).
type FiveGSTAI struct {
	PLMNID PLMNID `json:"plmn_id"`
	TAC    uint32 `json:"tac"`
}

func targetID(t s1ap.TargetID) TargetID {
	if n := t.TargetNgRanNodeID; n != nil {
		node := &TargetNgRanNode{
			SelectedTAI: FiveGSTAI{PLMNID: plmnToID(n.SelectedTAI.PLMNIdentity), TAC: uint32(n.SelectedTAI.TAC)},
		}

		if g := n.GlobalRANNodeID.GNB; g != nil {
			node.GlobalGNBID = &GlobalGNBID{
				PLMNID: plmnToID(g.GlobalGNBID.PLMNIdentity),
				GNBID:  g.GlobalGNBID.GNBID.Value,
				Bits:   g.GlobalGNBID.GNBID.Bits,
			}
		}

		if e := n.GlobalRANNodeID.NgENB; e != nil {
			id := globalENBID(e.GlobalNgENBID)
			node.GlobalNgENBID = &id
		}

		return TargetID{NgRanNode: node}
	}

	enb := globalENBID(t.TargeteNBID.GlobalENBID)
	selected := tai(t.TargeteNBID.SelectedTAI)

	return TargetID{GlobalENBID: &enb, SelectedTAI: &selected}
}

type SecurityContext struct {
	NextHopChainingCount uint8  `json:"next_hop_chaining_count"`
	NextHopParameter     string `json:"next_hop_parameter"`
}

type ERABToBeSetupHO struct {
	ERABID                uint8  `json:"erab_id"`
	QCI                   uint8  `json:"qci"`
	ARP                   ARP    `json:"arp"`
	TransportLayerAddress string `json:"transport_layer_address"`
	GTPTEID               uint32 `json:"gtp_teid"`
}

type ERABAdmitted struct {
	ERABID                  uint8  `json:"erab_id"`
	TransportLayerAddress   string `json:"transport_layer_address"`
	GTPTEID                 uint32 `json:"gtp_teid"`
	DLTransportLayerAddress string `json:"dl_transport_layer_address,omitempty"`
	DLGTPTEID               uint32 `json:"dl_gtp_teid,omitempty"`
	ULTransportLayerAddress string `json:"ul_transport_layer_address,omitempty"`
	ULGTPTEID               uint32 `json:"ul_gtp_teid,omitempty"`
}

func handoverType(t s1ap.HandoverType) utils.EnumField {
	return utils.NamedEnum(uint8(t), t.Name())
}

func securityContext(s s1ap.SecurityContext) SecurityContext {
	return SecurityContext{
		NextHopChainingCount: s.NextHopChainingCount,
		NextHopParameter:     hex.EncodeToString(s.NextHopParameter[:]),
	}
}

func erabItems(items []s1ap.ERABItem) []ERABFailedItem {
	out := make([]ERABFailedItem, 0, len(items))
	for _, it := range items {
		out = append(out, ERABFailedItem{ERABID: uint8(it.ERABID), Cause: cause(it.Cause)})
	}

	return out
}

func buildHandoverRequired(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverRequired(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Required: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDHandoverType, s1ap.CriticalityReject, handoverType(m.HandoverType)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = append(ies, ie(s1ap.IDTargetID, s1ap.CriticalityReject, targetID(m.TargetID)))

	if m.DirectForwardingPathAvailability != nil {
		d := *m.DirectForwardingPathAvailability
		ies = append(ies, ie(s1ap.IDDirectForwardingPathAvailability, s1ap.CriticalityIgnore, utils.NamedEnum(uint8(d), d.Name())))
	}

	ies = append(ies, ie(s1ap.IDSourceToTargetTransparentContainer, s1ap.CriticalityReject, hex.EncodeToString(m.SourceToTarget)))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Required (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Request: %v", err)}, ""
	}

	erabs := make([]ERABToBeSetupHO, 0, len(m.ERABToBeSetup))
	for _, it := range m.ERABToBeSetup {
		erabs = append(erabs, ERABToBeSetupHO{
			ERABID: uint8(it.ERABID),
			QCI:    uint8(it.QoS.QCI),
			ARP: ARP{
				PriorityLevel:           it.QoS.ARP.PriorityLevel,
				PreemptionCapability:    uint8(it.QoS.ARP.PreemptionCapability),
				PreemptionVulnerability: uint8(it.QoS.ARP.PreemptionVulnerability),
			},
			TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
			GTPTEID:               uint32(it.GTPTEID),
		})
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDHandoverType, s1ap.CriticalityReject, handoverType(m.HandoverType)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = append(ies,
		ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityReject, AMBR{DL: uint64(m.UEAMBR.DL), UL: uint64(m.UEAMBR.UL)}),
		ie(s1ap.IDERABToBeSetupListHOReq, s1ap.CriticalityReject, erabs),
		ie(s1ap.IDSourceToTargetTransparentContainer, s1ap.CriticalityReject, hex.EncodeToString(m.SourceToTarget)),
		ie(s1ap.IDUESecurityCapabilities, s1ap.CriticalityReject, UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms, "EIA"),
		}),
	)

	if m.HandoverRestrictionList != nil {
		ies = append(ies, ie(s1ap.IDHandoverRestrictionList, s1ap.CriticalityIgnore, handoverRestrictionList(*m.HandoverRestrictionList)))
	}

	ies = append(ies, ie(s1ap.IDSecurityContext, s1ap.CriticalityReject, securityContext(m.SecurityContext)))

	if len(m.NASSecurityParameterstoEUTRAN) > 0 {
		ies = append(ies, ie(s1ap.IDNASSecurityParameterstoEUTRAN, s1ap.CriticalityReject, hex.EncodeToString(m.NASSecurityParameterstoEUTRAN)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Request (MME-UE %d, %d E-RAB)", m.MMEUES1APID, len(m.ERABToBeSetup))
}

func buildHandoverRequestAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Request Acknowledge: %v", err)}, ""
	}

	admitted := make([]ERABAdmitted, 0, len(m.ERABAdmitted))
	for _, it := range m.ERABAdmitted {
		a := ERABAdmitted{
			ERABID:                  uint8(it.ERABID),
			TransportLayerAddress:   transportLayerAddress(it.TransportLayerAddress),
			GTPTEID:                 uint32(it.GTPTEID),
			DLTransportLayerAddress: transportLayerAddressOrEmpty(it.DLTransportLayerAddr),
			ULTransportLayerAddress: transportLayerAddressOrEmpty(it.ULTransportLayerAddr),
		}

		if it.DLGTPTEID != nil {
			a.DLGTPTEID = uint32(*it.DLGTPTEID)
		}

		if it.ULGTPTEID != nil {
			a.ULGTPTEID = uint32(*it.ULGTPTEID)
		}

		admitted = append(admitted, a)
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	ies = append(ies, ie(s1ap.IDERABAdmittedList, s1ap.CriticalityIgnore, admitted))

	if len(m.ERABFailedToSetup) > 0 {
		ies = append(ies, ie(s1ap.IDERABFailedToSetupListHOReqAck, s1ap.CriticalityIgnore, erabItems(m.ERABFailedToSetup)))
	}

	ies = append(ies, ie(s1ap.IDTargetToSourceTransparentContainer, s1ap.CriticalityReject, hex.EncodeToString(m.TargetToSource)))

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Request Acknowledge (MME-UE %s, eNB-UE %s, %d admitted)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID), len(m.ERABAdmitted))
}

func buildHandoverFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Failure: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Failure (MME-UE %s)", ueIDText(m.MMEUES1APID))
}

func buildHandoverCommand(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverCommand(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Command: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDHandoverType, s1ap.CriticalityReject, handoverType(m.HandoverType)),
	}

	if len(m.NASSecurityParametersfromEUTRAN) > 0 {
		ies = append(ies, ie(s1ap.IDNASSecurityParametersfromEUTRAN, s1ap.CriticalityReject, hex.EncodeToString(m.NASSecurityParametersfromEUTRAN)))
	}

	if len(m.ERABSubjecttoDataForwarding) > 0 {
		ies = append(ies, ie(s1ap.IDERABSubjecttoDataForwardingList, s1ap.CriticalityIgnore, erabDataForwardingItems(m.ERABSubjecttoDataForwarding)))
	}

	if len(m.ERABToRelease) > 0 {
		ies = append(ies, ie(s1ap.IDERABtoReleaseListHOCmd, s1ap.CriticalityIgnore, erabItems(m.ERABToRelease)))
	}

	ies = append(ies, ie(s1ap.IDTargetToSourceTransparentContainer, s1ap.CriticalityReject, hex.EncodeToString(m.TargetToSource)))

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Command (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverPreparationFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverPreparationFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Preparation Failure: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Preparation Failure (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}

func buildHandoverNotify(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Notify: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if m.EUTRANCGI != nil {
		ies = append(ies, ie(s1ap.IDEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(*m.EUTRANCGI)))
	}

	if m.TAI != nil {
		ies = append(ies, ie(s1ap.IDTAI, s1ap.CriticalityIgnore, tai(*m.TAI)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Notify (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverCancel(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Cancel: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Cancel (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverCancelAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverCancelAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Cancel Acknowledge: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Cancel Acknowledge (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}

func buildENBStatusTransfer(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseENBStatusTransfer(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse eNB Status Transfer: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDENBStatusTransferTransparentContainer, s1ap.CriticalityReject, hex.EncodeToString(m.Container)),
	}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("eNB Status Transfer (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildMMEStatusTransfer(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseMMEStatusTransfer(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse MME Status Transfer: %v", err)}, ""
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDENBStatusTransferTransparentContainer, s1ap.CriticalityReject, hex.EncodeToString(m.Container)),
	}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("MME Status Transfer (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func transportLayerAddressOrEmpty(b s1ap.TransportLayerAddress) string {
	if len(b) == 0 {
		return ""
	}

	return transportLayerAddress(b)
}

// HandoverRestrictionList is the decoded Handover Restriction List IE
// (TS 36.413 §9.2.1.22): where the target may hand the UE on to.
type HandoverRestrictionList struct {
	ServingPLMN        PLMNID           `json:"serving_plmn"`
	EquivalentPLMNs    []PLMNID         `json:"equivalent_plmns,omitempty"`
	ForbiddenTAs       []ForbiddenTAs   `json:"forbidden_tas,omitempty"`
	ForbiddenLAs       []ForbiddenLAs   `json:"forbidden_las,omitempty"`
	ForbiddenInterRATs *utils.EnumField `json:"forbidden_inter_rats,omitempty"`
}

// ForbiddenTAs is one PLMN's forbidden tracking areas (TS 36.413 §9.2.1.22).
type ForbiddenTAs struct {
	PLMNID PLMNID   `json:"plmn_id"`
	TACs   []uint16 `json:"tacs"`
}

// ForbiddenLAs is one PLMN's forbidden location areas (TS 36.413 §9.2.1.22).
type ForbiddenLAs struct {
	PLMNID PLMNID   `json:"plmn_id"`
	LACs   []uint16 `json:"lacs"`
}

func handoverRestrictionList(l s1ap.HandoverRestrictionList) HandoverRestrictionList {
	out := HandoverRestrictionList{ServingPLMN: plmnToID(l.ServingPLMN)}

	for _, p := range l.EquivalentPLMNs {
		out.EquivalentPLMNs = append(out.EquivalentPLMNs, plmnToID(p))
	}

	for _, it := range l.ForbiddenTAs {
		e := ForbiddenTAs{PLMNID: plmnToID(it.PLMNIdentity)}
		for _, tac := range it.ForbiddenTACs {
			e.TACs = append(e.TACs, uint16(tac))
		}

		out.ForbiddenTAs = append(out.ForbiddenTAs, e)
	}

	for _, it := range l.ForbiddenLAs {
		e := ForbiddenLAs{PLMNID: plmnToID(it.PLMNIdentity)}
		for _, lac := range it.ForbiddenLACs {
			e.LACs = append(e.LACs, uint16(lac))
		}

		out.ForbiddenLAs = append(out.ForbiddenLAs, e)
	}

	if l.ForbiddenInterRATs != nil {
		r := *l.ForbiddenInterRATs
		e := utils.NamedEnum(uint8(r), r.Name())
		out.ForbiddenInterRATs = &e
	}

	return out
}

// ERABDataForwarding is one E-RAB the source asked the target to forward data
// for (TS 36.413).
type ERABDataForwarding struct {
	ERABID                  uint8  `json:"erab_id"`
	DLTransportLayerAddress string `json:"dl_transport_layer_address,omitempty"`
	DLGTPTEID               uint32 `json:"dl_gtp_teid,omitempty"`
	ULTransportLayerAddress string `json:"ul_transport_layer_address,omitempty"`
	ULGTPTEID               uint32 `json:"ul_gtp_teid,omitempty"`
}

func erabDataForwardingItems(items []s1ap.ERABDataForwardingItem) []ERABDataForwarding {
	out := make([]ERABDataForwarding, 0, len(items))

	for _, it := range items {
		e := ERABDataForwarding{
			ERABID:                  uint8(it.ERABID),
			DLTransportLayerAddress: transportLayerAddressOrEmpty(it.DLTransportLayerAddr),
			ULTransportLayerAddress: transportLayerAddressOrEmpty(it.ULTransportLayerAddr),
		}

		if it.DLGTPTEID != nil {
			e.DLGTPTEID = uint32(*it.DLGTPTEID)
		}

		if it.ULGTPTEID != nil {
			e.ULGTPTEID = uint32(*it.ULGTPTEID)
		}

		out = append(out, e)
	}

	return out
}
