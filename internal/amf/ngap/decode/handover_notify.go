// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// HandoverNotify ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {HandoverNotifyIEs} },
//  ...
// }
// HandoverNotifyIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID          CRITICALITY reject TYPE AMF-UE-NGAP-ID          PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID          CRITICALITY reject TYPE RAN-UE-NGAP-ID          PRESENCE mandatory }|
//  { ID id-UserLocationInformation CRITICALITY ignore TYPE UserLocationInformation PRESENCE mandatory },
//  ...
// }

// DecodeHandoverNotify validates a HandoverNotify PDU body (3GPP TS
// 38.413). Class 2 procedure: procedure-level criticality is ignore.
func DecodeHandoverNotify(in *ngapType.HandoverNotify) (HandoverNotify, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeHandoverNotification,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out HandoverNotify

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
		haveULI         bool
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

		case ngapType.ProtocolIEIDUserLocationInformation:
			haveULI = true

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

	if !haveULI {
		report.MissingMandatory(ngapType.ProtocolIEIDUserLocationInformation, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
