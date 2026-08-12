// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.008 §10.5.6.3 container 0022H is MS to network only: the 5GSM cause the
// UE returns when it discarded the mapped 5GS QoS parameters (TS 24.501
// §6.1.4.1). It rides in the PCO of the ESM accept.
func TestModifyEPSBearerContextAcceptCarriesTheFiveGSMCause(t *testing.T) {
	pco := nas.ProtocolConfigurationOptions{
		Direction:      nas.PCOMSToNetwork,
		ConfigProtocol: nas.PCOConfigProtocolPPP,
		Containers:     []nas.PCOContainer{{ID: nas.PCOContainerFiveGSMCause, Content: []byte{83}}},
	}

	value, err := pco.MarshalBinary()
	if err != nil {
		t.Fatalf("encode the PCO: %v", err)
	}

	in := &eps.ModifyEPSBearerContextAccept{
		EPSBearerIdentity:            eps.EPSBearerIdentity(5),
		PTI:                          1,
		ProtocolConfigurationOptions: &pco,
	}

	wire, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Contains(wire, value) {
		t.Fatalf("encoded % x, want it to carry the PCO % x", wire, value)
	}

	out, err := eps.ParseModifyEPSBearerContextAccept(wire)
	if err != nil {
		t.Fatalf("ParseModifyEPSBearerContextAccept: %v", err)
	}

	if out.ProtocolConfigurationOptions == nil {
		t.Fatal("the accept's protocol configuration options were dropped")
	}

	cause, ok := out.ProtocolConfigurationOptions.FiveGSMCause()
	if !ok || cause != 83 {
		t.Errorf("5GSM cause = (%d, %v), want (83, true)", cause, ok)
	}

	again, err := out.MarshalBinary()
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}

	if !bytes.Equal(again, wire) {
		t.Errorf("re-encoded % x, want % x", again, wire)
	}
}

// The container is Reserved network to MS, so it must not be read downlink.
func TestFiveGSMCauseIsMSToNetworkOnly(t *testing.T) {
	pco := nas.ProtocolConfigurationOptions{
		Direction:      nas.PCONetworkToMS,
		ConfigProtocol: nas.PCOConfigProtocolPPP,
		Containers:     []nas.PCOContainer{{ID: nas.PCOContainerFiveGSMCause, Content: []byte{83}}},
	}

	if _, ok := pco.FiveGSMCause(); ok {
		t.Error("a network-to-MS container was read as a 5GSM cause")
	}
}
