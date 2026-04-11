// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// PDUSessionResourceSetupResponse ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {PDUSessionResourceSetupResponseIEs} },
//  ...
// }
// PDUSessionResourceSetupResponseIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                              CRITICALITY ignore TYPE AMF-UE-NGAP-ID                              PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                              CRITICALITY ignore TYPE RAN-UE-NGAP-ID                              PRESENCE mandatory }|
//  { ID id-PDUSessionResourceSetupListSURes            CRITICALITY ignore TYPE PDUSessionResourceSetupListSURes            PRESENCE optional  }|
//  { ID id-PDUSessionResourceFailedToSetupListSURes    CRITICALITY ignore TYPE PDUSessionResourceFailedToSetupListSURes    PRESENCE optional  }|
//  { ID id-CriticalityDiagnostics                      CRITICALITY ignore TYPE CriticalityDiagnostics                      PRESENCE optional  },
//  ...
// }

// DecodePDUSessionResourceSetupResponse validates a
// PDUSessionResourceSetupResponse PDU body (3GPP TS 38.413 §9.2.1.2).
// All IEs are criticality-ignore. AMFUENGAPID and RANUENGAPID are
// mandatory-ignore and surfaced as pointers (handler does conditional
// fallback lookups). SetupItems and FailedToSetupItems are optional.
// Duplicate IEs follow a last-wins policy.
func DecodePDUSessionResourceSetupResponse(in *ngapType.PDUSessionResourceSetupResponse) (PDUSessionResourceSetupResponse, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodePDUSessionResourceSetup,
		TriggeringMessage:    ngapType.TriggeringMessagePresentSuccessfulOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out PDUSessionResourceSetupResponse

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
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

		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
			if ie.Value.PDUSessionResourceSetupListSURes == nil {
				continue
			}

			out.SetupItems = ie.Value.PDUSessionResourceSetupListSURes.List

		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
			if ie.Value.PDUSessionResourceFailedToSetupListSURes == nil {
				continue
			}

			out.FailedToSetupItems = ie.Value.PDUSessionResourceFailedToSetupListSURes.List
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
