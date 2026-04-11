// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UEContextReleaseRequest ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UEContextReleaseRequest-IEs} },
//  ...
// }
// UEContextReleaseRequest-IEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                  CRITICALITY reject TYPE AMF-UE-NGAP-ID                  PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                  CRITICALITY reject TYPE RAN-UE-NGAP-ID                  PRESENCE mandatory }|
//  { ID id-PDUSessionResourceListCxtRelReq CRITICALITY reject TYPE PDUSessionResourceListCxtRelReq PRESENCE optional  }|
//  { ID id-Cause                           CRITICALITY ignore TYPE Cause                           PRESENCE mandatory },
//  ...
// }

// DecodeUEContextReleaseRequest validates a UEContextReleaseRequest PDU
// body (3GPP TS 38.413 §9.2.2.5). AMFUENGAPID and RANUENGAPID are
// mandatory-reject; Cause is mandatory-ignore; PDUSessionResourceList
// is optional-reject. Duplicate IEs follow a last-wins policy. The
// procedure is class 2 (no response), so the procedure-level
// criticality is "ignore".
func DecodeUEContextReleaseRequest(in *ngapType.UEContextReleaseRequest) (UEContextReleaseRequest, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUEContextReleaseRequest,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out UEContextReleaseRequest

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

		case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
			if ie.Value.PDUSessionResourceListCxtRelReq == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			items := ie.Value.PDUSessionResourceListCxtRelReq.List
			if items == nil {
				items = []ngapType.PDUSessionResourceItemCxtRelReq{}
			}

			out.PDUSessionResourceList = items

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
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
