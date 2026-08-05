// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

type messageParser struct {
	Name  string
	Parse func(value []byte) error
}

// Every exported ParseXxx in this package.
var messageParsers = []messageParser{
	{"ParseDownlinkNASTransport", func(v []byte) error { _, err := ParseDownlinkNASTransport(v); return err }},
	{"ParseDownlinkUEAssociatedLPPaTransport", func(v []byte) error { _, err := ParseDownlinkUEAssociatedLPPaTransport(v); return err }},
	{"ParseENBConfigurationTransfer", func(v []byte) error { _, err := ParseENBConfigurationTransfer(v); return err }},
	{"ParseMMEConfigurationTransfer", func(v []byte) error { _, err := ParseMMEConfigurationTransfer(v); return err }},
	{"ParseENBConfigurationUpdate", func(v []byte) error { _, err := ParseENBConfigurationUpdate(v); return err }},
	{"ParseENBConfigurationUpdateAcknowledge", func(v []byte) error { _, err := ParseENBConfigurationUpdateAcknowledge(v); return err }},
	{"ParseENBConfigurationUpdateFailure", func(v []byte) error { _, err := ParseENBConfigurationUpdateFailure(v); return err }},
	{"ParseENBStatusTransfer", func(v []byte) error { _, err := ParseENBStatusTransfer(v); return err }},
	{"ParseERABModificationIndication", func(v []byte) error { _, err := ParseERABModificationIndication(v); return err }},
	{"ParseERABModifyRequest", func(v []byte) error { _, err := ParseERABModifyRequest(v); return err }},
	{"ParseERABModifyResponse", func(v []byte) error { _, err := ParseERABModifyResponse(v); return err }},
	{"ParseERABReleaseCommand", func(v []byte) error { _, err := ParseERABReleaseCommand(v); return err }},
	{"ParseERABReleaseResponse", func(v []byte) error { _, err := ParseERABReleaseResponse(v); return err }},
	{"ParseERABSetupRequest", func(v []byte) error { _, err := ParseERABSetupRequest(v); return err }},
	{"ParseERABSetupResponse", func(v []byte) error { _, err := ParseERABSetupResponse(v); return err }},
	{"ParseErrorIndication", func(v []byte) error { _, err := ParseErrorIndication(v); return err }},
	{"ParseHandoverCancel", func(v []byte) error { _, err := ParseHandoverCancel(v); return err }},
	{"ParseHandoverCancelAcknowledge", func(v []byte) error { _, err := ParseHandoverCancelAcknowledge(v); return err }},
	{"ParseHandoverCommand", func(v []byte) error { _, err := ParseHandoverCommand(v); return err }},
	{"ParseHandoverFailure", func(v []byte) error { _, err := ParseHandoverFailure(v); return err }},
	{"ParseHandoverNotify", func(v []byte) error { _, err := ParseHandoverNotify(v); return err }},
	{"ParseHandoverPreparationFailure", func(v []byte) error { _, err := ParseHandoverPreparationFailure(v); return err }},
	{"ParseHandoverRequest", func(v []byte) error { _, err := ParseHandoverRequest(v); return err }},
	{"ParseHandoverRequestAcknowledge", func(v []byte) error { _, err := ParseHandoverRequestAcknowledge(v); return err }},
	{"ParseHandoverRequired", func(v []byte) error { _, err := ParseHandoverRequired(v); return err }},
	{"ParseInitialContextSetupFailure", func(v []byte) error { _, err := ParseInitialContextSetupFailure(v); return err }},
	{"ParseInitialContextSetupRequest", func(v []byte) error { _, err := ParseInitialContextSetupRequest(v); return err }},
	{"ParseInitialContextSetupResponse", func(v []byte) error { _, err := ParseInitialContextSetupResponse(v); return err }},
	{"ParseInitialUEMessage", func(v []byte) error { _, err := ParseInitialUEMessage(v); return err }},
	{"ParseLocationReport", func(v []byte) error { _, err := ParseLocationReport(v); return err }},
	{"ParseMMEStatusTransfer", func(v []byte) error { _, err := ParseMMEStatusTransfer(v); return err }},
	{"ParseNASNonDeliveryIndication", func(v []byte) error { _, err := ParseNASNonDeliveryIndication(v); return err }},
	{"ParsePaging", func(v []byte) error { _, err := ParsePaging(v); return err }},
	{"ParsePathSwitchRequest", func(v []byte) error { _, err := ParsePathSwitchRequest(v); return err }},
	{"ParsePathSwitchRequestAcknowledge", func(v []byte) error { _, err := ParsePathSwitchRequestAcknowledge(v); return err }},
	{"ParsePathSwitchRequestFailure", func(v []byte) error { _, err := ParsePathSwitchRequestFailure(v); return err }},
	{"ParseReset", func(v []byte) error { _, err := ParseReset(v); return err }},
	{"ParseResetAcknowledge", func(v []byte) error { _, err := ParseResetAcknowledge(v); return err }},
	{"ParseS1SetupFailure", func(v []byte) error { _, err := ParseS1SetupFailure(v); return err }},
	{"ParseS1SetupRequest", func(v []byte) error { _, err := ParseS1SetupRequest(v); return err }},
	{"ParseS1SetupResponse", func(v []byte) error { _, err := ParseS1SetupResponse(v); return err }},
	{"ParseUECapabilityInfoIndication", func(v []byte) error { _, err := ParseUECapabilityInfoIndication(v); return err }},
	{"ParseUEContextReleaseCommand", func(v []byte) error { _, err := ParseUEContextReleaseCommand(v); return err }},
	{"ParseUEContextReleaseComplete", func(v []byte) error { _, err := ParseUEContextReleaseComplete(v); return err }},
	{"ParseUEContextReleaseRequest", func(v []byte) error { _, err := ParseUEContextReleaseRequest(v); return err }},
	{"ParseUplinkNASTransport", func(v []byte) error { _, err := ParseUplinkNASTransport(v); return err }},
	{"ParseDownlinkNonUEAssociatedLPPaTransport", func(v []byte) error {
		_, err := ParseDownlinkNonUEAssociatedLPPaTransport(v)
		return err
	}},
	{"ParseUplinkNonUEAssociatedLPPaTransport", func(v []byte) error {
		_, err := ParseUplinkNonUEAssociatedLPPaTransport(v)
		return err
	}},
	{"ParseUplinkUEAssociatedLPPaTransport", func(v []byte) error { _, err := ParseUplinkUEAssociatedLPPaTransport(v); return err }},
}
