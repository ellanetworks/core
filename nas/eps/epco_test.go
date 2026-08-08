// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// A UE that supports the extended element sends it instead of the classic one
// (TS 24.301 §8.3.20.4), so both ESM messages that can carry the PDU session
// identity have to round-trip it.
func TestExtendedPCORoundTrip(t *testing.T) {
	epco := nas.ProtocolConfigurationOptions{
		ConfigProtocol: nas.PCOConfigProtocolPPP,
		Direction:      nas.PCOMSToNetwork,
		Containers:     []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{7}}},
	}

	t.Run("PDN CONNECTIVITY REQUEST", func(t *testing.T) {
		wire, err := (&PDNConnectivityRequest{
			PTI:                                  1,
			RequestType:                          RequestTypeHandover,
			PDNType:                              PDNTypeIPv4,
			ExtendedProtocolConfigurationOptions: &epco,
		}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		got, err := ParsePDNConnectivityRequest(wire)
		if err != nil {
			t.Fatal(err)
		}

		assertIdentity(t, got.ExtendedProtocolConfigurationOptions, got.ProtocolConfigurationOptions)
	})

	t.Run("ESM INFORMATION RESPONSE", func(t *testing.T) {
		wire, err := (&ESMInformationResponse{
			PTI:                                  1,
			ExtendedProtocolConfigurationOptions: &epco,
		}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		got, err := ParseESMInformationResponse(wire)
		if err != nil {
			t.Fatal(err)
		}

		assertIdentity(t, got.ExtendedProtocolConfigurationOptions, got.ProtocolConfigurationOptions)
	})
}

func assertIdentity(t *testing.T, epco, pco *nas.ProtocolConfigurationOptions) {
	t.Helper()

	if pco != nil {
		t.Error("a classic PCO was decoded from a message that carried only the extended one")
	}

	if epco == nil {
		t.Fatal("the extended PCO did not survive the round trip")
	}

	if id, ok := epco.PDUSessionID(); !ok || id != 7 {
		t.Errorf("PDU session identity = %d, %v; want 7, true", id, ok)
	}
}
