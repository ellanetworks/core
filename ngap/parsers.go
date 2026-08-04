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
