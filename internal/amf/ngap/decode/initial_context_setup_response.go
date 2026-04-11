// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// InitialContextSetupResponse ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {InitialContextSetupResponseIEs} },
//  ...
// }
// InitialContextSetupResponseIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                            CRITICALITY ignore TYPE AMF-UE-NGAP-ID                            PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                            CRITICALITY ignore TYPE RAN-UE-NGAP-ID                            PRESENCE mandatory }|
//  { ID id-PDUSessionResourceSetupListCxtRes         CRITICALITY ignore TYPE PDUSessionResourceSetupListCxtRes         PRESENCE optional  }|
//  { ID id-PDUSessionResourceFailedToSetupListCxtRes CRITICALITY ignore TYPE PDUSessionResourceFailedToSetupListCxtRes PRESENCE optional  }|
//  { ID id-CriticalityDiagnostics                    CRITICALITY ignore TYPE CriticalityDiagnostics                    PRESENCE optional  },
//  ...
// }

// DecodeInitialContextSetupResponse validates an InitialContextSetupResponse
// PDU body (3GPP TS 38.413 §9.2.2.2). AMFUENGAPID and RANUENGAPID are
// mandatory-ignore — every IE in this message is criticality ignore, so
// any structural problem yields a non-fatal report and the handler must
// still be invoked. The optional CriticalityDiagnostics IE is consumed
// only for log/debug purposes and is intentionally not surfaced.
// Duplicate IEs follow a last-wins policy.
func DecodeInitialContextSetupResponse(in *ngapType.InitialContextSetupResponse) (InitialContextSetupResponse, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeInitialContextSetup,
		TriggeringMessage:    ngapType.TriggeringMessagePresentSuccessfulOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out InitialContextSetupResponse

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

			out.AMFUENGAPID = ie.Value.AMFUENGAPID.Value

		case ngapType.ProtocolIEIDRANUENGAPID:
			haveRANUENGAPID = true

			if ie.Value.RANUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.RANUENGAPID = ie.Value.RANUENGAPID.Value

		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
			if ie.Value.PDUSessionResourceSetupListCxtRes == nil {
				continue
			}

			out.SetupItems = ie.Value.PDUSessionResourceSetupListCxtRes.List

		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
			if ie.Value.PDUSessionResourceFailedToSetupListCxtRes == nil {
				continue
			}

			out.FailedToSetupItems = ie.Value.PDUSessionResourceFailedToSetupListCxtRes.List
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
