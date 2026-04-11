// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UERadioCapabilityInfoIndication ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UERadioCapabilityInfoIndicationIEs} },
//  ...
// }
// UERadioCapabilityInfoIndicationIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                CRITICALITY reject TYPE AMF-UE-NGAP-ID                PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                CRITICALITY reject TYPE RAN-UE-NGAP-ID                PRESENCE mandatory }|
//  { ID id-UERadioCapability             CRITICALITY ignore TYPE UERadioCapability             PRESENCE mandatory }|
//  { ID id-UERadioCapabilityForPaging    CRITICALITY ignore TYPE UERadioCapabilityForPaging    PRESENCE optional  },
//  ...
// }

// DecodeUERadioCapabilityInfoIndication validates a
// UERadioCapabilityInfoIndication PDU body (3GPP TS 38.413 §9.2.7.7).
// AMFUENGAPID and RANUENGAPID are mandatory-reject; UERadioCapability
// is mandatory-ignore; UERadioCapabilityForPaging is optional-ignore.
// The procedure is class 2, so the procedure-level criticality is
// "ignore". Duplicate IEs follow a last-wins policy.
func DecodeUERadioCapabilityInfoIndication(in *ngapType.UERadioCapabilityInfoIndication) (UERadioCapabilityInfoIndication, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUERadioCapabilityInfoIndication,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out UERadioCapabilityInfoIndication

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID       bool
		haveRANUENGAPID       bool
		haveUERadioCapability bool
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

		case ngapType.ProtocolIEIDUERadioCapability:
			haveUERadioCapability = true

			if ie.Value.UERadioCapability == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UERadioCapability = ie.Value.UERadioCapability.Value

		case ngapType.ProtocolIEIDUERadioCapabilityForPaging:
			if ie.Value.UERadioCapabilityForPaging == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UERadioCapabilityForPaging = ie.Value.UERadioCapabilityForPaging
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveUERadioCapability {
		report.MissingMandatory(ngapType.ProtocolIEIDUERadioCapability, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
