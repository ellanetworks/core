// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// UplinkNASTransport ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {UplinkNASTransport-IEs} },
//  ...
// }
// UplinkNASTransport-IEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID           CRITICALITY reject TYPE AMF-UE-NGAP-ID          PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID           CRITICALITY reject TYPE RAN-UE-NGAP-ID          PRESENCE mandatory }|
//  { ID id-NAS-PDU                  CRITICALITY reject TYPE NAS-PDU                 PRESENCE mandatory }|
//  { ID id-UserLocationInformation  CRITICALITY ignore TYPE UserLocationInformation PRESENCE mandatory }|
//  { ID id-W-AGFIdentityInformation CRITICALITY reject TYPE OCTET STRING            PRESENCE optional  }|
//  { ID id-TNGFIdentityInformation  CRITICALITY reject TYPE OCTET STRING            PRESENCE optional  }|
//  { ID id-TWIFIdentityInformation  CRITICALITY reject TYPE OCTET STRING            PRESENCE optional  },
//  ...
// }

// DecodeUplinkNASTransport validates an UplinkNASTransport PDU body
// (3GPP TS 38.413 §9.2.5.3). Mandatory IEs are AMFUENGAPID, RANUENGAPID
// and NASPDU (all reject) and UserLocationInformation (ignore). The
// non-3GPP-access optional IEs (W-AGF/TNGF/TWIF identity information)
// are not consumed by any handler. Duplicate IEs follow a last-wins
// policy.
func DecodeUplinkNASTransport(in *ngapType.UplinkNASTransport) (UplinkNASTransport, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeUplinkNASTransport,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out UplinkNASTransport

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
		haveNASPDU      bool
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

		case ngapType.ProtocolIEIDNASPDU:
			haveNASPDU = true

			if ie.Value.NASPDU == nil || len(ie.Value.NASPDU.Value) == 0 {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.NASPDU = append([]byte(nil), ie.Value.NASPDU.Value...)

		case ngapType.ProtocolIEIDUserLocationInformation:
			haveULI = true

			uli, ok := decodeUserLocationInformation(ie.Value.UserLocationInformation)
			if !ok {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UserLocationInformation = uli
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveNASPDU {
		report.MissingMandatory(ngapType.ProtocolIEIDNASPDU, ngapType.CriticalityPresentReject)
	}

	if !haveULI {
		report.MissingMandatory(ngapType.ProtocolIEIDUserLocationInformation, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
