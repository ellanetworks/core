// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// PDUSessionResourceReleaseResponse ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {PDUSessionResourceReleaseResponseIEs} },
//  ...
// }
// PDUSessionResourceReleaseResponseIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                          CRITICALITY ignore TYPE AMF-UE-NGAP-ID                          PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                          CRITICALITY ignore TYPE RAN-UE-NGAP-ID                          PRESENCE mandatory }|
//  { ID id-PDUSessionResourceReleasedListRelRes    CRITICALITY ignore TYPE PDUSessionResourceReleasedListRelRes    PRESENCE mandatory }|
//  { ID id-UserLocationInformation                 CRITICALITY ignore TYPE UserLocationInformation                 PRESENCE optional  }|
//  { ID id-CriticalityDiagnostics                  CRITICALITY ignore TYPE CriticalityDiagnostics                  PRESENCE optional  },
//  ...
// }

// DecodePDUSessionResourceReleaseResponse validates a
// PDUSessionResourceReleaseResponse PDU body (3GPP TS 38.413 §9.2.1.5).
// All IEs are criticality-ignore. AMFUENGAPID, RANUENGAPID and
// PDUSessionResourceReleasedListRelRes are mandatory-ignore so a
// missing or malformed value yields a non-fatal report. AMFUENGAPID
// and RANUENGAPID are surfaced as pointers so the handler can
// nil-check before driving lookups. UserLocationInformation is
// optional. CriticalityDiagnostics is consumed for validation only.
// Duplicate IEs follow a last-wins policy.
func DecodePDUSessionResourceReleaseResponse(in *ngapType.PDUSessionResourceReleaseResponse) (PDUSessionResourceReleaseResponse, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodePDUSessionResourceRelease,
		TriggeringMessage:    ngapType.TriggeringMessagePresentSuccessfulOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out PDUSessionResourceReleaseResponse

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID    bool
		haveRANUENGAPID    bool
		haveReleasedListRR bool
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

		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
			haveReleasedListRR = true

			if ie.Value.PDUSessionResourceReleasedListRelRes == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.PDUSessionResourceReleasedItems = ie.Value.PDUSessionResourceReleasedListRelRes.List

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

	if !haveReleasedListRR {
		report.MissingMandatory(ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
