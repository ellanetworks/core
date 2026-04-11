// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// ErrorIndication ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {ErrorIndicationIEs} },
//  ...
// }
// ErrorIndicationIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID            CRITICALITY ignore TYPE AMF-UE-NGAP-ID            PRESENCE optional }|
//  { ID id-RAN-UE-NGAP-ID            CRITICALITY ignore TYPE RAN-UE-NGAP-ID            PRESENCE optional }|
//  { ID id-Cause                     CRITICALITY ignore TYPE Cause                     PRESENCE optional }|
//  { ID id-CriticalityDiagnostics    CRITICALITY ignore TYPE CriticalityDiagnostics    PRESENCE optional },
//  ...
// }

// DecodeErrorIndication validates an ErrorIndication PDU body (3GPP TS
// 38.413 §9.2.7.2). All four IEs are optional-ignore. AMF-UE-NGAP-ID
// and RAN-UE-NGAP-ID are validated structurally but not surfaced — no
// current handler reads them. The spec also requires that at least one
// of Cause or CriticalityDiagnostics be present; the decoder leaves
// that semantic check to the handler. The procedure is class 2, so the
// procedure-level criticality is "ignore". Duplicate IEs follow a
// last-wins policy.
func DecodeErrorIndication(in *ngapType.ErrorIndication) (ErrorIndication, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeErrorIndication,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out ErrorIndication

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			if ie.Value.AMFUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

		case ngapType.ProtocolIEIDRANUENGAPID:
			if ie.Value.RANUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

		case ngapType.ProtocolIEIDCause:
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

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
