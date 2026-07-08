// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// PDUSessionResourceNotify ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {PDUSessionResourceNotifyIEs} },
//  ...
// }
// PDUSessionResourceNotifyIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                       CRITICALITY reject TYPE AMF-UE-NGAP-ID                       PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                       CRITICALITY reject TYPE RAN-UE-NGAP-ID                       PRESENCE mandatory }|
//  { ID id-PDUSessionResourceNotifyList         CRITICALITY reject TYPE PDUSessionResourceNotifyList         PRESENCE optional  }|
//  { ID id-PDUSessionResourceReleasedListNot    CRITICALITY ignore TYPE PDUSessionResourceReleasedListNot    PRESENCE optional  }|
//  { ID id-UserLocationInformation              CRITICALITY ignore TYPE UserLocationInformation              PRESENCE optional  },
//  ...
// }

// DecodePDUSessionResourceNotify validates a PDUSessionResourceNotify
// PDU body (3GPP TS 38.413).
func DecodePDUSessionResourceNotify(in *ngapType.PDUSessionResourceNotify) (PDUSessionResourceNotify, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodePDUSessionResourceNotify,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out PDUSessionResourceNotify

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

		case ngapType.ProtocolIEIDPDUSessionResourceNotifyList:
			if ie.Value.PDUSessionResourceNotifyList == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.HasNotifyList = true

		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot:
			if ie.Value.PDUSessionResourceReleasedListNot == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.PDUSessionResourceReleasedItems = ie.Value.PDUSessionResourceReleasedListNot.List

		case ngapType.ProtocolIEIDUserLocationInformation:
			if ie.Value.UserLocationInformation == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UserLocationInformation = ie.Value.UserLocationInformation
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
