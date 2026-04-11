// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// HandoverRequired ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {HandoverRequiredIEs} },
//  ...
// }
// HandoverRequiredIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                       CRITICALITY reject TYPE AMF-UE-NGAP-ID                      PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                       CRITICALITY reject TYPE RAN-UE-NGAP-ID                      PRESENCE mandatory }|
//  { ID id-HandoverType                         CRITICALITY reject TYPE HandoverType                        PRESENCE mandatory }|
//  { ID id-Cause                                CRITICALITY ignore TYPE Cause                               PRESENCE mandatory }|
//  { ID id-TargetID                             CRITICALITY reject TYPE TargetID                            PRESENCE mandatory }|
//  { ID id-DirectForwardingPathAvailability     CRITICALITY ignore TYPE DirectForwardingPathAvailability    PRESENCE optional  }|
//  { ID id-PDUSessionResourceListHORqd          CRITICALITY reject TYPE PDUSessionResourceListHORqd         PRESENCE mandatory }|
//  { ID id-SourceToTarget-TransparentContainer  CRITICALITY reject TYPE SourceToTarget-TransparentContainer PRESENCE mandatory },
//  ...
// }

// DecodeHandoverRequired validates a HandoverRequired PDU body (3GPP TS
// 38.413 §9.2.3.1). Six IEs are mandatory-reject (AMFUENGAPID,
// RANUENGAPID, HandoverType, TargetID, PDUSessionResourceListHORqd,
// SourceToTargetTransparentContainer) and Cause is mandatory-ignore.
// DirectForwardingPathAvailability is optional and currently not
// surfaced. Duplicate IEs follow a last-wins policy.
func DecodeHandoverRequired(in *ngapType.HandoverRequired) (HandoverRequired, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeHandoverPreparation,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentReject,
	}

	var out HandoverRequired

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID  bool
		haveRANUENGAPID  bool
		haveHandoverType bool
		haveCause        bool
		haveTargetID     bool
		havePDUList      bool
		haveSrcToTgtCont bool
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

		case ngapType.ProtocolIEIDHandoverType:
			haveHandoverType = true

			if ie.Value.HandoverType == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.HandoverType = *ie.Value.HandoverType

		case ngapType.ProtocolIEIDCause:
			haveCause = true

			if ie.Value.Cause == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.Cause = *ie.Value.Cause

		case ngapType.ProtocolIEIDTargetID:
			haveTargetID = true

			if ie.Value.TargetID == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.TargetID = ie.Value.TargetID

		case ngapType.ProtocolIEIDPDUSessionResourceListHORqd:
			havePDUList = true

			if ie.Value.PDUSessionResourceListHORqd == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.PDUSessionResourceItems = ie.Value.PDUSessionResourceListHORqd.List

		case ngapType.ProtocolIEIDSourceToTargetTransparentContainer:
			haveSrcToTgtCont = true

			if ie.Value.SourceToTargetTransparentContainer == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentReject)
				continue
			}

			out.SourceToTargetTransparentContainer = *ie.Value.SourceToTargetTransparentContainer
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveHandoverType {
		report.MissingMandatory(ngapType.ProtocolIEIDHandoverType, ngapType.CriticalityPresentReject)
	}

	if !haveCause {
		report.MissingMandatory(ngapType.ProtocolIEIDCause, ngapType.CriticalityPresentIgnore)
	}

	if !haveTargetID {
		report.MissingMandatory(ngapType.ProtocolIEIDTargetID, ngapType.CriticalityPresentReject)
	}

	if !havePDUList {
		report.MissingMandatory(ngapType.ProtocolIEIDPDUSessionResourceListHORqd, ngapType.CriticalityPresentReject)
	}

	if !haveSrcToTgtCont {
		report.MissingMandatory(ngapType.ProtocolIEIDSourceToTargetTransparentContainer, ngapType.CriticalityPresentReject)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
