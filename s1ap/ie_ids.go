// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

// ProtocolIE-ID values (TS 36.413 §9.3, S1AP-Constants).
const (
	idMMEUES1APID                               ProtocolIEID = 0
	idCause                                     ProtocolIEID = 2
	idENBUES1APID                               ProtocolIEID = 8
	idERABReleaseItemBearerRelComp              ProtocolIEID = 15
	idERABToBeSetupListBearerSUReq              ProtocolIEID = 16
	idERABToBeSetupItemBearerSUReq              ProtocolIEID = 17
	idERABToBeSwitchedDLList                    ProtocolIEID = 22
	idERABToBeSwitchedDLItem                    ProtocolIEID = 23
	idERABToBeSetupListCtxtSUReq                ProtocolIEID = 24
	idNASPDU                                    ProtocolIEID = 26
	idERABSetupListBearerSURes                  ProtocolIEID = 28
	idERABFailedToSetupListBearerSURes          ProtocolIEID = 29
	idERABToBeModifiedListBearerModReq          ProtocolIEID = 30
	idERABModifyListBearerModRes                ProtocolIEID = 31
	idERABFailedToModifyList                    ProtocolIEID = 32
	idERABToBeReleasedList                      ProtocolIEID = 33
	idERABFailedToReleaseList                   ProtocolIEID = 34
	idERABItem                                  ProtocolIEID = 35
	idERABToBeModifiedItemBearerModReq          ProtocolIEID = 36
	idERABModifyItemBearerModRes                ProtocolIEID = 37
	idERABReleaseListBearerRelComp              ProtocolIEID = 69
	idERABSetupItemBearerSURes                  ProtocolIEID = 39
	idSecurityContext                           ProtocolIEID = 40
	idUEPagingID                                ProtocolIEID = 43
	idTAIList                                   ProtocolIEID = 46
	idTAIItem                                   ProtocolIEID = 47
	idERABFailedToSetupListCtxtSU               ProtocolIEID = 48
	idERABSetupItemCtxtSURes                    ProtocolIEID = 50
	idERABSetupListCtxtSURes                    ProtocolIEID = 51
	idERABToBeSetupItemCtxtSUReq                ProtocolIEID = 52
	idCriticalityDiagnostics                    ProtocolIEID = 58
	idGlobalENBID                               ProtocolIEID = 59
	idENBname                                   ProtocolIEID = 60
	idMMEname                                   ProtocolIEID = 61
	idSupportedTAs                              ProtocolIEID = 64
	idTimeToWait                                ProtocolIEID = 65
	idUEAggregateMaximumBitrate                 ProtocolIEID = 66
	idTAI                                       ProtocolIEID = 67
	idSecurityKey                               ProtocolIEID = 73
	idUERadioCapability                         ProtocolIEID = 74
	idGUMMEI                                    ProtocolIEID = 75
	idUEIdentityIndexValue                      ProtocolIEID = 80
	idRelativeMMECapacity                       ProtocolIEID = 87
	idSourceMMEUES1APID                         ProtocolIEID = 88
	idUEAssociatedLogicalS1ConnectionItem       ProtocolIEID = 91
	idResetType                                 ProtocolIEID = 92
	idUEAssociatedLogicalS1ConnectionListResAck ProtocolIEID = 93
	idSTMSI                                     ProtocolIEID = 96
	idUES1APIDs                                 ProtocolIEID = 99
	idEUTRANCGI                                 ProtocolIEID = 100
	idServedGUMMEIs                             ProtocolIEID = 105
	idUESecurityCapabilities                    ProtocolIEID = 107
	idCNDomain                                  ProtocolIEID = 109
	idUERadioCapabilityForPaging                ProtocolIEID = 117
	idRRCEstablishmentCause                     ProtocolIEID = 134
	idDefaultPagingDRX                          ProtocolIEID = 137
)
