// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "fmt"

// ProtocolIE-ID names from TS 36.413 §9.3.6, spelled as the ASN.1
// identifiers are. IDs are unique protocol-wide.
//
// #nosec G101 -- these are 3GPP IE names, not credentials. G101 matches its
// "bearer" pattern against the E-RAB list names, where a bearer is an EPS
// radio bearer (TS 36.413 §9.1.3).
var protocolIENames = map[ProtocolIEID]string{
	IDMMEUES1APID:                               "MME-UE-S1AP-ID",
	IDHandoverType:                              "HandoverType",
	IDCause:                                     "Cause",
	IDTargetID:                                  "TargetID",
	IDENBUES1APID:                               "eNB-UE-S1AP-ID",
	IDERABSubjecttoDataForwardingList:           "E-RABSubjecttoDataForwardingList",
	IDERABtoReleaseListHOCmd:                    "E-RABtoReleaseListHOCmd",
	IDERABDataForwardingItem:                    "E-RABDataForwardingItem",
	IDERABReleaseItemBearerRelComp:              "E-RABReleaseItemBearerRelComp",
	IDERABToBeSetupListBearerSUReq:              "E-RABToBeSetupListBearerSUReq",
	IDERABToBeSetupItemBearerSUReq:              "E-RABToBeSetupItemBearerSUReq",
	IDERABAdmittedList:                          "E-RABAdmittedList",
	IDERABFailedToSetupListHOReqAck:             "E-RABFailedToSetupListHOReqAck",
	IDERABAdmittedItem:                          "E-RABAdmittedItem",
	IDERABFailedtoSetupItemHOReqAck:             "E-RABFailedtoSetupItemHOReqAck",
	IDERABToBeSwitchedDLList:                    "E-RABToBeSwitchedDLList",
	IDERABToBeSwitchedDLItem:                    "E-RABToBeSwitchedDLItem",
	IDERABToBeSetupListCtxtSUReq:                "E-RABToBeSetupListCtxtSUReq",
	IDNASPDU:                                    "NAS-PDU",
	IDERABToBeSetupItemHOReq:                    "E-RABToBeSetupItemHOReq",
	IDERABSetupListBearerSURes:                  "E-RABSetupListBearerSURes",
	IDERABFailedToSetupListBearerSURes:          "E-RABFailedToSetupListBearerSURes",
	IDERABToBeModifiedListBearerModReq:          "E-RABToBeModifiedListBearerModReq",
	IDERABModifyListBearerModRes:                "E-RABModifyListBearerModRes",
	IDERABFailedToModifyList:                    "E-RABFailedToModifyList",
	IDERABToBeReleasedList:                      "E-RABToBeReleasedList",
	IDERABFailedToReleaseList:                   "E-RABFailedToReleaseList",
	IDERABItem:                                  "E-RABItem",
	IDERABToBeModifiedItemBearerModReq:          "E-RABToBeModifiedItemBearerModReq",
	IDERABModifyItemBearerModRes:                "E-RABModifyItemBearerModRes",
	IDERABSetupItemBearerSURes:                  "E-RABSetupItemBearerSURes",
	IDSecurityContext:                           "SecurityContext",
	IDHandoverRestrictionList:                   "HandoverRestrictionList",
	IDUEPagingID:                                "UEPagingID",
	IDPagingDRX:                                 "pagingDRX",
	IDTAIList:                                   "TAIList",
	IDTAIItem:                                   "TAIItem",
	IDERABFailedToSetupListCtxtSURes:            "E-RABFailedToSetupListCtxtSURes",
	IDERABSetupItemCtxtSURes:                    "E-RABSetupItemCtxtSURes",
	IDERABSetupListCtxtSURes:                    "E-RABSetupListCtxtSURes",
	IDERABToBeSetupItemCtxtSUReq:                "E-RABToBeSetupItemCtxtSUReq",
	IDERABToBeSetupListHOReq:                    "E-RABToBeSetupListHOReq",
	IDCriticalityDiagnostics:                    "CriticalityDiagnostics",
	IDGlobalENBID:                               "Global-ENB-ID",
	IDENBname:                                   "eNBname",
	IDMMEname:                                   "MMEname",
	IDSupportedTAs:                              "SupportedTAs",
	IDTimeToWait:                                "TimeToWait",
	IDUEAggregateMaximumBitrate:                 "uEaggregateMaximumBitrate",
	IDTAI:                                       "TAI",
	IDERABReleaseListBearerRelComp:              "E-RABReleaseListBearerRelComp",
	IDSecurityKey:                               "SecurityKey",
	IDUERadioCapability:                         "UERadioCapability",
	IDGUMMEI:                                    "GUMMEI-ID",
	IDDirectForwardingPathAvailability:          "Direct-Forwarding-Path-Availability",
	IDUEIdentityIndexValue:                      "UEIdentityIndexValue",
	IDRelativeMMECapacity:                       "RelativeMMECapacity",
	IDSourceMMEUES1APID:                         "SourceMME-UE-S1AP-ID",
	IDENBStatusTransferTransparentContainer:     "eNB-StatusTransfer-TransparentContainer",
	IDUEAssociatedLogicalS1ConnectionItem:       "UE-associatedLogicalS1-ConnectionItem",
	IDResetType:                                 "ResetType",
	IDUEAssociatedLogicalS1ConnectionListResAck: "UE-associatedLogicalS1-ConnectionListResAck",
	IDSTMSI:                               "S-TMSI",
	IDRequestType:                         "RequestType",
	IDUES1APIDs:                           "UE-S1AP-IDs",
	IDEUTRANCGI:                           "EUTRAN-CGI",
	IDSourceToTargetTransparentContainer:  "Source-ToTarget-TransparentContainer",
	IDServedGUMMEIs:                       "ServedGUMMEIs",
	IDUESecurityCapabilities:              "UESecurityCapabilities",
	IDCNDomain:                            "CNDomain",
	IDTargetToSourceTransparentContainer:  "Target-ToSource-TransparentContainer",
	IDSONConfigurationTransferECT:         "SONConfigurationTransferECT",
	IDSONConfigurationTransferMCT:         "SONConfigurationTransferMCT",
	IDRRCEstablishmentCause:               "RRC-Establishment-Cause",
	IDNASSecurityParametersfromEUTRAN:     "NASSecurityParametersfromE-UTRAN",
	IDNASSecurityParameterstoEUTRAN:       "NASSecurityParameterstoE-UTRAN",
	IDDefaultPagingDRX:                    "DefaultPagingDRX",
	IDDataForwardingNotPossible:           "Data-Forwarding-Not-Possible",
	IDLPPaPDU:                             "LPPa-PDU",
	IDRoutingID:                           "Routing-ID",
	IDPagingPriority:                      "PagingPriority",
	IDUserLocationInformation:             "UserLocationInformation",
	IDUERadioCapabilityForPaging:          "UERadioCapabilityForPaging",
	IDERABToBeModifiedListBearerModInd:    "E-RABToBeModifiedListBearerModInd",
	IDERABToBeModifiedItemBearerModInd:    "E-RABToBeModifiedItemBearerModInd",
	IDERABNotToBeModifiedListBearerModInd: "E-RABNotToBeModifiedListBearerModInd",
	IDERABNotToBeModifiedItemBearerModInd: "E-RABNotToBeModifiedItemBearerModInd",
	IDERABModifyListBearerModConf:         "E-RABModifyListBearerModConf",
	IDERABModifyItemBearerModConf:         "E-RABModifyItemBearerModConf",
	IDUERetentionInformation:              "UE-RetentionInformation",
}

// ProtocolIEIDName returns the TS 36.413 name of id, and whether it is known.
func ProtocolIEIDName(id ProtocolIEID) (string, bool) {
	name, ok := protocolIENames[id]

	return name, ok
}

func (id ProtocolIEID) String() string {
	if name, ok := ProtocolIEIDName(id); ok {
		return fmt.Sprintf("%s (%d)", name, uint16(id))
	}

	return fmt.Sprintf("ProtocolIEID(%d)", uint16(id))
}
