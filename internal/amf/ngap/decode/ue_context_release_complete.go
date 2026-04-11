// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UEContextReleaseComplete ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UEContextReleaseComplete-IEs} },
//  ...
// }
// UEContextReleaseComplete-IEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                                CRITICALITY ignore TYPE AMF-UE-NGAP-ID                                PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                                CRITICALITY ignore TYPE RAN-UE-NGAP-ID                                PRESENCE mandatory }|
//  { ID id-UserLocationInformation                       CRITICALITY ignore TYPE UserLocationInformation                       PRESENCE optional  }|
//  { ID id-InfoOnRecommendedCellsAndRANNodesForPaging    CRITICALITY ignore TYPE InfoOnRecommendedCellsAndRANNodesForPaging    PRESENCE optional  }|
//  { ID id-PDUSessionResourceListCxtRelCpl               CRITICALITY reject TYPE PDUSessionResourceListCxtRelCpl               PRESENCE optional  }|
//  { ID id-CriticalityDiagnostics                        CRITICALITY ignore TYPE CriticalityDiagnostics                        PRESENCE optional  },
//  ...
// }

// DecodeUEContextReleaseComplete validates a UEContextReleaseComplete PDU
// body (3GPP TS 38.413 §9.2.2.4). AMFUENGAPID and RANUENGAPID are
// mandatory-ignore so a missing or malformed value yields a non-fatal
// report and a nil pointer in the decoded struct. The handler must
// nil-check before driving lookups. UserLocationInformation,
// InfoOnRecommendedCellsAndRANNodesForPaging and PDUSessionResourceList
// are optional. CriticalityDiagnostics is consumed for validation only.
// Duplicate IEs follow a last-wins policy.
func DecodeUEContextReleaseComplete(in *ngapType.UEContextReleaseComplete) (UEContextReleaseComplete, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUEContextRelease,
		TriggeringMessage:    ngapType.TriggeringMessagePresentSuccessfulOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out UEContextReleaseComplete

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

		case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
			if ie.Value.InfoOnRecommendedCellsAndRANNodesForPaging == nil {
				continue
			}

			out.InfoOnRecommendedCellsAndRANNodesForPaging = ie.Value.InfoOnRecommendedCellsAndRANNodesForPaging

		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
			if ie.Value.PDUSessionResourceListCxtRelCpl == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.PDUSessionResourceList = ie.Value.PDUSessionResourceListCxtRelCpl
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
