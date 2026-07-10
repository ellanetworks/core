// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
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

func ieEnum(id int64) utils.EnumField[int64] {
	name, ok := ieNames[id]

	return utils.MakeEnum(id, name, !ok)
}

var procedureNames = map[s1ap.ProcedureCode]string{
	s1ap.ProcS1Setup:                           "S1Setup",
	s1ap.ProcInitialUEMessage:                  "InitialUEMessage",
	s1ap.ProcUplinkNASTransport:                "UplinkNASTransport",
	s1ap.ProcDownlinkNASTransport:              "DownlinkNASTransport",
	s1ap.ProcInitialContextSetup:               "InitialContextSetup",
	s1ap.ProcUEContextReleaseRequest:           "UEContextReleaseRequest",
	s1ap.ProcUEContextRelease:                  "UEContextRelease",
	s1ap.ProcUECapabilityInfoIndication:        "UECapabilityInfoIndication",
	s1ap.ProcErrorIndication:                   "ErrorIndication",
	s1ap.ProcPaging:                            "Paging",
	s1ap.ProcHandoverPreparation:               "HandoverPreparation",
	s1ap.ProcHandoverResourceAllocation:        "HandoverResourceAllocation",
	s1ap.ProcHandoverNotification:              "HandoverNotification",
	s1ap.ProcHandoverCancel:                    "HandoverCancel",
	s1ap.ProcENBStatusTransfer:                 "ENBStatusTransfer",
	s1ap.ProcMMEStatusTransfer:                 "MMEStatusTransfer",
	s1ap.ProcDownlinkUEAssociatedLPPaTransport: "DownlinkUEAssociatedLPPaTransport",
	s1ap.ProcUplinkUEAssociatedLPPaTransport:   "UplinkUEAssociatedLPPaTransport",
}

func procedureCodeName(code s1ap.ProcedureCode) string {
	if name, ok := procedureNames[code]; ok {
		return name
	}

	return ""
}

func procedureCodeToEnum(code s1ap.ProcedureCode) utils.EnumField[int64] {
	name := procedureCodeName(code)

	return utils.MakeEnum(int64(code), name, name == "")
}

func criticalityToEnum(c s1ap.Criticality) utils.EnumField[uint64] {
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
	mcc := fmt.Sprintf("%d%d%d", p[0]&0x0f, p[0]>>4, p[1]&0x0f)

	var mnc string
	if p[1]>>4 == 0x0f { // 2-digit MNC: the third digit is filler
		mnc = fmt.Sprintf("%d%d", p[2]&0x0f, p[2]>>4)
	} else {
		mnc = fmt.Sprintf("%d%d%d", p[1]>>4, p[2]&0x0f, p[2]>>4)
	}

	return PLMNID{Mcc: mcc, Mnc: mnc}
}
