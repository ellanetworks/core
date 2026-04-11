// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// HandoverCancel ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {HandoverCancelIEs} },
//  ...
// }
// HandoverCancelIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID CRITICALITY reject TYPE AMF-UE-NGAP-ID PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID CRITICALITY reject TYPE RAN-UE-NGAP-ID PRESENCE mandatory }|
//  { ID id-Cause          CRITICALITY ignore TYPE Cause          PRESENCE mandatory },
//  ...
// }

// DecodeHandoverCancel validates a HandoverCancel PDU body (3GPP TS
// 38.413 §9.2.3.4). AMFUENGAPID and RANUENGAPID are mandatory-reject;
// Cause is mandatory-ignore. The procedure is class 1, so the
// procedure-level criticality is "reject". Duplicate IEs follow a
// last-wins policy.
func DecodeHandoverCancel(in *ngapType.HandoverCancel) (HandoverCancel, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeHandoverCancel,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out HandoverCancel

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
