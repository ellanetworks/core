// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "fmt"

// Elementary procedure names from TS 38.413 §9.4.7 (Constant Definitions).
//
// #nosec G101 -- these are 3GPP procedure names, not credentials. G101 matches
// its "pw" pattern against the PWSCancel, PWSFailureIndication and
// PWSRestartIndication names, where PWS is the Public Warning System
// (TS 38.413 §8.11).
var procedureNames = map[ProcedureCode]string{
	ProcAMFConfigurationUpdate:                "AMFConfigurationUpdate",
	ProcAMFStatusIndication:                   "AMFStatusIndication",
	ProcCellTrafficTrace:                      "CellTrafficTrace",
	ProcDeactivateTrace:                       "DeactivateTrace",
	ProcDownlinkNASTransport:                  "DownlinkNASTransport",
	ProcDownlinkNonUEAssociatedNRPPaTransport: "DownlinkNonUEAssociatedNRPPaTransport",
	ProcDownlinkRANConfigurationTransfer:      "DownlinkRANConfigurationTransfer",
	ProcDownlinkRANStatusTransfer:             "DownlinkRANStatusTransfer",
	ProcDownlinkUEAssociatedNRPPaTransport:    "DownlinkUEAssociatedNRPPaTransport",
	ProcErrorIndication:                       "ErrorIndication",
	ProcHandoverCancel:                        "HandoverCancel",
	ProcHandoverNotification:                  "HandoverNotification",
	ProcHandoverPreparation:                   "HandoverPreparation",
	ProcHandoverResourceAllocation:            "HandoverResourceAllocation",
	ProcInitialContextSetup:                   "InitialContextSetup",
	ProcInitialUEMessage:                      "InitialUEMessage",
	ProcLocationReportingControl:              "LocationReportingControl",
	ProcLocationReportingFailureIndication:    "LocationReportingFailureIndication",
	ProcLocationReport:                        "LocationReport",
	ProcNASNonDeliveryIndication:              "NASNonDeliveryIndication",
	ProcNGReset:                               "NGReset",
	ProcNGSetup:                               "NGSetup",
	ProcOverloadStart:                         "OverloadStart",
	ProcOverloadStop:                          "OverloadStop",
	ProcPaging:                                "Paging",
	ProcPathSwitchRequest:                     "PathSwitchRequest",
	ProcPDUSessionResourceModify:              "PDUSessionResourceModify",
	ProcPDUSessionResourceModifyIndication:    "PDUSessionResourceModifyIndication",
	ProcPDUSessionResourceRelease:             "PDUSessionResourceRelease",
	ProcPDUSessionResourceSetup:               "PDUSessionResourceSetup",
	ProcPDUSessionResourceNotify:              "PDUSessionResourceNotify",
	ProcPrivateMessage:                        "PrivateMessage",
	ProcPWSCancel:                             "PWSCancel",
	ProcPWSFailureIndication:                  "PWSFailureIndication",
	ProcPWSRestartIndication:                  "PWSRestartIndication",
	ProcRANConfigurationUpdate:                "RANConfigurationUpdate",
	ProcRerouteNASRequest:                     "RerouteNASRequest",
	ProcRRCInactiveTransitionReport:           "RRCInactiveTransitionReport",
	ProcTraceFailureIndication:                "TraceFailureIndication",
	ProcTraceStart:                            "TraceStart",
	ProcUEContextModification:                 "UEContextModification",
	ProcUEContextRelease:                      "UEContextRelease",
	ProcUEContextReleaseRequest:               "UEContextReleaseRequest",
	ProcUERadioCapabilityCheck:                "UERadioCapabilityCheck",
	ProcUERadioCapabilityInfoIndication:       "UERadioCapabilityInfoIndication",
	ProcUETNLABindingRelease:                  "UETNLABindingRelease",
	ProcUplinkNASTransport:                    "UplinkNASTransport",
	ProcUplinkNonUEAssociatedNRPPaTransport:   "UplinkNonUEAssociatedNRPPaTransport",
	ProcUplinkRANConfigurationTransfer:        "UplinkRANConfigurationTransfer",
	ProcUplinkRANStatusTransfer:               "UplinkRANStatusTransfer",
	ProcUplinkUEAssociatedNRPPaTransport:      "UplinkUEAssociatedNRPPaTransport",
	ProcWriteReplaceWarning:                   "WriteReplaceWarning",
	ProcSecondaryRATDataUsageReport:           "SecondaryRATDataUsageReport",
	ProcUplinkRIMInformationTransfer:          "UplinkRIMInformationTransfer",
	ProcDownlinkRIMInformationTransfer:        "DownlinkRIMInformationTransfer",
	ProcRetrieveUEInformation:                 "RetrieveUEInformation",
	ProcUEInformationTransfer:                 "UEInformationTransfer",
	ProcRANCPRelocationIndication:             "RANCPRelocationIndication",
	ProcUEContextResume:                       "UEContextResume",
	ProcUEContextSuspend:                      "UEContextSuspend",
	ProcUERadioCapabilityIDMapping:            "UERadioCapabilityIDMapping",
	ProcHandoverSuccess:                       "HandoverSuccess",
	ProcUplinkRANEarlyStatusTransfer:          "UplinkRANEarlyStatusTransfer",
	ProcDownlinkRANEarlyStatusTransfer:        "DownlinkRANEarlyStatusTransfer",
	ProcAMFCPRelocationIndication:             "AMFCPRelocationIndication",
	ProcConnectionEstablishmentIndication:     "ConnectionEstablishmentIndication",
	ProcBroadcastSessionModification:          "BroadcastSessionModification",
	ProcBroadcastSessionRelease:               "BroadcastSessionRelease",
	ProcBroadcastSessionSetup:                 "BroadcastSessionSetup",
	ProcDistributionSetup:                     "DistributionSetup",
	ProcDistributionRelease:                   "DistributionRelease",
	ProcMulticastSessionActivation:            "MulticastSessionActivation",
	ProcMulticastSessionDeactivation:          "MulticastSessionDeactivation",
	ProcMulticastSessionUpdate:                "MulticastSessionUpdate",
	ProcMulticastGroupPaging:                  "MulticastGroupPaging",
	ProcBroadcastSessionReleaseRequired:       "BroadcastSessionReleaseRequired",
	ProcTimingSynchronisationStatus:           "TimingSynchronisationStatus",
	ProcTimingSynchronisationStatusReport:     "TimingSynchronisationStatusReport",
	ProcMTCommunicationHandling:               "MTCommunicationHandling",
	ProcRANPagingRequest:                      "RANPagingRequest",
	ProcBroadcastSessionTransport:             "BroadcastSessionTransport",
}

// ProcedureCodeName returns the TS 38.413 name of p, and whether it is known.
func ProcedureCodeName(p ProcedureCode) (string, bool) {
	name, ok := procedureNames[p]

	return name, ok
}

func (p ProcedureCode) String() string {
	if name, ok := ProcedureCodeName(p); ok {
		return fmt.Sprintf("%s (%d)", name, uint8(p))
	}

	return fmt.Sprintf("ProcedureCode(%d)", uint8(p))
}
