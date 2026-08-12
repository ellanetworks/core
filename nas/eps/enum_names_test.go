// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// Name is what the diagnostic decoders render, so a value TS 24.301 assigns
// names itself and one it does not reports nothing.
func TestEnumNames(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"MsgAttachRequest", MsgAttachRequest.Name(), "ATTACH REQUEST"},
		{"MessageType(0)", MessageType(0).Name(), ""},

		{"MsgPDNConnectivityRequest", MsgPDNConnectivityRequest.Name(), "PDN CONNECTIVITY REQUEST"},
		{"ESMMessageType(0)", ESMMessageType(0).Name(), ""},

		{"PDEMM", PDEMM.Name(), "EMM"},
		{"ProtocolDiscriminator(0)", ProtocolDiscriminator(0).Name(), ""},

		{"SHTPlain", SHTPlain.Name(), "plain"},
		{"SHTServiceRequest", SHTServiceRequest.Name(), "service request"},
		{"SecurityHeaderType(9)", SecurityHeaderType(9).Name(), ""},

		{"AttachTypeCombined", AttachTypeCombined.Name(), "Combined EPS/IMSI attach"},
		{"AttachType(0)", AttachType(0).Name(), ""},

		{"AttachResultEPS", AttachResultEPS.Name(), "EPS only"},
		{"AttachResult(0)", AttachResult(0).Name(), ""},

		{"EPSUpdateTypePeriodic", EPSUpdateTypePeriodic.Name(), "Periodic updating"},
		{"EPSUpdateType(9)", EPSUpdateType(9).Name(), ""},

		{"EPSUpdateResultTA", EPSUpdateResultTA.Name(), "TA updated"},
		{"EPSUpdateResult(9)", EPSUpdateResult(9).Name(), ""},

		{"PDNTypeIPv4v6", PDNTypeIPv4v6.Name(), "IPv4v6"},
		{"PDNType(0)", PDNType(0).Name(), ""},

		{"RequestTypeHandover", RequestTypeHandover.Name(), "Handover"},
		{"RequestType(0)", RequestType(0).Name(), ""},

		{"EMMCauseMACFailure", EMMCauseMACFailure.Name(), "MAC failure"},
		{"EMMCause(0)", EMMCause(0).Name(), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("%s.Name() = %q, want %q", tc.what, tc.got, tc.want)
		}
	}
}

// EPS names the algorithms EEA/EIA; the 5GS names of the same identifiers live
// in nas/fgs (TS 24.301 table 9.9.3.23.1).
func TestAlgorithmNames(t *testing.T) {
	for _, tc := range []struct {
		what string
		got  string
		want string
	}{
		{"CipheringNull", CipheringAlgorithmName(nas.CipheringNull), "EEA0"},
		{"CipheringAES", CipheringAlgorithmName(nas.CipheringAES), "128-EEA2"},
		{"CipheringAlgorithm(9)", CipheringAlgorithmName(nas.CipheringAlgorithm(9)), ""},
		{"IntegrityNull", IntegrityAlgorithmName(nas.IntegrityNull), "EIA0"},
		{"IntegrityZUC", IntegrityAlgorithmName(nas.IntegrityZUC), "128-EIA3"},
		{"IntegrityAlgorithm(9)", IntegrityAlgorithmName(nas.IntegrityAlgorithm(9)), ""},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.what, tc.got, tc.want)
		}
	}
}
