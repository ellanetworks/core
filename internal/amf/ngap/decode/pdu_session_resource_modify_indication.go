// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// PDUSessionResourceModifyIndication ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {PDUSessionResourceModifyIndicationIEs} },
//  ...
// }
// PDUSessionResourceModifyIndicationIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                          CRITICALITY reject TYPE AMF-UE-NGAP-ID                          PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                          CRITICALITY reject TYPE RAN-UE-NGAP-ID                          PRESENCE mandatory }|
//  { ID id-PDUSessionResourceModifyListModInd      CRITICALITY reject TYPE PDUSessionResourceModifyListModInd      PRESENCE mandatory }|
//  { ID id-UserLocationInformation                 CRITICALITY ignore TYPE UserLocationInformation                 PRESENCE optional  },
//  ...
// }

// DecodePDUSessionResourceModifyIndication validates a
// PDUSessionResourceModifyIndication PDU body (3GPP TS 38.413 §9.2.1.6).
// AMFUENGAPID, RANUENGAPID and PDUSessionResourceModifyListModInd are
// mandatory-reject so missing or malformed values produce a fatal
// report. The PDUSessionResourceModifyListModInd IE is validated for
// presence only — the handler does not consume its contents. The
// optional UserLocationInformation IE is not decoded (no consumer).
// Duplicate IEs follow a last-wins policy.
func DecodePDUSessionResourceModifyIndication(in *ngapType.PDUSessionResourceModifyIndication) (PDUSessionResourceModifyIndication, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodePDUSessionResourceModifyIndication,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out PDUSessionResourceModifyIndication

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
		haveModifyList  bool
	)

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			haveAMFUENGAPID = true

			if ie.Value.AMFUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.AMFUENGAPID = ie.Value.AMFUENGAPID.Value

		case ngapType.ProtocolIEIDRANUENGAPID:
			haveRANUENGAPID = true

			if ie.Value.RANUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.RANUENGAPID = ie.Value.RANUENGAPID.Value

		case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd:
			haveModifyList = true

			if ie.Value.PDUSessionResourceModifyListModInd == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveModifyList {
		report.MissingMandatory(ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd, ngapType.CriticalityPresentReject)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
