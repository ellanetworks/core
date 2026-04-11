// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// HandoverFailure ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {HandoverFailureIEs} },
//  ...
// }
// HandoverFailureIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID            CRITICALITY ignore TYPE AMF-UE-NGAP-ID            PRESENCE mandatory }|
//  { ID id-Cause                     CRITICALITY ignore TYPE Cause                     PRESENCE mandatory }|
//  { ID id-CriticalityDiagnostics    CRITICALITY ignore TYPE CriticalityDiagnostics    PRESENCE optional  },
//  ...
// }

// DecodeHandoverFailure validates a HandoverFailure PDU body (3GPP TS
// 38.413 §9.2.3.3). AMFUENGAPID and Cause are mandatory-ignore;
// CriticalityDiagnostics is optional-ignore. The procedure is class 1,
// so the procedure-level criticality is "reject". A missing or malformed
// AMF-UE-NGAP-ID yields a non-fatal report with a zero ID — the handler
// will fail to locate the UE and emit an error indication. Duplicate
// IEs follow a last-wins policy.
//
// CriticalityDiagnostics aliases the source PDU buffer; the handler
// forwards it to the source gNB on HandoverPreparationFailure.
func DecodeHandoverFailure(in *ngapType.HandoverFailure) (HandoverFailure, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeHandoverResourceAllocation,
		TriggeringMessage:    ngapType.TriggeringMessagePresentUnsuccessfullOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out HandoverFailure

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveCause       bool
	)

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			haveAMFUENGAPID = true

			if ie.Value.AMFUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.AMFUENGAPID = ie.Value.AMFUENGAPID.Value

		case ngapType.ProtocolIEIDCause:
			haveCause = true

			if ie.Value.Cause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.Cause = ie.Value.Cause

		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			if ie.Value.CriticalityDiagnostics == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.CriticalityDiagnostics = ie.Value.CriticalityDiagnostics
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
