// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UplinkRANStatusTransfer ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UplinkRANStatusTransferIEs} },
//  ...
// }
// UplinkRANStatusTransferIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                             CRITICALITY reject TYPE AMF-UE-NGAP-ID                             PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                             CRITICALITY reject TYPE RAN-UE-NGAP-ID                             PRESENCE mandatory }|
//  { ID id-RANStatusTransfer-TransparentContainer     CRITICALITY reject TYPE RANStatusTransfer-TransparentContainer     PRESENCE mandatory },
//  ...
// }

// UplinkRANStatusTransfer is the decoded body: the two UE identities and the opaque
// PDCP SN/HFN status container, which the AMF relays transparently to the target.
type UplinkRANStatusTransfer struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Container   *ngapType.RANStatusTransferTransparentContainer
}

// DecodeUplinkRANStatusTransfer validates an UplinkRANStatusTransfer PDU body (3GPP
// TS 38.413 §8.4.6). Class 1 procedure: procedure-level criticality is reject.
func DecodeUplinkRANStatusTransfer(in *ngapType.UplinkRANStatusTransfer) (UplinkRANStatusTransfer, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUplinkRANStatusTransfer,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out UplinkRANStatusTransfer

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
		haveContainer   bool
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

		case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
			haveContainer = true

			if ie.Value.RANStatusTransferTransparentContainer == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.Container = ie.Value.RANStatusTransferTransparentContainer
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveContainer {
		report.MissingMandatory(ngapType.ProtocolIEIDRANStatusTransferTransparentContainer, ngapType.CriticalityPresentReject)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
