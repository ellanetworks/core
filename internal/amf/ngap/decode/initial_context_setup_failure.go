// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// InitialContextSetupFailure ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {InitialContextSetupFailureIEs} },
//  ...
// }
// InitialContextSetupFailureIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                                CRITICALITY ignore TYPE AMF-UE-NGAP-ID                                PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                                CRITICALITY ignore TYPE RAN-UE-NGAP-ID                                PRESENCE mandatory }|
//  { ID id-PDUSessionResourceFailedToSetupListCxtFail    CRITICALITY ignore TYPE PDUSessionResourceFailedToSetupListCxtFail   PRESENCE optional  }|
//  { ID id-Cause                                         CRITICALITY ignore TYPE Cause                                        PRESENCE mandatory }|
//  { ID id-CriticalityDiagnostics                        CRITICALITY ignore TYPE CriticalityDiagnostics                       PRESENCE optional  },
//  ...
// }

// DecodeInitialContextSetupFailure validates an InitialContextSetupFailure
// PDU body (3GPP TS 38.413 §9.2.2.3). AMFUENGAPID, RANUENGAPID and Cause
// are mandatory-ignore; PDUSessionResourceFailedToSetupListCxtFail and
// CriticalityDiagnostics are optional-ignore. The procedure is class 1,
// so the procedure-level criticality is "reject". A missing or malformed
// AMF/RAN-UE-NGAP-ID yields a non-fatal report with a zero ID — the
// handler will fail to locate the UE and log. Duplicate IEs follow a
// last-wins policy.
func DecodeInitialContextSetupFailure(in *ngapType.InitialContextSetupFailure) (InitialContextSetupFailure, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeInitialContextSetup,
		TriggeringMessage:    ngapType.TriggeringMessagePresentUnsuccessfullOutcome,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out InitialContextSetupFailure

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

		case ngapType.ProtocolIEIDCause:
			haveCause = true

			if ie.Value.Cause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.Cause = *ie.Value.Cause

		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
			if ie.Value.PDUSessionResourceFailedToSetupListCxtFail == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.PDUSessionResourceFailedToSetupItems = ie.Value.PDUSessionResourceFailedToSetupListCxtFail.List
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentIgnore)
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
