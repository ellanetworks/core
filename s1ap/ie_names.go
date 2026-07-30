// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "fmt"

// protocolIENames maps each ProtocolIE-ID to its TS 36.413 name
// (S1AP-Constants, §9.3.5). IDs are globally unique across the protocol.
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
	idUES1APIDs:                           "UES1APIDs",
	idEUTRANCGI:                           "EUTRANCGI",
	idServedGUMMEIs:                       "ServedGUMMEIs",
	idUESecurityCapabilities:              "UESecurityCapabilities",
	idCNDomain:                            "CNDomain",
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

// String returns the IE name and its id, e.g. "GlobalENBID (59)".
func (id ProtocolIEID) String() string {
	if name, ok := protocolIENames[id]; ok {
		return fmt.Sprintf("%s (%d)", name, uint16(id))
	}

	return fmt.Sprintf("ProtocolIEID(%d)", uint16(id))
}
