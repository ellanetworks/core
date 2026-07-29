// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestValueIdentity asserts Parse(MarshalBinary(m)) equals m for messages built as
// struct literals rather than decoded. The fuzz round-trip property only
// exercises values that came from the wire; this covers the encode side, which
// is the direction Ella uses for every message it originates.
func TestValueIdentity(t *testing.T) {
	sd := [3]byte{0xaa, 0xbb, 0xcc}
	cause := GSMCauseRegularDeactivation

	cases := []struct {
		name  string
		msg   encoder
		parse func([]byte) (any, error)
	}{
		{
			"RegistrationReject", &RegistrationReject{Cause: GMMCausePLMNNotAllowed},
			func(b []byte) (any, error) { return ParseRegistrationReject(b) },
		},
		{
			"ServiceReject", &ServiceReject{Cause: GMMCauseCongestion},
			func(b []byte) (any, error) { return ParseServiceReject(b) },
		},
		{
			"GMMStatus", &GMMStatus{Cause: GMMCauseProtocolErrorUnspecified},
			func(b []byte) (any, error) { return ParseGMMStatus(b) },
		},
		{
			"IdentityRequest", &IdentityRequest{IdentityType: 1},
			func(b []byte) (any, error) { return ParseIdentityRequest(b) },
		},
		{
			"ServiceAccept", &ServiceAccept{PDUSessionStatus: &PSIBitmap{PSI: [16]bool{13: true}}},
			func(b []byte) (any, error) { return ParseServiceAccept(b) },
		},
		{
			"AuthenticationReject", &AuthenticationReject{},
			func(b []byte) (any, error) { return ParseAuthenticationReject(b) },
		},
		{
			"SecurityModeReject", &SecurityModeReject{Cause: GMMCauseUESecurityCapabilitiesMismatch},
			func(b []byte) (any, error) { return ParseSecurityModeReject(b) },
		},
		{
			"PDUSessionReleaseComplete", &PDUSessionReleaseComplete{PDUSessionID: 5, PTI: 1, Cause: &cause},
			func(b []byte) (any, error) { return ParsePDUSessionReleaseComplete(b) },
		},
		{
			"PDUSessionModificationComplete", &PDUSessionModificationComplete{PDUSessionID: 5, PTI: 2},
			func(b []byte) (any, error) { return ParsePDUSessionModificationComplete(b) },
		},
		{
			"SNSSAI",
			SNSSAI{SST: 1, SD: &sd},
			func(b []byte) (any, error) { return ParseSNSSAI(b) },
		},
		{
			"SessionAMBR",
			SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 100, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 50},
			func(b []byte) (any, error) { return ParseSessionAMBR(b) },
		},
		{
			"GPRSTimer3",
			nas.GPRSTimer3{Unit: nas.GPRSTimer3Unit1Hour, Value: 3},
			func(b []byte) (any, error) { return nas.ParseGPRSTimer3(b) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := tc.msg.MarshalBinary()
			if err != nil {
				t.Fatalf("MarshalBinary: %v", err)
			}

			got, err := tc.parse(raw)
			if err != nil {
				t.Fatalf("Parse(% x): %v", raw, err)
			}

			if !reflect.DeepEqual(got, tc.msg) {
				t.Fatalf("value identity broken\n got %+v\nwant %+v\nvia % x", got, tc.msg, raw)
			}
		})
	}
}
