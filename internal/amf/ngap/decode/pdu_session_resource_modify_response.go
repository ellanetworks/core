// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// PDUSessionResourceModifyResponse ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {PDUSessionResourceModifyResponseIEs} },
//  ...
// }
// PDUSessionResourceModifyResponseIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                                CRITICALITY ignore TYPE AMF-UE-NGAP-ID                                PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                                CRITICALITY ignore TYPE RAN-UE-NGAP-ID                                PRESENCE mandatory }|
//  { ID id-PDUSessionResourceModifyListModRes            CRITICALITY ignore TYPE PDUSessionResourceModifyListModRes            PRESENCE optional  }|
//  { ID id-PDUSessionResourceFailedToModifyListModRes    CRITICALITY ignore TYPE PDUSessionResourceFailedToModifyListModRes    PRESENCE optional  }|
//  { ID id-UserLocationInformation                       CRITICALITY ignore TYPE UserLocationInformation                       PRESENCE optional  }|
//  { ID id-CriticalityDiagnostics                        CRITICALITY ignore TYPE CriticalityDiagnostics                        PRESENCE optional  },
//  ...
// }

// DecodePDUSessionResourceModifyResponse validates a
// PDUSessionResourceModifyResponse PDU body (3GPP TS 38.413 §9.2.1.4).
// All IEs are criticality-ignore. AMFUENGAPID and RANUENGAPID are
// mandatory-ignore and surfaced as pointers (handler does conditional
// fallback lookups). UserLocationInformation is optional. The
// per-PDU-session lists are not surfaced because the current handler
// does not consume them. Duplicate IEs follow a last-wins policy.
func DecodePDUSessionResourceModifyResponse(in *ngapType.PDUSessionResourceModifyResponse) (PDUSessionResourceModifyResponse, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodePDUSessionResourceModify,
		TriggeringMessage:    ngapType.TriggeringMessagePresentSuccessfulOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out PDUSessionResourceModifyResponse

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

		case ngapType.ProtocolIEIDUserLocationInformation:
			if ie.Value.UserLocationInformation == nil {
				continue
			}

			out.UserLocationInformation = ie.Value.UserLocationInformation
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
