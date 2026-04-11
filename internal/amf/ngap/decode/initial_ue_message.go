// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// DecodeInitialUEMessage validates an InitialUEMessage PDU body
// (3GPP TS 38.413 §9.2.5.1). Mandatory IEs are RANUENGAPID, NASPDU,
// UserLocationInformation (all reject) and RRCEstablishmentCause
// (ignore). Duplicate IEs follow a last-wins policy.
func DecodeInitialUEMessage(in *ngapType.InitialUEMessage) (InitialUEMessage, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeInitialUEMessage,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out InitialUEMessage

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	// "have" flags are set when an IE id is observed regardless of
	// whether its value was well-formed, so a malformed IE is not also
	// reported as missing.
	var (
		haveRANUENGAPID bool
		haveNASPDU      bool
		haveULI         bool
		haveRRCCause    bool
	)

	for _, ie := range in.ProtocolIEs.List {
		switch ie.Id.Value {
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
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.UserLocationInformation = uli

		case ngapType.ProtocolIEIDRRCEstablishmentCause:
			haveRRCCause = true

			if ie.Value.RRCEstablishmentCause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.RRCEstablishmentCause = RRCEstablishmentCause(ie.Value.RRCEstablishmentCause.Value)

		case ngapType.ProtocolIEIDFiveGSTMSI:
			if ie.Value.FiveGSTMSI == nil {
				continue
			}

			tmsi, ok := decodeFiveGSTMSI(ie.Value.FiveGSTMSI)
			if !ok {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.FiveGSTMSI = &tmsi

		case ngapType.ProtocolIEIDUEContextRequest:
			out.UEContextRequest = ie.Value.UEContextRequest != nil
		}
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveNASPDU {
		report.MissingMandatory(ngapType.ProtocolIEIDNASPDU, ngapType.CriticalityPresentReject)
	}

	if !haveULI {
		report.MissingMandatory(ngapType.ProtocolIEIDUserLocationInformation, ngapType.CriticalityPresentReject)
	}

	if !haveRRCCause {
		report.MissingMandatory(ngapType.ProtocolIEIDRRCEstablishmentCause, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
