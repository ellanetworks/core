// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"
)

// TestTwoOctetSupportIndicatorSelectsTheContainer pins the choice TS 24.301
// §6.5.1.2 gives the network: the mapped QoS rules and flow descriptions go
// under their two-octet-length identifiers only when the MS said it can receive
// them, and under the one-octet ones otherwise.
func TestTwoOctetSupportIndicatorSelectsTheContainer(t *testing.T) {
	value := []byte{0x01, 0x02, 0x03}

	tests := []struct {
		name      string
		build     func([]byte, bool) (PCOContainer, error)
		oneOctet  uint16
		twoOctets uint16
	}{
		{"QoS rules", NewQoSRulesContainer, PCOContainerQoSRules, PCOContainerQoSRulesTwoOctet},
		{
			"QoS flow descriptions", NewQoSFlowDescriptionsContainer,
			PCOContainerQoSFlowDescriptions, PCOContainerQoSFlowDescriptionsTwoOctet,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plain, err := tc.build(value, false)
			if err != nil {
				t.Fatal(err)
			}

			if plain.ID != tc.oneOctet {
				t.Errorf("without the indicator: container %#04x, want %#04x", plain.ID, tc.oneOctet)
			}

			wide, err := tc.build(value, true)
			if err != nil {
				t.Fatal(err)
			}

			if wide.ID != tc.twoOctets {
				t.Errorf("with the indicator: container %#04x, want %#04x", wide.ID, tc.twoOctets)
			}

			if !bytes.Equal(wide.Content, value) {
				t.Errorf("content = % x, want % x", wide.Content, value)
			}

			// A value that outgrows a one-octet length has nowhere to go unless
			// the MS opted into the wide form; silently truncating it is what the
			// error prevents.
			long := make([]byte, maxOneOctetContainerLen+1)

			if _, err := tc.build(long, false); err == nil {
				t.Error("a 256-octet value without the indicator: want an error, got none")
			}

			if _, err := tc.build(long, true); err != nil {
				t.Errorf("a 256-octet value with the indicator: %v", err)
			}

			if _, err := tc.build(nil, true); err == nil {
				t.Error("an empty value: want an error, got none")
			}
		})
	}
}

// TestMappedQoSContainersFrameTheirLengths checks the encoder frames each
// identifier as TS 24.008 figure 10.5.136 requires: 0023H and 0024H carry a
// two-octet length network to MS, and 001CH, 001DH and 001FH a one-octet one.
func TestMappedQoSContainersFrameTheirLengths(t *testing.T) {
	ambr, err := NewSessionAMBRContainer([]byte{0x06, 0x01, 0x00, 0x06, 0x01, 0x00})
	if err != nil {
		t.Fatal(err)
	}

	rules, err := NewQoSRulesContainer([]byte{0xaa, 0xbb}, true)
	if err != nil {
		t.Fatal(err)
	}

	flows, err := NewQoSFlowDescriptionsContainer([]byte{0xcc}, false)
	if err != nil {
		t.Fatal(err)
	}

	p := ProtocolConfigurationOptions{
		ConfigProtocol: PCOConfigProtocolPPP,
		Direction:      PCONetworkToMS,
		Containers:     []PCOContainer{ambr, rules, flows},
	}

	raw, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{
		PCOConfigProtocolPPP,
		0x00, 0x1d, 0x06, 0x06, 0x01, 0x00, 0x06, 0x01, 0x00, // Session-AMBR, one-octet length
		0x00, 0x23, 0x00, 0x02, 0xaa, 0xbb, // QoS rules, two-octet length
		0x00, 0x1f, 0x01, 0xcc, // QoS flow descriptions, one-octet length
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("options = % x, want % x", raw, want)
	}

	back, err := ParseExtendedProtocolConfigurationOptions(raw, PCONetworkToMS)
	if err != nil {
		t.Fatal(err)
	}

	if len(back.Containers) != 3 {
		t.Fatalf("decoded %d containers, want 3", len(back.Containers))
	}

	for i, c := range back.Containers {
		if c.ID != p.Containers[i].ID || !bytes.Equal(c.Content, p.Containers[i].Content) {
			t.Errorf("container %d = %#04x/% x, want %#04x/% x", i, c.ID, c.Content, p.Containers[i].ID, p.Containers[i].Content)
		}
	}
}

// TestSupportIndicatorsAreUplinkOnly checks the indicators are read only in the
// direction they exist in: network to MS the same identifiers name the QoS rules
// and flow descriptions themselves, so reading them as indicators there would
// have the network answer its own message.
func TestSupportIndicatorsAreUplinkOnly(t *testing.T) {
	uplink := NewRequestedProtocolConfigurationOptions(
		PCOContainerQoSRulesTwoOctet, PCOContainerQoSFlowDescriptionsTwoOctet)

	if !uplink.SupportsTwoOctetQoSRules() || !uplink.SupportsTwoOctetQoSFlowDescriptions() {
		t.Error("MS-to-network indicators were not read")
	}

	downlink := uplink
	downlink.Direction = PCONetworkToMS

	if downlink.SupportsTwoOctetQoSRules() || downlink.SupportsTwoOctetQoSFlowDescriptions() {
		t.Error("network-to-MS containers were read as support indicators")
	}

	none := NewRequestedProtocolConfigurationOptions(PCOContainerPDUSessionID)
	if none.SupportsTwoOctetQoSRules() || none.SupportsTwoOctetQoSFlowDescriptions() {
		t.Error("indicators reported for options that carry none")
	}
}
