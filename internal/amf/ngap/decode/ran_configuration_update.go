// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// RANConfigurationUpdate ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {RANConfigurationUpdate-IEs} },
//  ...
// }
// RANConfigurationUpdate-IEs NGAP-PROTOCOL-IES ::= {
//  { ID id-RANNodeName       CRITICALITY ignore TYPE RANNodeName       PRESENCE optional }|
//  { ID id-SupportedTAList   CRITICALITY reject TYPE SupportedTAList   PRESENCE optional }|
//  { ID id-DefaultPagingDRX  CRITICALITY ignore TYPE PagingDRX         PRESENCE optional }|
//  { ID id-GlobalRANNodeID   CRITICALITY ignore TYPE GlobalRANNodeID   PRESENCE optional },
//  ...
// }

// DecodeRANConfigurationUpdate validates a RANConfigurationUpdate PDU
// body (3GPP TS 38.413 §9.2.6.6). All IEs are optional, so the decoder
// never raises a missing-mandatory diagnostic; only malformed IEs are
// reported. SupportedTAList is optional-reject (a malformed value
// produces a fatal report); RANNodeName, DefaultPagingDRX and
// GlobalRANNodeID are optional-ignore. The procedure is class 1, so the
// procedure-level criticality is "reject". Duplicate IEs follow a
// last-wins policy.
func DecodeRANConfigurationUpdate(in *ngapType.RANConfigurationUpdate) (RANConfigurationUpdate, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeRANConfigurationUpdate,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out RANConfigurationUpdate

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANNodeName:
			if ie.Value.RANNodeName == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

		case ngapType.ProtocolIEIDSupportedTAList:
			if ie.Value.SupportedTAList == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			items := ie.Value.SupportedTAList.List
			if items == nil {
				items = []ngapType.SupportedTAItem{}
			}

			out.SupportedTAItems = items

		case ngapType.ProtocolIEIDDefaultPagingDRX:
			if ie.Value.DefaultPagingDRX == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

		case ngapType.ProtocolIEIDGlobalRANNodeID:
			if ie.Value.GlobalRANNodeID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}
		}
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
