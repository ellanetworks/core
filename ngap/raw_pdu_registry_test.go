// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

var typedCodecs = map[typedKey]func([]byte) (typedMessage, error){
	{TriggeringInitiatingMessage, ProcAMFConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseAMFConfigurationUpdate(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcAMFConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseAMFConfigurationUpdateAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcAMFConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseAMFConfigurationUpdateFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcAMFStatusIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseAMFStatusIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUplinkRANConfigurationTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkRANConfigurationTransfer(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkRANConfigurationTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkRANConfigurationTransfer(v)
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
	{TriggeringInitiatingMessage, ProcLocationReportingControl}: func(v []byte) (typedMessage, error) {
		m, err := ParseLocationReportingControl(v)
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
	{TriggeringInitiatingMessage, ProcNGReset}: func(v []byte) (typedMessage, error) {
		m, err := ParseNGReset(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcNGReset}: func(v []byte) (typedMessage, error) {
		m, err := ParseNGResetAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcNGSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseNGSetupRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcNGSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseNGSetupFailure(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcNGSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParseNGSetupResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkUEAssociatedNRPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkUEAssociatedNRPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUplinkUEAssociatedNRPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkUEAssociatedNRPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkNonUEAssociatedNRPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkNonUEAssociatedNRPPaTransport(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcUplinkNonUEAssociatedNRPPaTransport}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkNonUEAssociatedNRPPaTransport(v)
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
	{TriggeringInitiatingMessage, ProcPDUSessionResourceModify}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceModifyRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcPDUSessionResourceModify}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceModifyResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPDUSessionResourceModifyIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceModifyIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcPDUSessionResourceModifyIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceModifyConfirm(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPDUSessionResourceNotify}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceNotify(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPDUSessionResourceRelease}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceReleaseCommand(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcPDUSessionResourceRelease}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceReleaseResponse(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcPDUSessionResourceSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceSetupRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcPDUSessionResourceSetup}: func(v []byte) (typedMessage, error) {
		m, err := ParsePDUSessionResourceSetupResponse(v)
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
	{TriggeringInitiatingMessage, ProcPWSCancel}: func(v []byte) (typedMessage, error) {
		m, err := ParsePWSCancelRequest(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcPWSCancel}: func(v []byte) (typedMessage, error) {
		m, err := ParsePWSCancelResponse(v)
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
	{TriggeringInitiatingMessage, ProcRANConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseRANConfigurationUpdate(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringSuccessfulOutcome, ProcRANConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseRANConfigurationUpdateAcknowledge(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringUnsuccessfulOutcome, ProcRANConfigurationUpdate}: func(v []byte) (typedMessage, error) {
		m, err := ParseRANConfigurationUpdateFailure(v)
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
	{TriggeringInitiatingMessage, ProcUplinkRANStatusTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseUplinkRANStatusTransfer(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
	{TriggeringInitiatingMessage, ProcDownlinkRANStatusTransfer}: func(v []byte) (typedMessage, error) {
		m, err := ParseDownlinkRANStatusTransfer(v)
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
	{TriggeringInitiatingMessage, ProcUERadioCapabilityInfoIndication}: func(v []byte) (typedMessage, error) {
		m, err := ParseUERadioCapabilityInfoIndication(v)
		if err != nil {
			return nil, err
		}

		return m, nil
	},
}

func sampleMessages() []typedMessage {
	return []typedMessage{
		goldErrorIndicationFull(),
		errorIndicationWithSTMSI(),
		goldNGResetPart(),
		goldNGResetAcknowledge(),
		goldRequest(),
		goldResponse(),
		goldFailure(),
	}
}
