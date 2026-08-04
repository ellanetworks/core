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
}
