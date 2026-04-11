// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UEContextModificationFailure ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UEContextModificationFailureIEs} },
//  ...
// }
// UEContextModificationFailureIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID            CRITICALITY ignore TYPE AMF-UE-NGAP-ID            PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID            CRITICALITY ignore TYPE RAN-UE-NGAP-ID            PRESENCE mandatory }|
//  { ID id-Cause                     CRITICALITY ignore TYPE Cause                     PRESENCE mandatory }|
//  { ID id-CriticalityDiagnostics    CRITICALITY ignore TYPE CriticalityDiagnostics    PRESENCE optional  },
//  ...
// }

// DecodeUEContextModificationFailure validates a UEContextModificationFailure
// PDU body (3GPP TS 38.413 §9.2.2.8). All four IEs are mandatory-ignore /
// optional-ignore, so the decoder records diagnostics in *Report but never
// raises a fatal error. The procedure is class 1, so the procedure-level
// criticality is "reject". Duplicate IEs follow a last-wins policy.
//
// AMFUENGAPID and RANUENGAPID are exposed as pointers because zero is a
// valid NGAP UE NGAP ID and the handler distinguishes "absent" from
// "present" to drive a fallback lookup.
func DecodeUEContextModificationFailure(in *ngapType.UEContextModificationFailure) (UEContextModificationFailure, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUEContextModification,
		TriggeringMessage:    ngapType.TriggeringMessagePresentUnsuccessfullOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out UEContextModificationFailure

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
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

			v := ie.Value.AMFUENGAPID.Value
			out.AMFUENGAPID = &v

		case ngapType.ProtocolIEIDRANUENGAPID:
			haveRANUENGAPID = true

			if ie.Value.RANUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			v := ie.Value.RANUENGAPID.Value
			out.RANUENGAPID = &v

		case ngapType.ProtocolIEIDCause:
			haveCause = true

			if ie.Value.Cause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.Cause = ie.Value.Cause
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
