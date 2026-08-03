// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "fmt"

// ProtocolIE-ID names from TS 36.413 §9.3.6. IDs are unique protocol-wide.
//
// #nosec G101 -- these are 3GPP IE names, not credentials. G101 matches its
// "bearer" pattern against the E-RAB list names, where a bearer is an EPS
// radio bearer (TS 36.413 §9.1.3).
var protocolIENames = map[ProtocolIEID]string{
	idMMEUES1APID:                               "MMEUES1APID",
	idHandoverType:                              "HandoverType",
	idCause:                                     "Cause",
	idTargetID:                                  "TargetID",
	idENBUES1APID:                               "ENBUES1APID",
	idERABtoReleaseListHOCmd:                    "ERABtoReleaseListHOCmd",
	idERABReleaseItemBearerRelComp:              "ERABReleaseItemBearerRelComp",
	idERABAdmittedList:                          "ERABAdmittedList",
	idERABFailedToSetupListHOReqAck:             "ERABFailedToSetupListHOReqAck",
	idERABAdmittedItem:                          "ERABAdmittedItem",
	idERABToBeSetupItemHOReq:                    "ERABToBeSetupItemHOReq",
	idERABToBeSetupListBearerSUReq:              "ERABToBeSetupListBearerSUReq",
	idERABToBeSetupItemBearerSUReq:              "ERABToBeSetupItemBearerSUReq",
	idERABToBeSwitchedDLList:                    "ERABToBeSwitchedDLList",
	idERABToBeSwitchedDLItem:                    "ERABToBeSwitchedDLItem",
	idERABToBeSetupListCtxtSUReq:                "ERABToBeSetupListCtxtSUReq",
	idNASPDU:                                    "NASPDU",
	idERABSetupListBearerSURes:                  "ERABSetupListBearerSURes",
	idERABFailedToSetupListBearerSURes:          "ERABFailedToSetupListBearerSURes",
	idERABToBeModifiedListBearerModReq:          "ERABToBeModifiedListBearerModReq",
	idERABModifyListBearerModRes:                "ERABModifyListBearerModRes",
	idERABFailedToModifyList:                    "ERABFailedToModifyList",
	idERABToBeReleasedList:                      "ERABToBeReleasedList",
	idERABFailedToReleaseList:                   "ERABFailedToReleaseList",
	idERABItem:                                  "ERABItem",
	idERABToBeModifiedItemBearerModReq:          "ERABToBeModifiedItemBearerModReq",
	idERABModifyItemBearerModRes:                "ERABModifyItemBearerModRes",
	idERABReleaseListBearerRelComp:              "ERABReleaseListBearerRelComp",
	idERABSetupItemBearerSURes:                  "ERABSetupItemBearerSURes",
	idSecurityContext:                           "SecurityContext",
	idUEPagingID:                                "UEPagingID",
	idPagingDRX:                                 "PagingDRX",
	idTAIList:                                   "TAIList",
	idTAIItem:                                   "TAIItem",
	idERABFailedToSetupListCtxtSU:               "ERABFailedToSetupListCtxtSU",
	idERABSetupItemCtxtSURes:                    "ERABSetupItemCtxtSURes",
	idERABSetupListCtxtSURes:                    "ERABSetupListCtxtSURes",
	idERABToBeSetupItemCtxtSUReq:                "ERABToBeSetupItemCtxtSUReq",
	idERABToBeSetupListHOReq:                    "ERABToBeSetupListHOReq",
	idCriticalityDiagnostics:                    "CriticalityDiagnostics",
	idGlobalENBID:                               "GlobalENBID",
	idENBname:                                   "ENBname",
	idMMEname:                                   "MMEname",
	idSupportedTAs:                              "SupportedTAs",
	idTimeToWait:                                "TimeToWait",
	idUEAggregateMaximumBitrate:                 "UEAggregateMaximumBitrate",
	idTAI:                                       "TAI",
	idENBStatusTransferTransparentContainer:     "ENBStatusTransferTransparentContainer",
	idSourceToTargetTransparentContainer:        "SourceToTargetTransparentContainer",
	idTargetToSourceTransparentContainer:        "TargetToSourceTransparentContainer",
	idSONConfigurationTransferECT:               "SONConfigurationTransferECT",
	idSONConfigurationTransferMCT:               "SONConfigurationTransferMCT",
	idSecurityKey:                               "SecurityKey",
	idUERadioCapability:                         "UERadioCapability",
	idGUMMEI:                                    "GUMMEI",
	idUEIdentityIndexValue:                      "UEIdentityIndexValue",
	idRelativeMMECapacity:                       "RelativeMMECapacity",
	idSourceMMEUES1APID:                         "SourceMMEUES1APID",
	idUEAssociatedLogicalS1ConnectionItem:       "UEAssociatedLogicalS1ConnectionItem",
	idResetType:                                 "ResetType",
	idUEAssociatedLogicalS1ConnectionListResAck: "UEAssociatedLogicalS1ConnectionListResAck",
	idSTMSI:                               "STMSI",
	idUERetentionInformation:              "UERetentionInformation",
	idUES1APIDs:                           "UES1APIDs",
	idEUTRANCGI:                           "EUTRANCGI",
	idServedGUMMEIs:                       "ServedGUMMEIs",
	idUESecurityCapabilities:              "UESecurityCapabilities",
	idCNDomain:                            "CNDomain",
	idPagingPriority:                      "PagingPriority",
	idUERadioCapabilityForPaging:          "UERadioCapabilityForPaging",
	idRequestType:                         "RequestType",
	idRRCEstablishmentCause:               "RRCEstablishmentCause",
	idDefaultPagingDRX:                    "DefaultPagingDRX",
	idLPPaPDU:                             "LPPaPDU",
	idRoutingID:                           "RoutingID",
	idUserLocationInformation:             "UserLocationInformation",
	idERABToBeModifiedListBearerModInd:    "ERABToBeModifiedListBearerModInd",
	idERABToBeModifiedItemBearerModInd:    "ERABToBeModifiedItemBearerModInd",
	idERABNotToBeModifiedListBearerModInd: "ERABNotToBeModifiedListBearerModInd",
	idERABNotToBeModifiedItemBearerModInd: "ERABNotToBeModifiedItemBearerModInd",
	idERABModifyListBearerModConf:         "ERABModifyListBearerModConf",
	idERABModifyItemBearerModConf:         "ERABModifyItemBearerModConf",
}

func (id ProtocolIEID) String() string {
	if name, ok := protocolIENames[id]; ok {
		return fmt.Sprintf("%s (%d)", name, uint16(id))
	}

	return fmt.Sprintf("ProtocolIEID(%d)", uint16(id))
}
