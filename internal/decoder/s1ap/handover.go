// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

type TargetID struct {
	GlobalENBID GlobalENBID `json:"global_enb_id"`
	SelectedTAI TAI         `json:"selected_tai"`
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

var handoverTypeNames = map[s1ap.HandoverType]string{
	s1ap.HandoverTypeIntraLTE:   "intralte",
	s1ap.HandoverTypeLTEtoUTRAN: "ltetoutran",
	s1ap.HandoverTypeLTEtoGERAN: "ltetogeran",
	s1ap.HandoverTypeUTRANtoLTE: "utrantolte",
	s1ap.HandoverTypeGERANtoLTE: "gerantolte",
}

func handoverType(t s1ap.HandoverType) utils.EnumField[uint64] {
	name, ok := handoverTypeNames[t]

	return utils.MakeEnum(uint64(t), name, !ok)
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
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idHandoverType, s1ap.CriticalityReject, handoverType(m.HandoverType)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
		ie(idTargetID, s1ap.CriticalityReject, TargetID{GlobalENBID: globalENBID(m.TargetID.TargeteNBID.GlobalENBID), SelectedTAI: tai(m.TargetID.TargeteNBID.SelectedTAI)}),
		ie(idSourceToTargetContainer, s1ap.CriticalityReject, hex.EncodeToString(m.SourceToTarget)),
	}
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
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idHandoverType, s1ap.CriticalityReject, handoverType(m.HandoverType)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
		ie(idUEAggregateMaximumBitrate, s1ap.CriticalityReject, AMBR{DL: uint64(m.UEAMBR.DL), UL: uint64(m.UEAMBR.UL)}),
		ie(idERABToBeSetupListHOReq, s1ap.CriticalityReject, erabs),
		ie(idSourceToTargetContainer, s1ap.CriticalityReject, hex.EncodeToString(m.SourceToTarget)),
		ie(idUESecurityCapabilities, s1ap.CriticalityReject, UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms, "EIA"),
		}),
		ie(idSecurityContext, s1ap.CriticalityReject, securityContext(m.SecurityContext)),
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

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityIgnore, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityIgnore, uint32(m.ENBUES1APID)),
		ie(idERABAdmittedList, s1ap.CriticalityIgnore, admitted),
	}

	if len(m.ERABFailedToSetup) > 0 {
		ies = append(ies, ie(idERABFailedToSetupListHOReqAck, s1ap.CriticalityIgnore, erabItems(m.ERABFailedToSetup)))
	}

	ies = append(ies, ie(idTargetToSourceContainer, s1ap.CriticalityReject, hex.EncodeToString(m.TargetToSource)))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Request Acknowledge (MME-UE %d, eNB-UE %d, %d admitted)", m.MMEUES1APID, m.ENBUES1APID, len(m.ERABAdmitted))
}

func buildHandoverFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Failure: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityIgnore, uint32(m.MMEUES1APID)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
	}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Failure (MME-UE %d)", m.MMEUES1APID)
}

func buildHandoverCommand(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverCommand(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Command: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idHandoverType, s1ap.CriticalityReject, handoverType(m.HandoverType)),
	}

	if len(m.ERABToRelease) > 0 {
		ies = append(ies, ie(idERABtoReleaseListHOCmd, s1ap.CriticalityIgnore, erabItems(m.ERABToRelease)))
	}

	ies = append(ies, ie(idTargetToSourceContainer, s1ap.CriticalityReject, hex.EncodeToString(m.TargetToSource)))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Command (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverPreparationFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverPreparationFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Preparation Failure: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
	}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Preparation Failure (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverNotify(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Notify: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idEUTRANCGI, s1ap.CriticalityIgnore, eutranCGI(m.EUTRANCGI)),
		ie(idTAIList, s1ap.CriticalityIgnore, tai(m.TAI)),
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
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idCause, s1ap.CriticalityIgnore, cause(m.Cause)),
	}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Cancel (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildHandoverCancelAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseHandoverCancelAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Handover Cancel Acknowledge: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityIgnore, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityIgnore, uint32(m.ENBUES1APID)),
	}
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Handover Cancel Acknowledge (MME-UE %d, eNB-UE %d)", m.MMEUES1APID, m.ENBUES1APID)
}

func buildENBStatusTransfer(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseENBStatusTransfer(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse eNB Status Transfer: %v", err)}, ""
	}

	ies := []IE{
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idENBStatusTransferContainer, s1ap.CriticalityReject, hex.EncodeToString(m.Container)),
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
		ie(idMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(idENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(idENBStatusTransferContainer, s1ap.CriticalityReject, hex.EncodeToString(m.Container)),
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
