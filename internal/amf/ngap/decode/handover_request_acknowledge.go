// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// HandoverRequestAcknowledge ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {HandoverRequestAcknowledgeIEs} },
//  ...
// }
// HandoverRequestAcknowledgeIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                              CRITICALITY ignore TYPE AMF-UE-NGAP-ID                              PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                              CRITICALITY ignore TYPE RAN-UE-NGAP-ID                              PRESENCE mandatory }|
//  { ID id-PDUSessionResourceAdmittedList              CRITICALITY ignore TYPE PDUSessionResourceAdmittedList              PRESENCE mandatory }|
//  { ID id-PDUSessionResourceFailedToSetupListHOAck    CRITICALITY ignore TYPE PDUSessionResourceFailedToSetupListHOAck    PRESENCE optional  }|
//  { ID id-TargetToSource-TransparentContainer         CRITICALITY reject TYPE TargetToSource-TransparentContainer         PRESENCE mandatory }|
//  { ID id-CriticalityDiagnostics                      CRITICALITY ignore TYPE CriticalityDiagnostics                      PRESENCE optional  },
//  ...
// }

// DecodeHandoverRequestAcknowledge validates a HandoverRequestAcknowledge
// PDU body (3GPP TS 38.413 §9.2.3.3 / Handover Resource Allocation).
// AMFUENGAPID, RANUENGAPID and PDUSessionResourceAdmittedList are
// mandatory-ignore (criticality-ignore per the spec). AMFUENGAPID and
// RANUENGAPID are surfaced as pointers because the handler differentiates
// absent vs present and 0 is a valid NGAP UE NGAP ID.
// TargetToSourceTransparentContainer is mandatory-reject; missing or
// malformed values produce a fatal report so the dispatcher returns an
// ErrorIndication carrying CriticalityDiagnostics. PDUSessionResourceFailedToSetupItems
// is optional-ignore. Duplicate IEs follow a last-wins policy.
func DecodeHandoverRequestAcknowledge(in *ngapType.HandoverRequestAcknowledge) (HandoverRequestAcknowledge, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeHandoverResourceAllocation,
		TriggeringMessage:    ngapType.TriggeringMessagePresentSuccessfulOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out HandoverRequestAcknowledge

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID  bool
		haveRANUENGAPID  bool
		haveAdmittedList bool
		haveTgtToSrcCont bool
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

		case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:
			haveAdmittedList = true

			if ie.Value.PDUSessionResourceAdmittedList == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.AdmittedItems = ie.Value.PDUSessionResourceAdmittedList.List

		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck:
			if ie.Value.PDUSessionResourceFailedToSetupListHOAck == nil {
				continue
			}

			out.FailedToSetupItems = ie.Value.PDUSessionResourceFailedToSetupListHOAck.List

		case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
			haveTgtToSrcCont = true

			if ie.Value.TargetToSourceTransparentContainer == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.TargetToSourceTransparentContainer = *ie.Value.TargetToSourceTransparentContainer
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveAdmittedList {
		report.MissingMandatory(ngapType.ProtocolIEIDPDUSessionResourceAdmittedList, ngapType.CriticalityPresentIgnore)
	}

	if !haveTgtToSrcCont {
		report.MissingMandatory(ngapType.ProtocolIEIDTargetToSourceTransparentContainer, ngapType.CriticalityPresentReject)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
