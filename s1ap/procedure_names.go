// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "fmt"

// Elementary procedure names from TS 36.413 §9.3.6 (Constant Definitions).
//
// #nosec G101 -- these are 3GPP procedure names, not credentials. G101 matches
// its "pw" pattern against PWSRestartIndication, where PWS is the Public
// Warning System (TS 36.413 §9.1.13).
var procedureNames = map[ProcedureCode]string{
	ProcHandoverPreparation:                "HandoverPreparation",
	ProcHandoverResourceAllocation:         "HandoverResourceAllocation",
	ProcHandoverNotification:               "HandoverNotification",
	ProcPathSwitchRequest:                  "PathSwitchRequest",
	ProcHandoverCancel:                     "HandoverCancel",
	ProcERABSetup:                          "ERABSetup",
	ProcERABModify:                         "ERABModify",
	ProcERABRelease:                        "ERABRelease",
	ProcERABReleaseIndication:              "ERABReleaseIndication",
	ProcInitialContextSetup:                "InitialContextSetup",
	ProcPaging:                             "Paging",
	ProcDownlinkNASTransport:               "DownlinkNASTransport",
	ProcInitialUEMessage:                   "InitialUEMessage",
	ProcUplinkNASTransport:                 "UplinkNASTransport",
	ProcReset:                              "Reset",
	ProcErrorIndication:                    "ErrorIndication",
	ProcNASNonDeliveryIndication:           "NASNonDeliveryIndication",
	ProcS1Setup:                            "S1Setup",
	ProcUEContextReleaseRequest:            "UEContextReleaseRequest",
	ProcDownlinkS1cdma2000tunnelling:       "DownlinkS1cdma2000tunnelling",
	ProcUplinkS1cdma2000tunnelling:         "UplinkS1cdma2000tunnelling",
	ProcUEContextModification:              "UEContextModification",
	ProcUECapabilityInfoIndication:         "UECapabilityInfoIndication",
	ProcUEContextRelease:                   "UEContextRelease",
	ProcENBStatusTransfer:                  "ENBStatusTransfer",
	ProcMMEStatusTransfer:                  "MMEStatusTransfer",
	ProcDeactivateTrace:                    "DeactivateTrace",
	ProcTraceStart:                         "TraceStart",
	ProcTraceFailureIndication:             "TraceFailureIndication",
	ProcENBConfigurationUpdate:             "ENBConfigurationUpdate",
	ProcMMEConfigurationUpdate:             "MMEConfigurationUpdate",
	ProcLocationReportingControl:           "LocationReportingControl",
	ProcLocationReportingFailureIndication: "LocationReportingFailureIndication",
	ProcLocationReport:                     "LocationReport",
	ProcOverloadStart:                      "OverloadStart",
	ProcOverloadStop:                       "OverloadStop",
	ProcWriteReplaceWarning:                "WriteReplaceWarning",
	ProcENBDirectInformationTransfer:       "ENBDirectInformationTransfer",
	ProcMMEDirectInformationTransfer:       "MMEDirectInformationTransfer",
	ProcPrivateMessage:                     "PrivateMessage",
	ProcENBConfigurationTransfer:           "ENBConfigurationTransfer",
	ProcMMEConfigurationTransfer:           "MMEConfigurationTransfer",
	ProcCellTrafficTrace:                   "CellTrafficTrace",
	ProcKill:                               "Kill",
	ProcDownlinkUEAssociatedLPPaTransport:  "DownlinkUEAssociatedLPPaTransport",
	ProcUplinkUEAssociatedLPPaTransport:    "UplinkUEAssociatedLPPaTransport",
	ProcDownlinkNonUEAssociatedLPPa:        "DownlinkNonUEAssociatedLPPa",
	ProcUplinkNonUEAssociatedLPPa:          "UplinkNonUEAssociatedLPPa",
	ProcUERadioCapabilityMatch:             "UERadioCapabilityMatch",
	ProcPWSRestartIndication:               "PWSRestartIndication",
	ProcERABModificationIndication:         "ERABModificationIndication",
	ProcPWSFailureIndication:               "PWSFailureIndication",
	ProcRerouteNASRequest:                  "RerouteNASRequest",
	ProcUEContextModificationIndication:    "UEContextModificationIndication",
	ProcConnectionEstablishmentIndication:  "ConnectionEstablishmentIndication",
	ProcUEContextSuspend:                   "UEContextSuspend",
	ProcUEContextResume:                    "UEContextResume",
	ProcNASDeliveryIndication:              "NASDeliveryIndication",
	ProcRetrieveUEInformation:              "RetrieveUEInformation",
	ProcUEInformationTransfer:              "UEInformationTransfer",
	ProcENBCPRelocationIndication:          "ENBCPRelocationIndication",
	ProcMMECPRelocationIndication:          "MMECPRelocationIndication",
	ProcSecondaryRATDataUsageReport:        "SecondaryRATDataUsageReport",
	ProcUERadioCapabilityIDMapping:         "UERadioCapabilityIDMapping",
	ProcHandoverSuccess:                    "HandoverSuccess",
	ProcENBEarlyStatusTransfer:             "ENBEarlyStatusTransfer",
	ProcMMEEarlyStatusTransfer:             "MMEEarlyStatusTransfer",
}

func (p ProcedureCode) String() string {
	if name, ok := procedureNames[p]; ok {
		return fmt.Sprintf("%s (%d)", name, uint8(p))
	}

	return fmt.Sprintf("ProcedureCode(%d)", uint8(p))
}
