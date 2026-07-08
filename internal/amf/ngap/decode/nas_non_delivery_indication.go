// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// NASNonDeliveryIndication ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {NASNonDeliveryIndicationIEs} },
//  ...
// }
// NASNonDeliveryIndicationIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID CRITICALITY reject TYPE AMF-UE-NGAP-ID PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID CRITICALITY reject TYPE RAN-UE-NGAP-ID PRESENCE mandatory }|
//  { ID id-NAS-PDU        CRITICALITY ignore TYPE NAS-PDU        PRESENCE mandatory }|
//  { ID id-Cause          CRITICALITY ignore TYPE Cause          PRESENCE mandatory },
//  ...
// }

// DecodeNASNonDeliveryIndication validates a NASNonDeliveryIndication
// PDU body (3GPP TS 38.413).
func DecodeNASNonDeliveryIndication(in *ngapType.NASNonDeliveryIndication) (NASNonDeliveryIndication, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeNASNonDeliveryIndication,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out NASNonDeliveryIndication

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
		haveNASPDU      bool
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

		case ngapType.ProtocolIEIDNASPDU:
			haveNASPDU = true

			if ie.Value.NASPDU == nil || len(ie.Value.NASPDU.Value) == 0 {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			// Copy so the NAS PDU can outlive the decode buffer.
			out.NASPDU = append([]byte(nil), ie.Value.NASPDU.Value...)

		case ngapType.ProtocolIEIDCause:
			haveCause = true

			if ie.Value.Cause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.Cause = *ie.Value.Cause
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveNASPDU {
		report.MissingMandatory(ngapType.ProtocolIEIDNASPDU, ngapType.CriticalityPresentIgnore)
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
