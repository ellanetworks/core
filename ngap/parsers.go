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
	{"ParseNGSetupFailure", func(v []byte) error { _, err := ParseNGSetupFailure(v); return err }},
	{"ParseNGSetupRequest", func(v []byte) error { _, err := ParseNGSetupRequest(v); return err }},
	{"ParseNGSetupResponse", func(v []byte) error { _, err := ParseNGSetupResponse(v); return err }},
}
