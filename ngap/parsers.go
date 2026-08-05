// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

type messageParser struct {
	Name  string
	Parse func(value []byte) error
}

// Every exported ParseXxx in this package.
var messageParsers = []messageParser{
	{"ParseAMFStatusIndication", func(v []byte) error { _, err := ParseAMFStatusIndication(v); return err }},
	{"ParseErrorIndication", func(v []byte) error { _, err := ParseErrorIndication(v); return err }},
	{"ParseNGReset", func(v []byte) error { _, err := ParseNGReset(v); return err }},
	{"ParseNGResetAcknowledge", func(v []byte) error { _, err := ParseNGResetAcknowledge(v); return err }},
	{"ParseNGSetupFailure", func(v []byte) error { _, err := ParseNGSetupFailure(v); return err }},
	{"ParseNGSetupRequest", func(v []byte) error { _, err := ParseNGSetupRequest(v); return err }},
	{"ParseNGSetupResponse", func(v []byte) error { _, err := ParseNGSetupResponse(v); return err }},
	{"ParseDownlinkRANConfigurationTransfer", func(v []byte) error {
		_, err := ParseDownlinkRANConfigurationTransfer(v)
		return err
	}},
	{"ParseUplinkRANConfigurationTransfer", func(v []byte) error {
		_, err := ParseUplinkRANConfigurationTransfer(v)
		return err
	}},
	{"ParseDownlinkNASTransport", func(v []byte) error { _, err := ParseDownlinkNASTransport(v); return err }},
	{"ParseInitialUEMessage", func(v []byte) error { _, err := ParseInitialUEMessage(v); return err }},
	{"ParseInitialContextSetupRequest", func(v []byte) error { _, err := ParseInitialContextSetupRequest(v); return err }},
	{"ParseInitialContextSetupResponse", func(v []byte) error { _, err := ParseInitialContextSetupResponse(v); return err }},
	{"ParseInitialContextSetupFailure", func(v []byte) error { _, err := ParseInitialContextSetupFailure(v); return err }},
	{"ParseUERadioCapabilityInfoIndication", func(v []byte) error { _, err := ParseUERadioCapabilityInfoIndication(v); return err }},
	{"ParsePDUSessionResourceSetupRequest", func(v []byte) error { _, err := ParsePDUSessionResourceSetupRequest(v); return err }},
	{"ParsePDUSessionResourceSetupResponse", func(v []byte) error { _, err := ParsePDUSessionResourceSetupResponse(v); return err }},
	{"ParsePDUSessionResourceReleaseCommand", func(v []byte) error { _, err := ParsePDUSessionResourceReleaseCommand(v); return err }},
	{"ParsePDUSessionResourceReleaseResponse", func(v []byte) error { _, err := ParsePDUSessionResourceReleaseResponse(v); return err }},
	{"ParsePDUSessionResourceModifyRequest", func(v []byte) error { _, err := ParsePDUSessionResourceModifyRequest(v); return err }},
	{"ParsePDUSessionResourceModifyResponse", func(v []byte) error { _, err := ParsePDUSessionResourceModifyResponse(v); return err }},
	{"ParsePDUSessionResourceModifyIndication", func(v []byte) error { _, err := ParsePDUSessionResourceModifyIndication(v); return err }},
	{"ParsePDUSessionResourceModifyConfirm", func(v []byte) error { _, err := ParsePDUSessionResourceModifyConfirm(v); return err }},
	{"ParseHandoverRequired", func(v []byte) error {
		_, err := ParseHandoverRequired(v)
		return err
	}},
	{"ParseHandoverCommand", func(v []byte) error {
		_, err := ParseHandoverCommand(v)
		return err
	}},
	{"ParseHandoverPreparationFailure", func(v []byte) error {
		_, err := ParseHandoverPreparationFailure(v)
		return err
	}},
	{"ParseHandoverRequest", func(v []byte) error {
		_, err := ParseHandoverRequest(v)
		return err
	}},
	{"ParseHandoverRequestAcknowledge", func(v []byte) error {
		_, err := ParseHandoverRequestAcknowledge(v)
		return err
	}},
	{"ParseHandoverFailure", func(v []byte) error {
		_, err := ParseHandoverFailure(v)
		return err
	}},
	{"ParseHandoverCancel", func(v []byte) error {
		_, err := ParseHandoverCancel(v)
		return err
	}},
	{"ParseHandoverCancelAcknowledge", func(v []byte) error {
		_, err := ParseHandoverCancelAcknowledge(v)
		return err
	}},
	{"ParseUplinkRANStatusTransfer", func(v []byte) error {
		_, err := ParseUplinkRANStatusTransfer(v)
		return err
	}},
	{"ParseDownlinkRANStatusTransfer", func(v []byte) error {
		_, err := ParseDownlinkRANStatusTransfer(v)
		return err
	}},
	{"ParseDownlinkUEAssociatedNRPPaTransport", func(v []byte) error {
		_, err := ParseDownlinkUEAssociatedNRPPaTransport(v)
		return err
	}},
	{"ParseDownlinkNonUEAssociatedNRPPaTransport", func(v []byte) error {
		_, err := ParseDownlinkNonUEAssociatedNRPPaTransport(v)
		return err
	}},
	{"ParseUplinkNonUEAssociatedNRPPaTransport", func(v []byte) error {
		_, err := ParseUplinkNonUEAssociatedNRPPaTransport(v)
		return err
	}},
	{"ParseUplinkUEAssociatedNRPPaTransport", func(v []byte) error {
		_, err := ParseUplinkUEAssociatedNRPPaTransport(v)
		return err
	}},
	{"ParseLocationReport", func(v []byte) error {
		_, err := ParseLocationReport(v)
		return err
	}},
	{"ParseLocationReportingControl", func(v []byte) error {
		_, err := ParseLocationReportingControl(v)
		return err
	}},
	{"ParsePathSwitchRequest", func(v []byte) error {
		_, err := ParsePathSwitchRequest(v)
		return err
	}},
	{"ParsePathSwitchRequestAcknowledge", func(v []byte) error {
		_, err := ParsePathSwitchRequestAcknowledge(v)
		return err
	}},
	{"ParsePathSwitchRequestFailure", func(v []byte) error {
		_, err := ParsePathSwitchRequestFailure(v)
		return err
	}},
	{"ParseHandoverNotify", func(v []byte) error {
		_, err := ParseHandoverNotify(v)
		return err
	}},
	{"ParsePDUSessionResourceNotify", func(v []byte) error { _, err := ParsePDUSessionResourceNotify(v); return err }},
	{"ParseNASNonDeliveryIndication", func(v []byte) error { _, err := ParseNASNonDeliveryIndication(v); return err }},
	{"ParsePaging", func(v []byte) error { _, err := ParsePaging(v); return err }},
	{"ParseUplinkNASTransport", func(v []byte) error { _, err := ParseUplinkNASTransport(v); return err }},
	{"ParseUEContextReleaseCommand", func(v []byte) error { _, err := ParseUEContextReleaseCommand(v); return err }},
	{"ParseUEContextReleaseComplete", func(v []byte) error { _, err := ParseUEContextReleaseComplete(v); return err }},
	{"ParseUEContextReleaseRequest", func(v []byte) error { _, err := ParseUEContextReleaseRequest(v); return err }},
	{"ParseRANConfigurationUpdate", func(v []byte) error { _, err := ParseRANConfigurationUpdate(v); return err }},
	{"ParseRANConfigurationUpdateAcknowledge", func(v []byte) error {
		_, err := ParseRANConfigurationUpdateAcknowledge(v)
		return err
	}},
	{"ParseRANConfigurationUpdateFailure", func(v []byte) error {
		_, err := ParseRANConfigurationUpdateFailure(v)
		return err
	}},
}

