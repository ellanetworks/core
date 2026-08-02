// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

type messageParser struct {
	Name  string
	Parse func(value []byte) error
}

// messageParsers lists every exported ParseXxx in this package;
// TestEveryParserIsRegistered fails if one is missing.
var messageParsers = []messageParser{
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
