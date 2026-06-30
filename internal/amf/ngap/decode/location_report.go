// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

// LocationReport ::= SEQUENCE {
//  protocolIEs ProtocolIE-Container { {LocationReportIEs} },
//  ...
// }
// LocationReportIEs NGAP-PROTOCOL-IES ::= {
//  { ID id-AMF-UE-NGAP-ID                    CRITICALITY reject TYPE AMF-UE-NGAP-ID                    PRESENCE mandatory }|
//  { ID id-RAN-UE-NGAP-ID                    CRITICALITY reject TYPE RAN-UE-NGAP-ID                    PRESENCE mandatory }|
//  { ID id-UserLocationInformation           CRITICALITY ignore TYPE UserLocationInformation           PRESENCE mandatory }|
//  { ID id-UEPresenceInAreaOfInterestList    CRITICALITY ignore TYPE UEPresenceInAreaOfInterestList    PRESENCE optional  }|
//  { ID id-LocationReportingRequestType      CRITICALITY ignore TYPE LocationReportingRequestType      PRESENCE mandatory },
//  ...
// }

// DecodeLocationReport validates a LocationReport PDU body (3GPP TS
// 38.413). AMFUENGAPID and RANUENGAPID are mandatory-reject;
// UserLocationInformation and LocationReportingRequestType are
// mandatory-ignore; UEPresenceInAreaOfInterestList is optional-ignore.
// Class 2 procedure, so procedure-level criticality is "ignore".
// Duplicate IEs follow a last-wins policy.
func DecodeLocationReport(in *ngapType.LocationReport) (LocationReport, *Report) {
	report := &Report{
		ProcedureCode:        ngapType.ProcedureCodeLocationReport,
		TriggeringMessage:    ngapType.TriggeringMessagePresentInitiatingMessage,
		ProcedureCriticality: ngapType.CriticalityPresentIgnore,
	}

	var out LocationReport

	if in == nil {
		report.ProcedureRejected = true
		return out, report
	}

	var (
		haveAMFUENGAPID bool
		haveRANUENGAPID bool
		haveULI         bool
		haveLRRT        bool
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

		case ngapType.ProtocolIEIDUserLocationInformation:
			haveULI = true

			if ie.Value.UserLocationInformation == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UserLocationInformation = ie.Value.UserLocationInformation

		case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
			if ie.Value.UEPresenceInAreaOfInterestList == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.UEPresenceInAreaOfInterestList = ie.Value.UEPresenceInAreaOfInterestList

		case ngapType.ProtocolIEIDLocationReportingRequestType:
			haveLRRT = true

			if ie.Value.LocationReportingRequestType == nil {
				report.Malformed(ie.Id.Value, ngapType.CriticalityPresentIgnore)
				continue
			}

			out.LocationReportingRequestType = ie.Value.LocationReportingRequestType
		}
	}

	if !haveAMFUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDAMFUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveRANUENGAPID {
		report.MissingMandatory(ngapType.ProtocolIEIDRANUENGAPID, ngapType.CriticalityPresentReject)
	}

	if !haveULI {
		report.MissingMandatory(ngapType.ProtocolIEIDUserLocationInformation, ngapType.CriticalityPresentIgnore)
	}

	if !haveLRRT {
		report.MissingMandatory(ngapType.ProtocolIEIDLocationReportingRequestType, ngapType.CriticalityPresentIgnore)
	}

	if !report.HasItems() {
		return out, nil
	}

	return out, report
}
