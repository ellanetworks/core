// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap"
)

// UEAssociatedLogicalS1Connection names one UE-associated logical S1 connection
// (TS 36.413 §9.1.8.1). Either identity may be absent: the sender names the
// connection with whichever ids it holds.
type UEAssociatedLogicalS1Connection struct {
	MMEUES1APID *uint32 `json:"mme_ue_s1ap_id,omitempty"`
	ENBUES1APID *uint32 `json:"enb_ue_s1ap_id,omitempty"`
}

// ResetType selects what the RESET covers (TS 36.413 §9.1.8.1): the whole S1
// interface, or the listed UE-associated logical S1 connections.
type ResetType struct {
	S1Interface bool                              `json:"s1_interface,omitempty"`
	PartOfS1    []UEAssociatedLogicalS1Connection `json:"part_of_s1_interface,omitempty"`
}

func ueAssociatedConnections(items []s1ap.UEAssociatedLogicalS1ConnectionItem) []UEAssociatedLogicalS1Connection {
	out := make([]UEAssociatedLogicalS1Connection, 0, len(items))

	for _, it := range items {
		var c UEAssociatedLogicalS1Connection

		if it.MMEUES1APID != nil {
			v := uint32(*it.MMEUES1APID)
			c.MMEUES1APID = &v
		}

		if it.ENBUES1APID != nil {
			v := uint32(*it.ENBUES1APID)
			c.ENBUES1APID = &v
		}

		out = append(out, c)
	}

	return out
}

func buildReset(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseReset(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Reset: %v", err)}, ""
	}

	var ies []IE

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	rt := ResetType{S1Interface: m.ResetType.All}
	if !m.ResetType.All {
		rt.PartOfS1 = ueAssociatedConnections(m.ResetType.Part)
	}

	ies = append(ies, ie(s1ap.IDResetType, s1ap.CriticalityReject, rt))
	ies = appendUnknownIEs(ies, m.UnknownIEs())

	scope := fmt.Sprintf("%d connection(s)", len(m.ResetType.Part))
	if m.ResetType.All {
		scope = "whole S1 interface"
	}

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Reset (%s)", scope)
}

func buildResetAcknowledge(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseResetAcknowledge(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Reset Acknowledge: %v", err)}, ""
	}

	var ies []IE

	if len(m.ConnectionList) > 0 {
		ies = append(ies, ie(s1ap.IDUEAssociatedLogicalS1ConnectionListResAck, s1ap.CriticalityIgnore, ueAssociatedConnections(m.ConnectionList)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Reset Acknowledge (%d connection(s))", len(m.ConnectionList))
}
