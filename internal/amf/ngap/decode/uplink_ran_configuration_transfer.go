// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UplinkRANConfigurationTransfer ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UplinkRANConfigurationTransferIEs} },
//  ...
// }
// UplinkRANConfigurationTransferIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-SONConfigurationTransferUL     CRITICALITY ignore TYPE SONConfigurationTransfer     PRESENCE optional },
//  ...
// }

// DecodeUplinkRANConfigurationTransfer validates an
// UplinkRANConfigurationTransfer PDU body (3GPP TS 38.413 §9.2.6.8).
// All IEs are optional-ignore. SONConfigurationTransferUL is surfaced
// because the handler needs it to forward the embedded SON data.
// ENDCSONConfigurationTransferUL is consumed for validation only.
// Duplicate IEs follow a last-wins policy.
func DecodeUplinkRANConfigurationTransfer(in *ngapType.UplinkRANConfigurationTransfer) (UplinkRANConfigurationTransfer, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUplinkRANConfigurationTransfer,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out UplinkRANConfigurationTransfer

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDSONConfigurationTransferUL:
			if ie.Value.SONConfigurationTransferUL == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.SONConfigurationTransferUL = ie.Value.SONConfigurationTransferUL
		}
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
