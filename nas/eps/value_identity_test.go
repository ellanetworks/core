// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestValueIdentity asserts Parse(MarshalBinary(m)) equals m for messages built as
// struct literals rather than decoded, covering the encode direction the MME
// uses for every message it originates. It mirrors the 5GS test of the same
// name.
func TestValueIdentity(t *testing.T) {
	emmCause := EMMCauseCongestion

	cases := []struct {
		name  string
		msg   encoder
		parse func([]byte) (any, error)
	}{
		{
			"AttachReject", &AttachReject{Cause: EMMCauseIMSIUnknownInHSS},
			func(b []byte) (any, error) { return ParseAttachReject(b) },
		},
		{
			"EMMStatus", &EMMStatus{Cause: EMMCauseProtocolErrorUnspecified},
			func(b []byte) (any, error) { return ParseEMMStatus(b) },
		},
		{
			"IdentityRequest", &IdentityRequest{IdentityType: 1},
			func(b []byte) (any, error) { return ParseIdentityRequest(b) },
		},
		{
			"DetachAccept", &DetachAccept{},
			func(b []byte) (any, error) { return ParseDetachAccept(b) },
		},
		{
			"DetachRequestNetwork", &DetachRequestNetwork{TypeOfDetach: DetachTypeReattachRequired, Cause: &emmCause},
			func(b []byte) (any, error) { return ParseDetachRequestNetwork(b) },
		},
		{
			"ESMStatus", &ESMStatus{EPSBearerIdentity: 5, PTI: 1, Cause: ESMCauseInvalidEPSBearerIdentity},
			func(b []byte) (any, error) { return ParseESMStatus(b) },
		},
		{
			"ServiceRequest", &ServiceRequest{KSI: 3, SeqShort: 7, ShortMAC: [2]byte{0xab, 0xcd}},
			func(b []byte) (any, error) { return ParseServiceRequest(b) },
		},
		{
			"EPSQoS",
			EPSQoS{QCI: 9},
			func(b []byte) (any, error) { return ParseEPSQoS(b) },
		},
		{
			"APNAMBR", mustAPNAMBR(100_000, 50_000),
			func(b []byte) (any, error) { return ParseAPNAMBR(b) },
		},
		{
			"UESecurityCapability",
			UESecurityCapability{EEA: 0x80, EIA: 0x80, HasUMTS: true, UEA: 0xc0, UIA: 0x40},
			func(b []byte) (any, error) { return ParseUESecurityCapability(b) },
		},
		{
			"GPRSTimer2",
			nas.GPRSTimer2{Unit: nas.GPRSTimer2Unit1Minute, Value: 12},
			func(b []byte) (any, error) { return nas.ParseGPRSTimer2(b) },
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

// mustAPNAMBR builds an APN-AMBR from rates the element can express exactly.
func mustAPNAMBR(downlinkKbps, uplinkKbps uint64) APNAMBR {
	a, err := APNAMBRFromKbps(downlinkKbps, uplinkKbps)
	if err != nil {
		panic(err)
	}

	return a
}
