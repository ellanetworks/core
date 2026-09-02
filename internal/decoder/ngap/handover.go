// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

// TargetID names the node a handover is aimed at: an NG-RAN node for an intra-5GS
// handover, an ng-eNB for one leaving 5GS (TS 38.413 §9.3.1.15).
type TargetID struct {
	TargetRANNodeID *TargetRANNodeID `json:"target_ran_node_id,omitempty"`
	TargetENBID     *TargetENBID     `json:"target_enb_id,omitempty"`
}

type TargetRANNodeID struct {
	GlobalRANNodeID GlobalRANNodeIDIE `json:"global_ran_node_id"`
	SelectedTAI     TAI               `json:"selected_tai"`
}

type TargetENBID struct {
	GlobalNgENBID  GlobalNgENBID `json:"global_ng_enb_id"`
	SelectedEPSTAI EPSTAI        `json:"selected_eps_tai"`
}

type GlobalNgENBID struct {
	PLMNIdentity PLMNID `json:"plmn_identity"`
	NgENBID      string `json:"ng_enb_id"`
}

type EPSTAI struct {
	PLMNIdentity PLMNID `json:"plmn_identity"`
	TAC          string `json:"tac"`
}

// DataForwardingResponseDRB is one DRB's forwarding endpoints, as the target
// node reports them (TS 38.413 §9.3.1.35).
type DataForwardingResponseDRB struct {
	DRBID                        uint8      `json:"drb_id"`
	DLForwardingUPTNLInformation *GTPTunnel `json:"dl_forwarding_up_tnl_information,omitempty"`
	ULForwardingUPTNLInformation *GTPTunnel `json:"ul_forwarding_up_tnl_information,omitempty"`
}

// QosFlowWithDataForwarding is a QoS flow the target admitted, and whether it
// accepted data forwarding for it.
type QosFlowWithDataForwarding struct {
	QosFlowIdentifier      int64 `json:"qos_flow_identifier"`
	DataForwardingAccepted *bool `json:"data_forwarding_accepted,omitempty"`
}

// HandoverRequiredTransferDecoded is the source node's per-session transfer
// (TS 38.413 §9.3.4.15).
type HandoverRequiredTransferDecoded struct {
	DirectForwardingPathAvailability *bool `json:"direct_forwarding_path_availability,omitempty"`
}

// HandoverCommandTransferDecoded is the per-session forwarding plan the source
// node is given (TS 38.413 §9.3.4.16).
type HandoverCommandTransferDecoded struct {
	DLForwardingUPTNLInformation *GTPTunnel                  `json:"dl_forwarding_up_tnl_information,omitempty"`
	QosFlowToBeForwarded         []int64                     `json:"qos_flow_to_be_forwarded,omitempty"`
	DataForwardingResponseDRB    []DataForwardingResponseDRB `json:"data_forwarding_response_drb,omitempty"`
}

// HandoverRequestAcknowledgeTransferDecoded is what the target node reports per
// admitted session (TS 38.413 §9.3.4.13).
type HandoverRequestAcknowledgeTransferDecoded struct {
	DLNGUUPTNLInformation        GTPTunnel                   `json:"dl_ng_u_up_tnl_information"`
	DLForwardingUPTNLInformation *GTPTunnel                  `json:"dl_forwarding_up_tnl_information,omitempty"`
	SecurityResult               *SecurityResult             `json:"security_result,omitempty"`
	QosFlowSetupResponse         []QosFlowWithDataForwarding `json:"qos_flow_setup_response,omitempty"`
	QosFlowFailedToSetup         []QosFlowWithCause          `json:"qos_flow_failed_to_setup,omitempty"`
	DataForwardingResponseDRB    []DataForwardingResponseDRB `json:"data_forwarding_response_drb,omitempty"`
}

// HandoverResourceAllocationUnsuccessfulTransferDecoded says why the target node
// refused one session (TS 38.413 §9.3.4.14).
type HandoverResourceAllocationUnsuccessfulTransferDecoded struct {
	Cause                  Cause                   `json:"cause"`
	CriticalityDiagnostics *CriticalityDiagnostics `json:"criticality_diagnostics,omitempty"`
}

