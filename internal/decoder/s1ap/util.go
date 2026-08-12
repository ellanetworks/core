// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

// ProtocolIE-ID values used by the decoded messages (TS 36.413 §9.3,
// S1AP-Constants). The s1ap library keeps its own copies unexported, so the
// decoder mirrors the spec values it needs.
const (
	idMMEUES1APID                    int64 = 0
	idHandoverType                   int64 = 1
	idCause                          int64 = 2
	idTargetID                       int64 = 4
	idENBUES1APID                    int64 = 8
	idERABtoReleaseListHOCmd         int64 = 13
	idERABAdmittedList               int64 = 18
	idERABFailedToSetupListHOReqAck  int64 = 19
	idERABToBeSetupListCtxtSUReq     int64 = 24
	idNASPDU                         int64 = 26
	idSecurityContext                int64 = 40
	idERABToBeSetupListHOReq         int64 = 53
	idENBStatusTransferContainer     int64 = 90
	idSourceToTargetContainer        int64 = 104
	idTargetToSourceContainer        int64 = 123
	idERABFailedToSetupListCtxtSURes int64 = 48
	idERABSetupItemCtxtSURes         int64 = 50
	idERABSetupListCtxtSURes         int64 = 51
	idCriticalityDiagnostics         int64 = 58
	idGlobalENBID                    int64 = 59
	idENBname                        int64 = 60
	idMMEname                        int64 = 61
	idSupportedTAs                   int64 = 64
	idTimeToWait                     int64 = 65
	idUEAggregateMaximumBitrate      int64 = 66
	idSecurityKey                    int64 = 73
	idUERadioCapability              int64 = 74
	idGUMMEI                         int64 = 75
	idUEIdentityIndexValue           int64 = 80
	idRelativeMMECapacity            int64 = 87
	idSTMSI                          int64 = 96
	idUES1APIDs                      int64 = 99
	idEUTRANCGI                      int64 = 100
	idServedGUMMEIs                  int64 = 105
	idUESecurityCapabilities         int64 = 107
	idCNDomain                       int64 = 109
	idUERadioCapabilityForPaging     int64 = 117
	idRRCEstablishmentCause          int64 = 134
	idDefaultPagingDRX               int64 = 137
	idLPPaPDU                        int64 = 147
	idRoutingID                      int64 = 148
	idTAIList                        int64 = 46
)

var ieNames = map[int64]string{
	idMMEUES1APID:                    "MME-UE-S1AP-ID",
	idHandoverType:                   "HandoverType",
	idCause:                          "Cause",
	idTargetID:                       "TargetID",
	idENBUES1APID:                    "eNB-UE-S1AP-ID",
	idERABtoReleaseListHOCmd:         "E-RABtoReleaseListHOCmd",
	idERABAdmittedList:               "E-RABAdmittedList",
	idERABFailedToSetupListHOReqAck:  "E-RABFailedToSetupListHOReqAck",
	idSecurityContext:                "SecurityContext",
	idERABToBeSetupListHOReq:         "E-RABToBeSetupListHOReq",
	idENBStatusTransferContainer:     "eNB-StatusTransfer-TransparentContainer",
	idSourceToTargetContainer:        "Source-ToTarget-TransparentContainer",
	idTargetToSourceContainer:        "Target-ToSource-TransparentContainer",
	idERABToBeSetupListCtxtSUReq:     "E-RABToBeSetupListCtxtSUReq",
	idNASPDU:                         "NAS-PDU",
	idERABSetupItemCtxtSURes:         "E-RABSetupItemCtxtSURes",
	idERABSetupListCtxtSURes:         "E-RABSetupListCtxtSURes",
	idERABFailedToSetupListCtxtSURes: "E-RABFailedToSetupListCtxtSURes",
	idCriticalityDiagnostics:         "CriticalityDiagnostics",
	idGlobalENBID:                    "Global-ENB-ID",
	idENBname:                        "eNBname",
	idMMEname:                        "MMEname",
	idSupportedTAs:                   "SupportedTAs",
	idTimeToWait:                     "TimeToWait",
	idUEAggregateMaximumBitrate:      "uEaggregateMaximumBitrate",
	idSecurityKey:                    "SecurityKey",
	idUERadioCapability:              "UERadioCapability",
	idUEIdentityIndexValue:           "UEIdentityIndexValue",
	idRelativeMMECapacity:            "RelativeMMECapacity",
	idSTMSI:                          "S-TMSI",
	idUES1APIDs:                      "UE-S1AP-IDs",
	idEUTRANCGI:                      "EUTRAN-CGI",
	idServedGUMMEIs:                  "ServedGUMMEIs",
	idUESecurityCapabilities:         "UESecurityCapabilities",
	idCNDomain:                       "CNDomain",
	idGUMMEI:                         "GUMMEI",
	idUERadioCapabilityForPaging:     "UERadioCapabilityForPaging",
	idRRCEstablishmentCause:          "RRC-Establishment-Cause",
	idDefaultPagingDRX:               "DefaultPagingDRX",
	idTAIList:                        "TAIList",
	idLPPaPDU:                        "LPPa-PDU",
	idRoutingID:                      "Routing-ID",
}

