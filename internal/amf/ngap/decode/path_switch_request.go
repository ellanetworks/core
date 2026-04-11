// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// DecodePathSwitchRequest validates a PathSwitchRequest PDU body
// (3GPP TS 38.413 §9.2.3.1). Mandatory IEs are RANUENGAPID,
// SourceAMFUENGAPID, PDUSessionResourceToBeSwitchedDLList (all reject),
// UserLocationInformation and UESecurityCapabilities (both ignore).
// PDUSessionResourceFailedToSetupListPSReq is optional. Duplicate IEs
// follow a last-wins policy.
//
// UESecurityCapabilities is exposed as the validated free5gc pointer:
// the handler walks NRencryptionAlgorithms / NRintegrityProtectionAlgorithms
// directly, and a flattened decoded form is left for a follow-up.
func DecodePathSwitchRequest(in *ngapType.PathSwitchRequest) (PathSwitchRequest, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodePathSwitchRequest,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out PathSwitchRequest

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveRANUENGAPID       bool
		haveSourceAMFUENGAPID bool
		haveULI               bool
		haveSecCaps           bool
		havePDUList           bool
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

		case ngapType.ProtocolIEIDSourceAMFUENGAPID:
			haveSourceAMFUENGAPID = true

			if ie.Value.SourceAMFUENGAPID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.SourceAMFUENGAPID = ie.Value.SourceAMFUENGAPID.Value

		case ngapType.ProtocolIEIDUserLocationInformation:
			haveULI = true

			uli, ok := decodeUserLocationInformation(ie.Value.UserLocationInformation)
			if !ok {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UserLocationInformation = uli

		case ngapType.ProtocolIEIDUESecurityCapabilities:
			haveSecCaps = true

			if ie.Value.UESecurityCapabilities == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UESecurityCapabilities = ie.Value.UESecurityCapabilities

		case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
			havePDUList = true

			if ie.Value.PDUSessionResourceToBeSwitchedDLList == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.PDUSessionResourceItems = ie.Value.PDUSessionResourceToBeSwitchedDLList.List

		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq:
			if ie.Value.PDUSessionResourceFailedToSetupListPSReq == nil {
				continue
			}

			out.FailedToSetupItems = ie.Value.PDUSessionResourceFailedToSetupListPSReq.List
		}
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveSourceAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDSourceAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !havePDUList {
		report.MissingMandatory(ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList, ngapType.CriticalityPresentReject)
	}

	if !haveULI {
		report.MissingMandatory(ngapType.ProtocolIEIDUserLocationInformation, ngapType.CriticalityPresentIgnore)
	}

	if !haveSecCaps {
		report.MissingMandatory(ngapType.ProtocolIEIDUESecurityCapabilities, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