// transferParsers are the §9.3.4 transfer parsers. They take a
// TransferContainer rather than a message body, so they are registered apart
// from messageParsers; the fuzzer drives both.
var transferParsers = []messageParser{
	{"ParsePDUSessionResourceSetupUnsuccessfulTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceSetupUnsuccessfulTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceModifyIndicationUnsuccessfulTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceModifyIndicationUnsuccessfulTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceSetupRequestTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceSetupRequestTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceSetupResponseTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceSetupResponseTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceReleaseCommandTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceReleaseCommandTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceModifyRequestTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceModifyRequestTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceModifyConfirmTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceModifyConfirmTransfer(v)
		return err
	}},
	{"ParsePDUSessionResourceModifyIndicationTransfer", func(v []byte) error {
		_, err := ParsePDUSessionResourceModifyIndicationTransfer(v)
		return err
	}},
	{"ParseHandoverCommandTransfer", func(v []byte) error {
		_, err := ParseHandoverCommandTransfer(v)
		return err
	}},
	{"ParseHandoverPreparationUnsuccessfulTransfer", func(v []byte) error {
		_, err := ParseHandoverPreparationUnsuccessfulTransfer(v)
		return err
	}},
	{"ParseHandoverResourceAllocationUnsuccessfulTransfer", func(v []byte) error {
		_, err := ParseHandoverResourceAllocationUnsuccessfulTransfer(v)
		return err
	}},
	{"ParseHandoverRequestAcknowledgeTransfer", func(v []byte) error {
		_, err := ParseHandoverRequestAcknowledgeTransfer(v)
		return err
	}},
	{"ParseHandoverRequiredTransfer", func(v []byte) error {
		_, err := ParseHandoverRequiredTransfer(v)
		return err
	}},
	{"ParsePathSwitchRequestTransfer", func(v []byte) error {
		_, err := ParsePathSwitchRequestTransfer(v)
		return err
	}},
	{"ParsePathSwitchRequestUnsuccessfulTransfer", func(v []byte) error {
		_, err := ParsePathSwitchRequestUnsuccessfulTransfer(v)
		return err
	}},
	{"ParsePathSwitchRequestAcknowledgeTransfer", func(v []byte) error {
		_, err := ParsePathSwitchRequestAcknowledgeTransfer(v)
		return err
	}},
	{"ParsePathSwitchRequestSetupFailedTransfer", func(v []byte) error {
		_, err := ParsePathSwitchRequestSetupFailedTransfer(v)
		return err
	}},
}
