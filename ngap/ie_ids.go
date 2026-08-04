// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// ProtocolIE-ID values (TS 38.413, NGAP-Constants).
const (
	idAllowedNSSAI                               ProtocolIEID = 0
	idAMFName                                    ProtocolIEID = 1
	idAMFSetID                                   ProtocolIEID = 3
	idAMFUENGAPID                                ProtocolIEID = 10
	idCause                                      ProtocolIEID = 15
	idCriticalityDiagnostics                     ProtocolIEID = 19
	idDefaultPagingDRX                           ProtocolIEID = 21
	idFiveGSTMSI                                 ProtocolIEID = 26
	idGUAMI                                      ProtocolIEID = 28
	idGlobalRANNodeID                            ProtocolIEID = 27
	idNASPDU                                     ProtocolIEID = 38
	idPDUSessionResourceFailedToSetupListCxtRes  ProtocolIEID = 55
	idPDUSessionResourceFailedToSetupListSURes   ProtocolIEID = 58
	idPDUSessionResourceFailedToModifyListModRes ProtocolIEID = 54
	idPDUSessionResourceListCxtRelCpl            ProtocolIEID = 60
	idPDUSessionResourceSetupListCxtReq          ProtocolIEID = 71
	idPDUSessionResourceSetupListCxtRes          ProtocolIEID = 72
	idPDUSessionResourceModifyListModCfm         ProtocolIEID = 62
	idPDUSessionResourceModifyListModInd         ProtocolIEID = 63
	idPDUSessionResourceModifyListModReq         ProtocolIEID = 64
	idPDUSessionResourceModifyListModRes         ProtocolIEID = 65
	idPDUSessionResourceNotifyList               ProtocolIEID = 66
	idPDUSessionResourceReleasedListNot          ProtocolIEID = 67
	idPDUSessionResourceReleasedListRelRes       ProtocolIEID = 70
	idPDUSessionResourceSetupListSUReq           ProtocolIEID = 74
	idPDUSessionResourceSetupListSURes           ProtocolIEID = 75
	idPDUSessionResourceToReleaseListRelCmd      ProtocolIEID = 79
	idPDUSessionResourceFailedToSetupListCxtFail ProtocolIEID = 132
	idPDUSessionResourceFailedToModifyListModCfm ProtocolIEID = 131
	idPDUSessionResourceListCxtRelReq            ProtocolIEID = 133
	idPLMNSupportList                            ProtocolIEID = 80
	idPagingDRX                                  ProtocolIEID = 50
	idPagingOrigin                               ProtocolIEID = 51
	idPagingPriority                             ProtocolIEID = 52
	idRANNodeName                                ProtocolIEID = 82
	idRANUENGAPID                                ProtocolIEID = 85
	idRRCEstablishmentCause                      ProtocolIEID = 90
	idRelativeAMFCapacity                        ProtocolIEID = 86
	idResetType                                  ProtocolIEID = 88
	idServedGUAMIList                            ProtocolIEID = 96
	idSecurityKey                                ProtocolIEID = 94
	idSourceAMFUENGAPID                          ProtocolIEID = 100
	idSONConfigurationTransferDL                 ProtocolIEID = 98
	idSONConfigurationTransferUL                 ProtocolIEID = 99
	idSupportedTAList                            ProtocolIEID = 102
	idTAIListForPaging                           ProtocolIEID = 103
	idTimeToWait                                 ProtocolIEID = 107
	idNGRANTNLAssociationToRemoveList            ProtocolIEID = 167
	idUEAssociatedLogicalNGConnectionList        ProtocolIEID = 111
	idUEAggregateMaximumBitRate                  ProtocolIEID = 110
	idUEContextRequest                           ProtocolIEID = 112
	idUENGAPIDs                                  ProtocolIEID = 114
	idUEPagingIdentity                           ProtocolIEID = 115
	idUnavailableGUAMIList                       ProtocolIEID = 120
	idUERadioCapability                          ProtocolIEID = 117
	idUESecurityCapabilities                     ProtocolIEID = 119
	idUserLocationInformation                    ProtocolIEID = 121
	idUERadioCapabilityForPaging                 ProtocolIEID = 118
	idUERetentionInformation                     ProtocolIEID = 147

	// Carried inside the §9.3.4 PDU session transfers rather than a message body.
	idAdditionalULNGUUPTNLInformation   ProtocolIEID = 126
	idDataForwardingNotPossible         ProtocolIEID = 127
	idNetworkInstance                   ProtocolIEID = 129
	idPDUSessionAggregateMaximumBitRate ProtocolIEID = 130
	idPDUSessionType                    ProtocolIEID = 134
	idQosFlowSetupRequestList           ProtocolIEID = 136
	idSecurityIndication                ProtocolIEID = 138
	idULNGUUPTNLInformation             ProtocolIEID = 139
)
