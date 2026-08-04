// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "fmt"

// ProtocolIE-ID names from TS 38.413 §9.4.5. IDs are unique protocol-wide.
var protocolIENames = map[ProtocolIEID]string{
	idAllowedNSSAI:                        "AllowedNSSAI",
	idAMFName:                             "AMFName",
	idAMFSetID:                            "AMFSetID",
	idAMFUENGAPID:                         "AMFUENGAPID",
	idCause:                               "Cause",
	idCriticalityDiagnostics:              "CriticalityDiagnostics",
	idDefaultPagingDRX:                    "DefaultPagingDRX",
	idFiveGSTMSI:                          "FiveGSTMSI",
	idGlobalRANNodeID:                     "GlobalRANNodeID",
	idNASPDU:                              "NASPDU",
	idPLMNSupportList:                     "PLMNSupportList",
	idRANNodeName:                         "RANNodeName",
	idRANUENGAPID:                         "RANUENGAPID",
	idRRCEstablishmentCause:               "RRCEstablishmentCause",
	idRelativeAMFCapacity:                 "RelativeAMFCapacity",
	idResetType:                           "ResetType",
	idServedGUAMIList:                     "ServedGUAMIList",
	idSourceAMFUENGAPID:                   "SourceAMFUENGAPID",
	idSONConfigurationTransferDL:          "SONConfigurationTransferDL",
	idSONConfigurationTransferUL:          "SONConfigurationTransferUL",
	idPagingDRX:                           "PagingDRX",
	idPagingOrigin:                        "PagingOrigin",
	idPagingPriority:                      "PagingPriority",
	idTAIListForPaging:                    "TAIListForPaging",
	idUEPagingIdentity:                    "UEPagingIdentity",
	idUnavailableGUAMIList:                "UnavailableGUAMIList",
	idUserLocationInformation:             "UserLocationInformation",
	idUERadioCapabilityForPaging:          "UERadioCapabilityForPaging",
	idSupportedTAList:                     "SupportedTAList",
	idTimeToWait:                          "TimeToWait",
	idNGRANTNLAssociationToRemoveList:     "NGRANTNLAssociationToRemoveList",
	idUEAssociatedLogicalNGConnectionList: "UEAssociatedLogicalNGConnectionList",
	idUEContextRequest:                    "UEContextRequest",
	idUERetentionInformation:              "UERetentionInformation",
}

func (id ProtocolIEID) String() string {
	if name, ok := protocolIENames[id]; ok {
		return fmt.Sprintf("%s (%d)", name, uint16(id))
	}

	return fmt.Sprintf("ProtocolIEID(%d)", uint16(id))
}
