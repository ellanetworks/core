// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// NGReset ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {NGResetIEs} },
//  ...
// }
// NGResetIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-Cause     CRITICALITY ignore TYPE Cause     PRESENCE mandatory }|
//  { ID id-ResetType CRITICALITY reject TYPE ResetType PRESENCE mandatory },
//  ...
// }

// DecodeNGReset validates an NGReset PDU body (3GPP TS 38.413 §9.2.6.10).
// Cause is mandatory-ignore and ResetType is mandatory-reject. The
// procedure is class 1, so the procedure-level criticality is "reject".
// Duplicate IEs follow a last-wins policy.
func DecodeNGReset(in *ngapType.NGReset) (NGReset, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeNGReset,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out NGReset

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveCause     bool
		haveResetType bool
	)

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			haveCause = true

			if ie.Value.Cause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.Cause = *ie.Value.Cause

		case ngapType.ProtocolIEIDResetType:
			haveResetType = true

			if ie.Value.ResetType == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.ResetType = ie.Value.ResetType
		}
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !haveResetType {
		report.MissingMandatory(ngapType.ProtocolIEIDResetType, ngapType.CriticalityPresentReject)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