func ieEnum(id int64) utils.EnumField {
	name, ok := ieNames[id]

	return utils.MakeEnum(id, name, !ok)
}

// procedureNames is the label vocabulary the events drawer renders procedure
// codes with, in wire order. internal/decoder/ngap keeps the same shape.
var procedureNames = map[s1ap.ProcedureCode]string{
	s1ap.ProcHandoverPreparation:                  "HandoverPreparation",
	s1ap.ProcHandoverResourceAllocation:           "HandoverResourceAllocation",
	s1ap.ProcHandoverNotification:                 "HandoverNotification",
	s1ap.ProcPathSwitchRequest:                    "PathSwitchRequest",
	s1ap.ProcHandoverCancel:                       "HandoverCancel",
	s1ap.ProcERABSetup:                            "ERABSetup",
	s1ap.ProcERABModify:                           "ERABModify",
	s1ap.ProcERABRelease:                          "ERABRelease",
	s1ap.ProcERABReleaseIndication:                "ERABReleaseIndication",
	s1ap.ProcInitialContextSetup:                  "InitialContextSetup",
	s1ap.ProcPaging:                               "Paging",
	s1ap.ProcDownlinkNASTransport:                 "DownlinkNASTransport",
	s1ap.ProcInitialUEMessage:                     "InitialUEMessage",
	s1ap.ProcUplinkNASTransport:                   "UplinkNASTransport",
	s1ap.ProcReset:                                "Reset",
	s1ap.ProcErrorIndication:                      "ErrorIndication",
	s1ap.ProcNASNonDeliveryIndication:             "NASNonDeliveryIndication",
	s1ap.ProcS1Setup:                              "S1Setup",
	s1ap.ProcUEContextReleaseRequest:              "UEContextReleaseRequest",
	s1ap.ProcDownlinkS1cdma2000tunnelling:         "DownlinkS1cdma2000tunnelling",
	s1ap.ProcUplinkS1cdma2000tunnelling:           "UplinkS1cdma2000tunnelling",
	s1ap.ProcUEContextModification:                "UEContextModification",
	s1ap.ProcUECapabilityInfoIndication:           "UECapabilityInfoIndication",
	s1ap.ProcUEContextRelease:                     "UEContextRelease",
	s1ap.ProcENBStatusTransfer:                    "ENBStatusTransfer",
	s1ap.ProcMMEStatusTransfer:                    "MMEStatusTransfer",
	s1ap.ProcDeactivateTrace:                      "DeactivateTrace",
	s1ap.ProcTraceStart:                           "TraceStart",
	s1ap.ProcTraceFailureIndication:               "TraceFailureIndication",
	s1ap.ProcENBConfigurationUpdate:               "ENBConfigurationUpdate",
	s1ap.ProcMMEConfigurationUpdate:               "MMEConfigurationUpdate",
	s1ap.ProcLocationReportingControl:             "LocationReportingControl",
	s1ap.ProcLocationReportingFailureIndication:   "LocationReportingFailureIndication",
	s1ap.ProcLocationReport:                       "LocationReport",
	s1ap.ProcOverloadStart:                        "OverloadStart",
	s1ap.ProcOverloadStop:                         "OverloadStop",
	s1ap.ProcWriteReplaceWarning:                  "WriteReplaceWarning",
	s1ap.ProcENBDirectInformationTransfer:         "ENBDirectInformationTransfer",
	s1ap.ProcMMEDirectInformationTransfer:         "MMEDirectInformationTransfer",
	s1ap.ProcPrivateMessage:                       "PrivateMessage",
	s1ap.ProcENBConfigurationTransfer:             "ENBConfigurationTransfer",
	s1ap.ProcMMEConfigurationTransfer:             "MMEConfigurationTransfer",
	s1ap.ProcCellTrafficTrace:                     "CellTrafficTrace",
	s1ap.ProcKill:                                 "Kill",
	s1ap.ProcDownlinkUEAssociatedLPPaTransport:    "DownlinkUEAssociatedLPPaTransport",
	s1ap.ProcUplinkUEAssociatedLPPaTransport:      "UplinkUEAssociatedLPPaTransport",
	s1ap.ProcDownlinkNonUEAssociatedLPPaTransport: "DownlinkNonUEAssociatedLPPaTransport",
	s1ap.ProcUplinkNonUEAssociatedLPPaTransport:   "UplinkNonUEAssociatedLPPaTransport",
	s1ap.ProcUERadioCapabilityMatch:               "UERadioCapabilityMatch",
	s1ap.ProcPWSRestartIndication:                 "PWSRestartIndication",
	s1ap.ProcERABModificationIndication:           "ERABModificationIndication",
	s1ap.ProcPWSFailureIndication:                 "PWSFailureIndication",
	s1ap.ProcRerouteNASRequest:                    "RerouteNASRequest",
	s1ap.ProcUEContextModificationIndication:      "UEContextModificationIndication",
	s1ap.ProcConnectionEstablishmentIndication:    "ConnectionEstablishmentIndication",
	s1ap.ProcUEContextSuspend:                     "UEContextSuspend",
	s1ap.ProcUEContextResume:                      "UEContextResume",
	s1ap.ProcNASDeliveryIndication:                "NASDeliveryIndication",
	s1ap.ProcRetrieveUEInformation:                "RetrieveUEInformation",
	s1ap.ProcUEInformationTransfer:                "UEInformationTransfer",
	s1ap.ProcENBCPRelocationIndication:            "ENBCPRelocationIndication",
	s1ap.ProcMMECPRelocationIndication:            "MMECPRelocationIndication",
	s1ap.ProcSecondaryRATDataUsageReport:          "SecondaryRATDataUsageReport",
	s1ap.ProcUERadioCapabilityIDMapping:           "UERadioCapabilityIDMapping",
	s1ap.ProcHandoverSuccess:                      "HandoverSuccess",
	s1ap.ProcENBEarlyStatusTransfer:               "ENBEarlyStatusTransfer",
	s1ap.ProcMMEEarlyStatusTransfer:               "MMEEarlyStatusTransfer",
}

