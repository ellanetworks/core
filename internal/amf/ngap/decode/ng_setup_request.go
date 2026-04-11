// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// DecodeNGSetupRequest validates an NGSetupRequest PDU body
// (3GPP TS 38.413 §9.2.6.1). Mandatory IEs are GlobalRANNodeID,
// SupportedTAList and DefaultPagingDRX (all reject for the procedure,
// reject criticality on the IE itself for GlobalRANNodeID and
// SupportedTAList, ignore for DefaultPagingDRX). RANNodeName is
// optional. Duplicate IEs follow a last-wins policy.
//
// DefaultPagingDRX is validated for presence and structural validity
// but not stored: no current handler consumes the value.
func DecodeNGSetupRequest(in *ngapType.NGSetupRequest) (NGSetupRequest, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeNGSetup,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out NGSetupRequest

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveGlobalRANNodeID bool
		haveSupportedTAList bool
		havePagingDRX       bool
	)

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDGlobalRANNodeID:
			haveGlobalRANNodeID = true

			id, ok := decodeGlobalRANNodeID(ie.Value.GlobalRANNodeID)
			if !ok {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.GlobalRANNodeID = id

		case ngapType.ProtocolIEIDSupportedTAList:
			haveSupportedTAList = true

			if ie.Value.SupportedTAList == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.SupportedTAItems = ie.Value.SupportedTAList.List

		case ngapType.ProtocolIEIDDefaultPagingDRX:
			havePagingDRX = true

			if ie.Value.DefaultPagingDRX == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

		case ngapType.ProtocolIEIDRANNodeName:
			if ie.Value.RANNodeName == nil {
				continue
			}

			out.RANNodeName = ie.Value.RANNodeName.Value
		}
	}

	if !haveGlobalRANNodeID {
		report.MissingMandatory(ngapType.ProtocolIEIDGlobalRANNodeID, ngapType.CriticalityPresentReject)
	}

	if !haveSupportedTAList {
		report.MissingMandatory(ngapType.ProtocolIEIDSupportedTAList, ngapType.CriticalityPresentReject)
	}

	if !havePagingDRX {
		report.MissingMandatory(ngapType.ProtocolIEIDDefaultPagingDRX, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
