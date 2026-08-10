// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
)

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

// TS 24.301 §8.3.18.13
func TestModifyEPSBearerContextRequestExtendedPCO(t *testing.T) {
	pco := nas.ProtocolConfigurationOptions{
		Direction:      nas.PCONetworkToMS,
		ConfigProtocol: nas.PCOConfigProtocolPPP,
		Containers:     []nas.PCOContainer{{ID: nas.PCOContainerSNSSAI, Content: []byte{0x01, 0x00, 0x00, 0x01}}},
	}

	in := &ModifyEPSBearerContextRequest{
		EPSBearerIdentity:                    5,
		PTI:                                  1,
		ExtendedProtocolConfigurationOptions: &pco,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if !bytes.Contains(wire, []byte{ieiExtendedProtocolConfigurationOptions}) {
		t.Fatalf("encoded % x carries no extended protocol configuration options IEI", wire)
	}

	if bytes.Contains(wire, []byte{ieiProtocolConfigurationOptions}) {
		t.Error("both protocol configuration options elements were encoded; §8.3.18.9/.13 make them exclusive")
	}

	out, err := ParseModifyEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("the extended element did not survive a round trip")
	}

	if out.ProtocolConfigurationOptions != nil {
		t.Error("the extended element decoded into the classic field")
	}

	got := out.ExtendedProtocolConfigurationOptions.Containers
	if len(got) != 1 || got[0].ID != nas.PCOContainerSNSSAI {
		t.Errorf("containers = %+v, want the S-NSSAI container", got)
	}
}
