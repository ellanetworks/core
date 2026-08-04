// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "fmt"

// ProtocolIE-ID names from TS 38.413 §9.4.5. IDs are unique protocol-wide.
var protocolIENames = map[ProtocolIEID]string{
	idAllowedNSSAI:           "AllowedNSSAI",
	idAMFName:                "AMFName",
	idAMFSetID:               "AMFSetID",
	idAMFUENGAPID:            "AMFUENGAPID",
	idCause:                  "Cause",
	idCriticalityDiagnostics: "CriticalityDiagnostics",
	idDefaultPagingDRX:       "DefaultPagingDRX",
	idFiveGSTMSI:             "FiveGSTMSI",
	idGUAMI:                  "GUAMI",
	idGlobalRANNodeID:        "GlobalRANNodeID",
	idNASPDU:                 "NASPDU",
	idPDUSessionResourceFailedToSetupListCxtRes:  "PDUSessionResourceFailedToSetupListCxtRes",
	idPDUSessionResourceFailedToSetupListCxtFail: "PDUSessionResourceFailedToSetupListCxtFail",
	idPDUSessionResourceSetupListCxtReq:          "PDUSessionResourceSetupListCxtReq",
	idPDUSessionResourceSetupListCxtRes:          "PDUSessionResourceSetupListCxtRes",
	idPDUSessionResourceFailedToSetupListSURes:   "PDUSessionResourceFailedToSetupListSURes",
	idPDUSessionResourceSetupListSUReq:           "PDUSessionResourceSetupListSUReq",
	idAdditionalULNGUUPTNLInformation:            "AdditionalUL-NGU-UP-TNLInformation",
	idDataForwardingNotPossible:                  "DataForwardingNotPossible",
	idNetworkInstance:                            "NetworkInstance",
	idPDUSessionAggregateMaximumBitRate:          "PDUSessionAggregateMaximumBitRate",
	idPDUSessionType:                             "PDUSessionType",
	idQosFlowSetupRequestList:                    "QosFlowSetupRequestList",
	idSecurityIndication:                         "SecurityIndication",
	idULNGUUPTNLInformation:                      "UL-NGU-UP-TNLInformation",
	idPDUSessionResourceSetupListSURes:           "PDUSessionResourceSetupListSURes",
	idPDUSessionResourceReleasedListRelRes:       "PDUSessionResourceReleasedListRelRes",
	idPDUSessionResourceNotifyList:               "PDUSessionResourceNotifyList",
	idPDUSessionResourceReleasedListNot:          "PDUSessionResourceReleasedListNot",
	idPDUSessionResourceFailedToModifyListModRes: "PDUSessionResourceFailedToModifyListModRes",
	idPDUSessionResourceModifyListModReq:         "PDUSessionResourceModifyListModReq",
	idPDUSessionResourceModifyListModCfm:         "PDUSessionResourceModifyListModCfm",
	idPDUSessionResourceModifyListModInd:         "PDUSessionResourceModifyListModInd",
	idPDUSessionResourceFailedToModifyListModCfm: "PDUSessionResourceFailedToModifyListModCfm",
	idPDUSessionResourceModifyListModRes:         "PDUSessionResourceModifyListModRes",
	idPDUSessionResourceToReleaseListRelCmd:      "PDUSessionResourceToReleaseListRelCmd",
	idSecurityKey:                                "SecurityKey",
	idUEAggregateMaximumBitRate:                  "UEAggregateMaximumBitRate",
	idUERadioCapability:                          "UERadioCapability",
	idUESecurityCapabilities:                     "UESecurityCapabilities",
	idPDUSessionResourceListCxtRelCpl:            "PDUSessionResourceListCxtRelCpl",
	idPDUSessionResourceListCxtRelReq:            "PDUSessionResourceListCxtRelReq",
	idPLMNSupportList:                            "PLMNSupportList",
	idRANNodeName:                                "RANNodeName",
	idRANUENGAPID:                                "RANUENGAPID",
	idRRCEstablishmentCause:                      "RRCEstablishmentCause",
	idRelativeAMFCapacity:                        "RelativeAMFCapacity",
	idResetType:                                  "ResetType",
	idServedGUAMIList:                            "ServedGUAMIList",
	idSourceAMFUENGAPID:                          "SourceAMFUENGAPID",
	idSONConfigurationTransferDL:                 "SONConfigurationTransferDL",
	idSONConfigurationTransferUL:                 "SONConfigurationTransferUL",
	idPagingDRX:                                  "PagingDRX",
	idPagingOrigin:                               "PagingOrigin",
	idPagingPriority:                             "PagingPriority",
	idTAIListForPaging:                           "TAIListForPaging",
	idUEPagingIdentity:                           "UEPagingIdentity",
	idUnavailableGUAMIList:                       "UnavailableGUAMIList",
	idUserLocationInformation:                    "UserLocationInformation",
	idUERadioCapabilityForPaging:                 "UERadioCapabilityForPaging",
	idSupportedTAList:                            "SupportedTAList",
	idTimeToWait:                                 "TimeToWait",
	idNGRANTNLAssociationToRemoveList:            "NGRANTNLAssociationToRemoveList",
	idUEAssociatedLogicalNGConnectionList:        "UEAssociatedLogicalNGConnectionList",
	idUEContextRequest:                           "UEContextRequest",
	idUENGAPIDs:                                  "UENGAPIDs",
	idUERetentionInformation:                     "UERetentionInformation",
}

func (id ProtocolIEID) String() string {
	if name, ok := protocolIENames[id]; ok {
		return fmt.Sprintf("%s (%d)", name, uint16(id))
	}

	return fmt.Sprintf("ProtocolIEID(%d)", uint16(id))
}
