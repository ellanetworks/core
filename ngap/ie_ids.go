// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// ProtocolIE-ID values (TS 38.413, NGAP-Constants).
const (
	idAMFName                             ProtocolIEID = 1
	idAMFUENGAPID                         ProtocolIEID = 10
	idCause                               ProtocolIEID = 15
	idCriticalityDiagnostics              ProtocolIEID = 19
	idDefaultPagingDRX                    ProtocolIEID = 21
	idFiveGSTMSI                          ProtocolIEID = 26
	idGlobalRANNodeID                     ProtocolIEID = 27
	idPLMNSupportList                     ProtocolIEID = 80
	idPagingDRX                           ProtocolIEID = 50
	idPagingOrigin                        ProtocolIEID = 51
	idPagingPriority                      ProtocolIEID = 52
	idRANNodeName                         ProtocolIEID = 82
	idRANUENGAPID                         ProtocolIEID = 85
	idRelativeAMFCapacity                 ProtocolIEID = 86
	idResetType                           ProtocolIEID = 88
	idServedGUAMIList                     ProtocolIEID = 96
	idSourceAMFUENGAPID                   ProtocolIEID = 100
	idSONConfigurationTransferDL          ProtocolIEID = 98
	idSONConfigurationTransferUL          ProtocolIEID = 99
	idSupportedTAList                     ProtocolIEID = 102
	idTAIListForPaging                    ProtocolIEID = 103
	idTimeToWait                          ProtocolIEID = 107
	idNGRANTNLAssociationToRemoveList     ProtocolIEID = 167
	idUEAssociatedLogicalNGConnectionList ProtocolIEID = 111
	idUEPagingIdentity                    ProtocolIEID = 115
	idUERadioCapabilityForPaging          ProtocolIEID = 118
	idUERetentionInformation              ProtocolIEID = 147
)
