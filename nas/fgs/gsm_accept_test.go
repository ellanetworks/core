// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"encoding/hex"
	"net"
	"testing"
)

// Golden vectors verified byte-identical to the free5gc encoder.

func TestMarshalEstablishmentAcceptGolden(t *testing.T) {
	sd := [3]byte{0x01, 0x02, 0x03}

	pco := BuildProtocolConfigurationOptions([][]byte{net.IPv4(8, 8, 8, 8).To4()}, 1400)

	m := &PDUSessionEstablishmentAccept{
		PDUSessionID:        5,
		PTI:                 1,
		PDUSessionType:      PDUSessionTypeIPv4,
		SSCMode:             1,
		QoSRules:            MarshalQoSRules([]QoSRule{DefaultQoSRule(1, 1)}),
		SessionAMBR:         SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 2000, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 1000},
		PDUAddress:          &PDUAddress{SessionType: PDUSessionTypeIPv4, IPv4: [4]byte{10, 0, 0, 1}},
		SNSSAI:              &SNSSAI{SST: 1, SD: &sd},
		QoSFlowDescriptions: MarshalCreateQoSFlow(1, 9),
		ExtendedPCO:         pco,
		DNN:                 "internet",
	}

	want := "2e0501c211000901000631310101ff01060607d00603e82905010a0000012204010102037900060120410101097b000d80000d04080808080010020578250908696e7465726e6574"

	assertMarshal(t, m.Marshal, want)
}

func TestMarshalModificationCommandGolden(t *testing.T) {
	m := &PDUSessionModificationCommand{
		PDUSessionID:        7,
		SessionAMBR:         &SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 600, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 500},
		QoSFlowDescriptions: MarshalModifyQoSFlow(2, 8),
	}

	assertMarshal(t, m.Marshal, "2e0700cb2a060602580601f4790006024041010108")
}

func TestQoSAndPCOEncoders(t *testing.T) {
	if got := hex.EncodeToString(MarshalQoSRules([]QoSRule{DefaultQoSRule(1, 1)})); got != "01000631310101ff01" {
		t.Errorf("QoS rules = %s", got)
	}

	if got := hex.EncodeToString(MarshalCreateQoSFlow(1, 9)); got != "012041010109" {
		t.Errorf("QoS flow (create) = %s", got)
	}

	if got := hex.EncodeToString(MarshalModifyQoSFlow(2, 8)); got != "024041010108" {
		t.Errorf("QoS flow (modify) = %s", got)
	}

	pco := BuildProtocolConfigurationOptions([][]byte{net.IPv4(8, 8, 8, 8).To4()}, 1400)
	if got := hex.EncodeToString(pco); got != "80000d04080808080010020578" {
		t.Errorf("PCO = %s", got)
	}
}

func TestEstablishmentAcceptRoundTrip(t *testing.T) {
	sd := [3]byte{0x0A, 0x0B, 0x0C}
	apsi := uint8(1)

	pco := BuildProtocolConfigurationOptions([][]byte{net.IPv4(1, 1, 1, 1).To4()}, 0)

	orig := &PDUSessionEstablishmentAccept{
		PDUSessionID:        7,
		PTI:                 3,
		PDUSessionType:      PDUSessionTypeIPv4IPv6,
		SSCMode:             1,
		QoSRules:            MarshalQoSRules([]QoSRule{DefaultQoSRule(1, 5)}),
		SessionAMBR:         SessionAMBR{DownlinkUnit: SessionAMBRUnit1Gbps, Downlink: 2, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 500},
		Cause:               GSMCausePDUSessionTypeIPv4OnlyAllowed,
		PDUAddress:          &PDUAddress{SessionType: PDUSessionTypeIPv4IPv6, IPv4: [4]byte{10, 0, 0, 9}, IPv6IID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
		SNSSAI:              &SNSSAI{SST: 2, SD: &sd},
		AlwaysOn:            &apsi,
		QoSFlowDescriptions: MarshalCreateQoSFlow(5, 9),
		ExtendedPCO:         pco,
		DNN:                 "ella.internet",
	}

	b, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := ParsePDUSessionEstablishmentAccept(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.SSCMode != 1 || got.PDUSessionType != PDUSessionTypeIPv4IPv6 || got.Cause != orig.Cause ||
		got.DNN != orig.DNN || got.AlwaysOn == nil || *got.AlwaysOn != 1 ||
		got.SNSSAI == nil || got.SNSSAI.SST != 2 || got.SNSSAI.SD == nil || *got.SNSSAI.SD != sd ||
		got.PDUAddress == nil || got.PDUAddress.IPv4 != orig.PDUAddress.IPv4 || got.PDUAddress.IPv6IID != orig.PDUAddress.IPv6IID ||
		got.SessionAMBR != orig.SessionAMBR || !bytes.Equal(got.QoSRules, orig.QoSRules) ||
		!bytes.Equal(got.QoSFlowDescriptions, orig.QoSFlowDescriptions) || !bytes.Equal(got.ExtendedPCO, orig.ExtendedPCO) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, orig)
	}
}

func TestModificationCommandRoundTrip(t *testing.T) {
	orig := &PDUSessionModificationCommand{
		PDUSessionID:        4,
		PTI:                 0,
		SessionAMBR:         &SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 100, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 50},
		QoSFlowDescriptions: MarshalModifyQoSFlow(3, 7),
	}

	b, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, err := ParsePDUSessionModificationCommand(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.SessionAMBR == nil || *got.SessionAMBR != *orig.SessionAMBR ||
		!bytes.Equal(got.QoSFlowDescriptions, orig.QoSFlowDescriptions) || got.ExtendedPCO != nil {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, orig)
	}
}

func assertMarshal(t *testing.T, fn func() ([]byte, error), want string) {
	t.Helper()

	b, err := fn()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if got := hex.EncodeToString(b); got != want {
		t.Fatalf("Marshal =\n %s\nwant\n %s", got, want)
	}
}
