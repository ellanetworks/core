// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

var typedCodecs = map[typedKey]func([]byte) (typedMessage, error){
	{TriggeringInitiatingMessage, ProcENBConfigurationTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseENBConfigurationTransfer(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcMMEConfigurationTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseMMEConfigurationTransfer(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcENBConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseENBConfigurationUpdate(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcENBConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseENBConfigurationUpdateAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcENBConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseENBConfigurationUpdateFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcERABModificationIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABModificationIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcERABModificationIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABModificationConfirm(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcERABModify}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABModifyRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcERABModify}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABModifyResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcERABRelease}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABReleaseCommand(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcERABRelease}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABReleaseResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcERABReleaseIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABReleaseIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcERABSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABSetupRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcERABSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseERABSetupResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcErrorIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseErrorIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcHandoverCancel}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverCancel(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcHandoverCancel}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverCancelAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcHandoverNotification}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverNotify(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcHandoverPreparation}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverRequired(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcHandoverPreparation}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverCommand(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcHandoverPreparation}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverPreparationFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcHandoverResourceAllocation}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcHandoverResourceAllocation}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverRequestAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcHandoverResourceAllocation}: func(v []byte) (typedMessage, error) {
		m, err := ParseHandoverFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcInitialContextSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseInitialContextSetupRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcInitialContextSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseInitialContextSetupResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcInitialContextSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseInitialContextSetupFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcLocationReport}: func(v []byte) (typedMessage, error) {
		m, err := ParseLocationReport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkUEAssociatedLPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkUEAssociatedLPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUplinkUEAssociatedLPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkUEAssociatedLPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkNonUEAssociatedLPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkNonUEAssociatedLPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUplinkNonUEAssociatedLPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkNonUEAssociatedLPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcMMEConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseMMEConfigurationUpdate(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcMMEConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseMMEConfigurationUpdateAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcMMEConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseMMEConfigurationUpdateFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcNASNonDeliveryIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseNASNonDeliveryIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcInitialUEMessage}: func(v []byte) (typedMessage, error) {
		m, err := ParseInitialUEMessage(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUplinkNASTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkNASTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkNASTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkNASTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcOverloadStart}: func(v []byte) (typedMessage, error) {
		m, err := ParseOverloadStart(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcOverloadStop}: func(v []byte) (typedMessage, error) {
		m, err := ParseOverloadStop(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPaging}: func(v []byte) (typedMessage, error) {
		m, err := ParsePaging(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPathSwitchRequest}: func(v []byte) (typedMessage, error) {
		m, err := ParsePathSwitchRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcPathSwitchRequest}: func(v []byte) (typedMessage, error) {
		m, err := ParsePathSwitchRequestAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcPathSwitchRequest}: func(v []byte) (typedMessage, error) {
		m, err := ParsePathSwitchRequestFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcWriteReplaceWarning}: func(v []byte) (typedMessage, error) {
		m, err := ParseWriteReplaceWarningRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcWriteReplaceWarning}: func(v []byte) (typedMessage, error) {
		m, err := ParseWriteReplaceWarningResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcKill}: func(v []byte) (typedMessage, error) {
		m, err := ParseKillRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcKill}: func(v []byte) (typedMessage, error) {
		m, err := ParseKillResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPWSRestartIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParsePWSRestartIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPWSFailureIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParsePWSFailureIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcRerouteNASRequest}: func(v []byte) (typedMessage, error) {
		m, err := ParseRerouteNASRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcReset}: func(v []byte) (typedMessage, error) {
		m, err := ParseReset(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcReset}: func(v []byte) (typedMessage, error) {
		m, err := ParseResetAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcS1Setup}: func(v []byte) (typedMessage, error) {
		m, err := ParseS1SetupRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcS1Setup}: func(v []byte) (typedMessage, error) {
		m, err := ParseS1SetupFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcS1Setup}: func(v []byte) (typedMessage, error) {
		m, err := ParseS1SetupResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcENBStatusTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseENBStatusTransfer(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcMMEStatusTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseMMEStatusTransfer(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcTraceStart}: func(v []byte) (typedMessage, error) {
		m, err := ParseTraceStart(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcTraceFailureIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseTraceFailureIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUECapabilityInfoIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseUECapabilityInfoIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUEContextModification}: func(v []byte) (typedMessage, error) {
		m, err := ParseUEContextModificationRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcUEContextModification}: func(v []byte) (typedMessage, error) {
		m, err := ParseUEContextModificationResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcUEContextModification}: func(v []byte) (typedMessage, error) {
		m, err := ParseUEContextModificationFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUEContextRelease}: func(v []byte) (typedMessage, error) {
		m, err := ParseUEContextReleaseCommand(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcUEContextRelease}: func(v []byte) (typedMessage, error) {
		m, err := ParseUEContextReleaseComplete(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUEContextReleaseRequest}: func(v []byte) (typedMessage, error) {
		m, err := ParseUEContextReleaseRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
}

func sampleMessages() []typedMessage {
	return []typedMessage{
		&DownlinkNASTransport{NASPDU: NASPDU{0x07}},
		&DownlinkUEAssociatedLPPaTransport{},
		&ENBConfigurationUpdate{},
		&ENBConfigurationUpdateAcknowledge{},
		&ENBConfigurationUpdateFailure{Cause: new(Cause)},
		&ENBStatusTransfer{Container: StatusTransferContainer{0xaa}},
		&ERABModificationConfirm{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&ERABModificationIndication{ToBeModified: []ERABToBeModifiedItemBearerModInd{{ERABID: 1, TransportLayerAddress: goldTLA(), DLGTPTEID: 1}}},
		&ERABModifyRequest{ERABToBeModified: []ERABToBeModifiedItemBearerModReq{{ERABID: 1, QoS: goldQoS(), NASPDU: NASPDU{0x01}}}},
		&ERABModifyResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&ERABReleaseCommand{ERABToBeReleased: []ERABItem{goldERABItem()}},
		&ERABReleaseIndication{ERABReleased: []ERABItem{goldERABItem()}},
		&ERABModificationConfirm{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), FailedToModify: []ERABItem{goldERABItem()}, ToBeReleased: []ERABItem{goldERABItem()}},
		&ERABReleaseResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&ERABSetupRequest{ERABToBeSetup: []ERABToBeSetupItemBearerSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, NASPDU: NASPDU{0x01}}}},
		&ERABSetupResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&HandoverCancel{Cause: new(Cause)},
		&HandoverCancelAcknowledge{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&HandoverCommand{TargetToSource: TransparentContainer{0x00}},
		&HandoverFailure{MMEUES1APID: new(MMEUES1APID), Cause: new(Cause)},
		&HandoverNotify{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI)},
		&HandoverPreparationFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)},
		&HandoverRequest{Cause: new(Cause), SourceToTarget: TransparentContainer{0x00}, ERABToBeSetup: []ERABToBeSetupItemHOReq{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1, QoS: goldQoS()}}},
		&HandoverRequestAcknowledge{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), TargetToSource: TransparentContainer{0x00}, ERABAdmitted: []ERABAdmittedItem{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}}},
		&HandoverRequired{Cause: new(Cause), SourceToTarget: TransparentContainer{0x00}},
		&InitialContextSetupFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)},
		&InitialContextSetupRequest{ERABToBeSetup: []ERABToBeSetupItemCtxtSUReq{{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1}}},
		&InitialContextSetupResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), ERABSetup: []ERABSetupItemCtxtSURes{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}}},
		&InitialUEMessage{NASPDU: NASPDU{0x07}, EUTRANCGI: new(EUTRANCGI), RRCEstablishmentCause: new(RRCEstablishmentCause)},
		&LocationReport{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI), RequestType: new(RequestType)},
		&MMEConfigurationTransfer{},
		&MMEConfigurationUpdate{RelativeMMECapacity: new(uint8)},
		&MMEConfigurationUpdateAcknowledge{},
		&MMEConfigurationUpdateFailure{Cause: new(Cause)},
		&MMEStatusTransfer{Container: StatusTransferContainer{0xaa}},
		&NASNonDeliveryIndication{NASPDU: NASPDU{}, Cause: new(Cause)},
		&Paging{UEIdentityIndexValue: new(uint16), STMSI: new(STMSI), CNDomain: new(CNDomain), TAIList: []TAI{{PLMNIdentity: goldPLMN(), TAC: 1}}},
		&PathSwitchRequest{EUTRANCGI: new(EUTRANCGI), TAI: new(TAI), UESecurityCapabilities: new(UESecurityCapabilities), ERABToBeSwitchedDL: []ERABToBeSwitchedDLItem{{ERABID: 1, TransportLayerAddress: goldTLA(), GTPTEID: 1}}},
		&PathSwitchRequestAcknowledge{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&PathSwitchRequestFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)},
		&Reset{Cause: new(Cause), ResetType: ResetType{All: true}},
		&ResetAcknowledge{},
		&S1SetupFailure{Cause: new(Cause)},
		&S1SetupRequest{DefaultPagingDRX: new(PagingDRX), SupportedTAs: SupportedTAs{{TAC: 1, BroadcastPLMNs: BPLMNs{goldPLMN()}}}},
		&S1SetupResponse{RelativeMMECapacity: new(uint8), ServedGUMMEIs: ServedGUMMEIs{{ServedPLMNs: []PLMNIdentity{goldPLMN()}, ServedGroupIDs: []MMEGroupID{{0x80, 0x01}}, ServedMMECs: []MMECode{1}}}},
		&UECapabilityInfoIndication{UERadioCapability: []byte{}},
		&UEContextModificationFailure{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID), Cause: new(Cause)},
		&UEContextModificationRequest{UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{}},
		&UEContextModificationResponse{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&UEContextReleaseCommand{Cause: new(Cause)},
		&UEContextReleaseComplete{MMEUES1APID: new(MMEUES1APID), ENBUES1APID: new(ENBUES1APID)},
		&UEContextReleaseRequest{Cause: new(Cause)},
		&UplinkNASTransport{NASPDU: NASPDU{0x07}, EUTRANCGI: new(EUTRANCGI), TAI: new(TAI)},
		&UplinkUEAssociatedLPPaTransport{},
	}
}