// HandoverSession is one PDU session in a handover list, with whichever transfer
// travelled with it.
type HandoverSession struct {
	PDUSessionID int64   `json:"pdu_session_id"`
	SNSSAI       *SNSSAI `json:"snssai,omitempty"`

	HandoverRequiredTransfer                       *HandoverRequiredTransferDecoded                       `json:"handover_required_transfer,omitempty"`
	HandoverCommandTransfer                        *HandoverCommandTransferDecoded                        `json:"handover_command_transfer,omitempty"`
	HandoverRequestAcknowledgeTransfer             *HandoverRequestAcknowledgeTransferDecoded             `json:"handover_request_acknowledge_transfer,omitempty"`
	HandoverResourceAllocationUnsuccessfulTransfer *HandoverResourceAllocationUnsuccessfulTransferDecoded `json:"handover_resource_allocation_unsuccessful_transfer,omitempty"`
	SetupRequestTransfer                           *PDUSessionResourceSetupRequestTransfer                `json:"pdu_session_resource_setup_request_transfer,omitempty"`
	CauseTransfer                                  *CauseTransferDecoded                                  `json:"cause_transfer,omitempty"`

	TransferHex string `json:"transfer_hex,omitempty"` // set when the transfer did not decode
	Error       string `json:"error,omitempty"`
}

func dataForwardingResponseDRBs(list ngap.DataForwardingResponseDRBList) []DataForwardingResponseDRB {
	out := make([]DataForwardingResponseDRB, 0, len(list))

	for _, it := range list {
		entry := DataForwardingResponseDRB{DRBID: uint8(it.DRBID)}

		if it.DLForwardingUPTNLInformation != nil {
			tunnel := libGTPTunnel(*it.DLForwardingUPTNLInformation)
			entry.DLForwardingUPTNLInformation = &tunnel
		}

		if it.ULForwardingUPTNLInformation != nil {
			tunnel := libGTPTunnel(*it.ULForwardingUPTNLInformation)
			entry.ULForwardingUPTNLInformation = &tunnel
		}

		out = append(out, entry)
	}

	return out
}

func libHandoverRequiredTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParseHandoverRequiredTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &HandoverRequiredTransferDecoded{}

	if t.DirectForwardingPathAvailability != nil {
		available := *t.DirectForwardingPathAvailability == ngap.DirectForwardingPathAvailable
		out.DirectForwardingPathAvailability = &available
	}

	return out, nil
}

func libHandoverCommandTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParseHandoverCommandTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &HandoverCommandTransferDecoded{DataForwardingResponseDRB: dataForwardingResponseDRBs(t.DataForwardingResponseDRB)}

	if t.DLForwardingUPTNLInformation != nil {
		tunnel := libGTPTunnel(*t.DLForwardingUPTNLInformation)
		out.DLForwardingUPTNLInformation = &tunnel
	}

	for _, f := range t.QosFlowToBeForwarded {
		out.QosFlowToBeForwarded = append(out.QosFlowToBeForwarded, int64(f.QosFlowIdentifier))
	}

	return out, nil
}

func libHandoverRequestAcknowledgeTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParseHandoverRequestAcknowledgeTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &HandoverRequestAcknowledgeTransferDecoded{
		DLNGUUPTNLInformation:     libGTPTunnel(t.DLNGUUPTNLInformation),
		QosFlowFailedToSetup:      qosFlowsWithCause(t.QosFlowFailedToSetup),
		DataForwardingResponseDRB: dataForwardingResponseDRBs(t.DataForwardingResponseDRB),
	}

	if t.DLForwardingUPTNLInformation != nil {
		tunnel := libGTPTunnel(*t.DLForwardingUPTNLInformation)
		out.DLForwardingUPTNLInformation = &tunnel
	}

	if t.SecurityResult != nil {
		result := securityResult(*t.SecurityResult)
		out.SecurityResult = &result
	}

	for _, f := range t.QosFlowSetupResponse {
		entry := QosFlowWithDataForwarding{QosFlowIdentifier: int64(f.QosFlowIdentifier)}
		if f.DataForwardingAccepted != nil {
			accepted := true
			entry.DataForwardingAccepted = &accepted
		}

		out.QosFlowSetupResponse = append(out.QosFlowSetupResponse, entry)
	}

	return out, nil
}

func libHandoverResourceAllocationUnsuccessfulTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParseHandoverResourceAllocationUnsuccessfulTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &HandoverResourceAllocationUnsuccessfulTransferDecoded{Cause: cause(t.Cause)}
	if t.CriticalityDiagnostics != nil {
		cd := criticalityDiagnostics(*t.CriticalityDiagnostics)
		out.CriticalityDiagnostics = &cd
	}

	return out, nil
}

func libHandoverPreparationUnsuccessfulTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParseHandoverPreparationUnsuccessfulTransfer(raw)
	if err != nil {
		return nil, err
	}

	return &CauseTransferDecoded{Cause: cause(t.Cause)}, nil
}

func libHandoverSetupRequestTransfer(raw ngap.TransferContainer) (any, error) {
	return libPDUSessionResourceSetupRequestTransfer(raw)
}

func handoverSession(id ngap.PDUSessionID, raw ngap.TransferContainer, decode func(ngap.TransferContainer) (any, error)) HandoverSession {
	s := HandoverSession{PDUSessionID: int64(id)}

	decoded, err := decode(raw)
	if err != nil {
		s.Error = err.Error()
		s.TransferHex = hex.EncodeToString(raw)

		return s
	}

	switch v := decoded.(type) {
	case *HandoverRequiredTransferDecoded:
		s.HandoverRequiredTransfer = v
	case *HandoverCommandTransferDecoded:
		s.HandoverCommandTransfer = v
	case *HandoverRequestAcknowledgeTransferDecoded:
		s.HandoverRequestAcknowledgeTransfer = v
	case *HandoverResourceAllocationUnsuccessfulTransferDecoded:
		s.HandoverResourceAllocationUnsuccessfulTransfer = v
	case *PDUSessionResourceSetupRequestTransfer:
		s.SetupRequestTransfer = v
	case *CauseTransferDecoded:
		s.CauseTransfer = v
	}

	return s
}

func handoverTypeToEnum(h ngap.HandoverType) utils.EnumField {
	return utils.NamedEnum(uint8(h), h.Name())
}

func tacsToStrings(list []ngap.TAC) []string {
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, fmt.Sprintf("%06x", uint32(t)))
	}

	return out
}

// ratRestrictionNames are the RATs a restriction bitmap can bar, in transmission
// order (TS 38.413 §9.3.1.85).
var ratRestrictionNames = []struct {
	bit  ngap.RATRestrictionInformation
	name string
}{
	{ngap.RATRestrictionEUTRA, "e-UTRA"},
	{ngap.RATRestrictionNR, "nR"},
	{ngap.RATRestrictionNRUnlicensed, "nR-unlicensed"},
	{ngap.RATRestrictionNRLEO, "nR-LEO"},
	{ngap.RATRestrictionNRMEO, "nR-MEO"},
	{ngap.RATRestrictionNRGEO, "nR-GEO"},
	{ngap.RATRestrictionNROtherSat, "nR-OTHERSAT"},
}

func ratRestrictionInformation(v ngap.RATRestrictionInformation) string {
	var restricted []string

	for _, r := range ratRestrictionNames {
		if v&r.bit != 0 {
			restricted = append(restricted, r.name)
		}
	}

	if len(restricted) == 0 {
		return "none"
	}

	out := restricted[0]
	for _, r := range restricted[1:] {
		out += "," + r
	}

	return out
}

func mobilityRestrictionList(l ngap.MobilityRestrictionList) MobilityRestrictionList {
	out := MobilityRestrictionList{ServingPLMN: plmnIDToDecoder(l.ServingPLMN)}

	for _, p := range l.EquivalentPLMNs {
		out.EquivalentPLMNs = append(out.EquivalentPLMNs, plmnIDToDecoder(p))
	}

	for _, r := range l.RATRestrictions {
		out.RATRestrictions = append(out.RATRestrictions, RATRestriction{
			PLMNID:                    plmnIDToDecoder(r.PLMNIdentity),
			RATRestrictionInformation: ratRestrictionInformation(r.RATRestrictionInformation),
		})
	}

	for _, f := range l.ForbiddenAreaInformation {
		out.ForbiddenAreaInformation = append(out.ForbiddenAreaInformation, ForbiddenAreaInformation{
			PLMNID:        plmnIDToDecoder(f.PLMNIdentity),
			ForbiddenTACs: tacsToStrings(f.ForbiddenTACs),
		})
	}

	for _, a := range l.ServiceAreaInformation {
		out.ServiceAreaInformation = append(out.ServiceAreaInformation, ServiceAreaInformation{
			PLMNID:         plmnIDToDecoder(a.PLMNIdentity),
			AllowedTACs:    tacsToStrings(a.AllowedTACs),
			NotAllowedTACs: tacsToStrings(a.NotAllowedTACs),
		})
	}

	return out
}