func procedureCodeName(code s1ap.ProcedureCode) string {
	if name, ok := procedureNames[code]; ok {
		return name
	}

	return ""
}

func procedureCodeToEnum(code s1ap.ProcedureCode) utils.EnumField {
	name := procedureCodeName(code)

	return utils.MakeEnum(int64(code), name, name == "")
}

func criticalityToEnum(c s1ap.Criticality) utils.EnumField {
	switch c {
	case s1ap.CriticalityReject:
		return utils.MakeEnum(uint64(c), "Reject", false)
	case s1ap.CriticalityIgnore:
		return utils.MakeEnum(uint64(c), "Ignore", false)
	case s1ap.CriticalityNotify:
		return utils.MakeEnum(uint64(c), "Notify", false)
	default:
		return utils.MakeEnum(uint64(c), "", true)
	}
}

// PLMNID is the MCC/MNC view of a 3-octet PLMN identity.
type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

// plmnToID decodes a PLMN identity (TS 24.008 §10.5.1.3 / TS 23.003 BCD nibble
// packing) into its MCC/MNC digits.
func plmnToID(p s1ap.PLMNIdentity) PLMNID {
	plmn, err := nas.ParsePLMN([3]byte(p))
	if err != nil {
		return PLMNID{}
	}

	return PLMNID{Mcc: plmn.MCC, Mnc: plmn.MNC}
}
