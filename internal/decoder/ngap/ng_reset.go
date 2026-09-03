// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/ngap"
)

// UEAssociatedLogicalNGConnection names one UE-associated logical NG connection
// (TS 38.413 §9.3.3.25). Either identity may be absent: the sender names the
// connection with whichever ids it holds.
type UEAssociatedLogicalNGConnection struct {
	AMFUENGAPID *int64 `json:"amf_ue_ngap_id,omitempty"`
	RANUENGAPID *int64 `json:"ran_ue_ngap_id,omitempty"`
}

// ResetType selects what the NG RESET covers (TS 38.413 §9.2.6.11): the whole NG
// interface, or the listed UE-associated logical NG connections.
type ResetType struct {
	NGInterface bool                              `json:"ng_interface,omitempty"`
	PartOfNG    []UEAssociatedLogicalNGConnection `json:"part_of_ng_interface,omitempty"`
}

func ueAssociatedConnections(items ngap.UEAssociatedLogicalNGConnectionList) []UEAssociatedLogicalNGConnection {
	out := make([]UEAssociatedLogicalNGConnection, 0, len(items))

	for _, it := range items {
		var c UEAssociatedLogicalNGConnection

		if it.AMFUENGAPID != nil {
			v := int64(*it.AMFUENGAPID)
			c.AMFUENGAPID = &v
		}

		if it.RANUENGAPID != nil {
			v := int64(*it.RANUENGAPID)
			c.RANUENGAPID = &v
		}

		out = append(out, c)
	}

	return out
}

func buildNGReset(value []byte) NGAPMessageValue {
	m, err := ngap.ParseNGReset(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse NG Reset: %v", err)}
	}

	var ies []IE

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	rt := ResetType{NGInterface: m.ResetType.All}
	if !m.ResetType.All {
		rt.PartOfNG = ueAssociatedConnections(m.ResetType.Part)
	}

	ies = append(ies, ie(ngap.IDResetType, ngap.CriticalityReject, rt))

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildNGResetAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParseNGResetAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse NG Reset Acknowledge: %v", err)}
	}

	var ies []IE

	if len(m.ConnectionList) > 0 {
		ies = append(ies, ie(ngap.IDUEAssociatedLogicalNGConnectionList, ngap.CriticalityIgnore, ueAssociatedConnections(m.ConnectionList)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