func targetID(t ngap.TargetID) TargetID {
	var out TargetID

	if t.TargetRANNodeID != nil {
		selected := tai(t.TargetRANNodeID.SelectedTAI)
		out.TargetRANNodeID = &TargetRANNodeID{
			GlobalRANNodeID: buildGlobalRANNodeID(t.TargetRANNodeID.GlobalRANNodeID),
			SelectedTAI:     selected,
		}
	}

	if t.TargeteNBID != nil {
		out.TargetENBID = &TargetENBID{
			GlobalNgENBID: GlobalNgENBID{
				PLMNIdentity: plmnIDToDecoder(t.TargeteNBID.GlobalENBID.PLMNIdentity),
				NgENBID:      fmt.Sprintf("%x", t.TargeteNBID.GlobalENBID.NgENBID.Value),
			},
			SelectedEPSTAI: EPSTAI{
				PLMNIdentity: plmnIDToDecoder(t.TargeteNBID.SelectedEPSTAI.PLMNIdentity),
				TAC:          fmt.Sprintf("%04x", uint16(t.TargeteNBID.SelectedEPSTAI.TAC)),
			},
		}
	}

	return out
}

func buildHandoverRequired(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverRequired(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Required: %v", err)}
	}

	sessions := make([]HandoverSession, 0, len(m.PDUSessionResourceListHORqd))
	for _, it := range m.PDUSessionResourceListHORqd {
		sessions = append(sessions, handoverSession(it.PDUSessionID, it.Transfer, libHandoverRequiredTransfer))
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDHandoverType, ngap.CriticalityReject, handoverTypeToEnum(m.HandoverType)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = append(ies, ie(ngap.IDTargetID, ngap.CriticalityReject, targetID(m.TargetID)))

	if m.DirectForwardingPathAvailability != nil {
		available := *m.DirectForwardingPathAvailability == ngap.DirectForwardingPathAvailable
		ies = append(ies, ie(ngap.IDDirectForwardingPathAvailability, ngap.CriticalityIgnore, available))
	}

	ies = append(ies,
		ie(ngap.IDPDUSessionResourceListHORqd, ngap.CriticalityReject, sessions),
		ie(ngap.IDSourceToTargetTransparentContainer, ngap.CriticalityReject, hex.EncodeToString(m.SourceToTargetTransparentContainer)),
	)

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverCommand(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverCommand(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Command: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDHandoverType, ngap.CriticalityReject, handoverTypeToEnum(m.HandoverType)),
	}

	if len(m.NASSecurityParametersFromNGRAN) > 0 {
		ies = append(ies, ie(ngap.IDNASSecurityParametersFromNGRAN, ngap.CriticalityReject, hex.EncodeToString(m.NASSecurityParametersFromNGRAN)))
	}

	if len(m.PDUSessionResourceHandoverList) > 0 {
		sessions := make([]HandoverSession, 0, len(m.PDUSessionResourceHandoverList))
		for _, it := range m.PDUSessionResourceHandoverList {
			sessions = append(sessions, handoverSession(it.PDUSessionID, it.Transfer, libHandoverCommandTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceHandoverList, ngap.CriticalityIgnore, sessions))
	}

	if len(m.PDUSessionResourceToReleaseList) > 0 {
		released := make([]HandoverSession, 0, len(m.PDUSessionResourceToReleaseList))
		for _, it := range m.PDUSessionResourceToReleaseList {
			released = append(released, handoverSession(it.PDUSessionID, it.Transfer, libHandoverPreparationUnsuccessfulTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceToReleaseListHOCmd, ngap.CriticalityIgnore, released))
	}

	ies = append(ies, ie(ngap.IDTargetToSourceTransparentContainer, ngap.CriticalityReject, hex.EncodeToString(m.TargetToSourceTransparentContainer)))

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverPreparationFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverPreparationFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Preparation Failure: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	if len(m.TargettoSourceFailureTransparentContainer) > 0 {
		ies = append(ies, ie(ngap.IDTargettoSourceFailureTransparentContainer, ngap.CriticalityIgnore, hex.EncodeToString(m.TargettoSourceFailureTransparentContainer)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Request: %v", err)}
	}

	sessions := make([]HandoverSession, 0, len(m.PDUSessionResourceSetupListHOReq))

	for _, it := range m.PDUSessionResourceSetupListHOReq {
		s := handoverSession(it.PDUSessionID, it.Transfer, libHandoverSetupRequestTransfer)
		snssai := buildSNSSAIValue(it.SNSSAI)
		s.SNSSAI = &snssai
		sessions = append(sessions, s)
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDHandoverType, ngap.CriticalityReject, handoverTypeToEnum(m.HandoverType)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	ies = append(ies,
		ie(ngap.IDUEAggregateMaximumBitRate, ngap.CriticalityReject, MaximumBitRate{
			UplinkNAggregateMaximumBitRate:   uint64(m.UEAggregateMaximumBitRate.UL),
			DownlinkNAggregateMaximumBitRate: uint64(m.UEAggregateMaximumBitRate.DL),
			Unit:                             "bps",
		}),
		ie(ngap.IDUESecurityCapabilities, ngap.CriticalityReject, libUESecurityCapabilities(m.UESecurityCapabilities)),
		ie(ngap.IDSecurityContext, ngap.CriticalityReject, SecurityContext{
			NextHopChainingCount: int64(m.SecurityContext.NextHopChainingCount),
			NextHopNH:            hex.EncodeToString(m.SecurityContext.NextHopNH[:]),
		}),
	)

	if m.NewSecurityContextInd != nil {
		ies = append(ies, ie(ngap.IDNewSecurityContextInd, ngap.CriticalityReject, true))
	}

	if len(m.NASC) > 0 {
		ies = append(ies, ie(ngap.IDNASC, ngap.CriticalityReject, libNASPDU(m.NASC)))
	}

	ies = append(ies, ie(ngap.IDPDUSessionResourceSetupListHOReq, ngap.CriticalityReject, sessions))

	if len(m.AllowedNSSAI) > 0 {
		allowed := make([]SNSSAI, 0, len(m.AllowedNSSAI))
		for _, it := range m.AllowedNSSAI {
			allowed = append(allowed, buildSNSSAIValue(it.SNSSAI))
		}

		ies = append(ies, ie(ngap.IDAllowedNSSAI, ngap.CriticalityReject, allowed))
	}

	ies = append(ies, ie(ngap.IDSourceToTargetTransparentContainer, ngap.CriticalityReject, hex.EncodeToString(m.SourceToTargetTransparentContainer)))

	if m.MobilityRestrictionList != nil {
		ies = append(ies, ie(ngap.IDMobilityRestrictionList, ngap.CriticalityIgnore, mobilityRestrictionList(*m.MobilityRestrictionList)))
	}

	ies = append(ies, ie(ngap.IDGUAMI, ngap.CriticalityReject, guami(m.GUAMI)))

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverRequestAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Request Acknowledge: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if len(m.PDUSessionResourceAdmittedList) > 0 {
		admitted := make([]HandoverSession, 0, len(m.PDUSessionResourceAdmittedList))
		for _, it := range m.PDUSessionResourceAdmittedList {
			admitted = append(admitted, handoverSession(it.PDUSessionID, it.Transfer, libHandoverRequestAcknowledgeTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceAdmittedList, ngap.CriticalityIgnore, admitted))
	}

	if len(m.PDUSessionResourceFailedToSetup) > 0 {
		failed := make([]HandoverSession, 0, len(m.PDUSessionResourceFailedToSetup))
		for _, it := range m.PDUSessionResourceFailedToSetup {
			failed = append(failed, handoverSession(it.PDUSessionID, it.Transfer, libHandoverResourceAllocationUnsuccessfulTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceFailedToSetupListHOAck, ngap.CriticalityIgnore, failed))
	}

	ies = append(ies, ie(ngap.IDTargetToSourceTransparentContainer, ngap.CriticalityReject, hex.EncodeToString(m.TargetToSourceTransparentContainer)))

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Failure: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	if len(m.TargettoSourceFailureTransparentContainer) > 0 {
		ies = append(ies, ie(ngap.IDTargettoSourceFailureTransparentContainer, ngap.CriticalityIgnore, hex.EncodeToString(m.TargettoSourceFailureTransparentContainer)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverNotify(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverNotify(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Notify: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	if m.NotifySourceNGRANNode != nil {
		ies = append(ies, ie(ngap.IDNotifySourceNGRANNode, ngap.CriticalityIgnore, true))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverCancel(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverCancel(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Cancel: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildHandoverCancelAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParseHandoverCancelAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Handover Cancel Acknowledge: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
