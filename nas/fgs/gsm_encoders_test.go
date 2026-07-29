// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"
	"net"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestMarshalEstablishmentAcceptGolden(t *testing.T) {
	sd := [3]byte{0x01, 0x02, 0x03}

	pco := nas.NewProtocolConfigurationOptions([][]byte{net.IPv4(8, 8, 8, 8).To4()}, 1400)
	pcoPtr := &pco

	m := &PDUSessionEstablishmentAccept{
		PDUSessionID:        5,
		PTI:                 1,
		PDUSessionType:      PDUSessionTypeIPv4,
		SSCMode:             1,
		QoSRules:            QoSRules{DefaultQoSRule(1, 1)},
		SessionAMBR:         SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 2000, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 1000},
		PDUAddress:          &PDUAddress{SessionType: PDUSessionTypeIPv4, IPv4: [4]byte{10, 0, 0, 1}},
		SNSSAI:              &SNSSAI{SST: 1, SD: &sd},
		QoSFlowDescriptions: QoSFlowDescriptions{FiveQIQoSFlow(1, 9, QoSFlowOpCreate)},
		ExtendedPCO:         pcoPtr,
		DNN:                 ptr(DNN("internet")),
	}

	want := "2e0501c211000901000631310101ff01060607d00603e82905010a0000012204010102037900060120410101097b000d80000d04080808080010020578250908696e7465726e6574"

	assertMarshal(t, m.MarshalBinary, want)
}

func TestMarshalModificationCommandGolden(t *testing.T) {
	m := &PDUSessionModificationCommand{
		PDUSessionID:        7,
		SessionAMBR:         &SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 600, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 500},
		QoSFlowDescriptions: QoSFlowDescriptions{FiveQIQoSFlow(2, 8, QoSFlowOpModify)},
	}

	assertMarshal(t, m.MarshalBinary, "2e0700cb2a060602580601f4790006026041010108")
}

func TestQoSAndPCOEncoders(t *testing.T) {
	if got := hex.EncodeToString(mustBytes(QoSRules{DefaultQoSRule(1, 1)}.MarshalBinary())); got != "01000631310101ff01" {
		t.Errorf("QoS rules = %s", got)
	}

	if got := hex.EncodeToString(mustBytes(QoSFlowDescriptions{FiveQIQoSFlow(1, 9, QoSFlowOpCreate)}.MarshalBinary())); got != "012041010109" {
		t.Errorf("QoS flow (create) = %s", got)
	}

	if got := hex.EncodeToString(mustBytes(QoSFlowDescriptions{FiveQIQoSFlow(2, 8, QoSFlowOpModify)}.MarshalBinary())); got != "026041010108" {
		t.Errorf("QoS flow (modify) = %s", got)
	}

	pco := nas.NewProtocolConfigurationOptions([][]byte{net.IPv4(8, 8, 8, 8).To4()}, 1400)
	if got := hex.EncodeToString(mustBytes(pco.MarshalBinary())); got != "80000d04080808080010020578" {
		t.Errorf("PCO = %s", got)
	}
}

func TestEstablishmentAcceptRoundTrip(t *testing.T) {
	sd := [3]byte{0x0A, 0x0B, 0x0C}
	apsi := true

	pco := nas.NewProtocolConfigurationOptions([][]byte{net.IPv4(1, 1, 1, 1).To4()}, 0)
	pcoPtr := &pco

	orig := &PDUSessionEstablishmentAccept{
		PDUSessionID:        7,
		PTI:                 3,
		PDUSessionType:      PDUSessionTypeIPv4v6,
		SSCMode:             1,
		QoSRules:            QoSRules{DefaultQoSRule(1, 5)},
		SessionAMBR:         SessionAMBR{DownlinkUnit: SessionAMBRUnit1Gbps, Downlink: 2, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 500},
		Cause:               ptr(GSMCausePDUSessionTypeIPv4OnlyAllowed),
		PDUAddress:          &PDUAddress{SessionType: PDUSessionTypeIPv4v6, IPv4: [4]byte{10, 0, 0, 9}, IPv6IID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
		SNSSAI:              &SNSSAI{SST: 2, SD: &sd},
		AlwaysOn:            &apsi,
		QoSFlowDescriptions: QoSFlowDescriptions{FiveQIQoSFlow(5, 9, QoSFlowOpCreate)},
		ExtendedPCO:         pcoPtr,
		DNN:                 ptr(DNN("ella.internet")),
	}

	b, err := orig.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	got, err := ParsePDUSessionEstablishmentAccept(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.SSCMode != 1 || got.PDUSessionType != PDUSessionTypeIPv4v6 || (got.Cause == nil || *got.Cause != *orig.Cause) ||
		got.DNN == nil || *got.DNN != *orig.DNN || got.AlwaysOn == nil || !*got.AlwaysOn ||
		!reflect.DeepEqual(got.SNSSAI, orig.SNSSAI) ||
		!reflect.DeepEqual(got.PDUAddress, orig.PDUAddress) ||
		got.SessionAMBR != orig.SessionAMBR || !reflect.DeepEqual(got.QoSRules, orig.QoSRules) ||
		!reflect.DeepEqual(got.QoSFlowDescriptions, orig.QoSFlowDescriptions) || !reflect.DeepEqual(got.ExtendedPCO, orig.ExtendedPCO) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, orig)
	}
}

func TestModificationCommandRoundTrip(t *testing.T) {
	orig := &PDUSessionModificationCommand{
		PDUSessionID:        4,
		PTI:                 0,
		SessionAMBR:         &SessionAMBR{DownlinkUnit: SessionAMBRUnit1Mbps, Downlink: 100, UplinkUnit: SessionAMBRUnit1Mbps, Uplink: 50},
		QoSFlowDescriptions: QoSFlowDescriptions{FiveQIQoSFlow(3, 7, QoSFlowOpModify)},
	}

	b, err := orig.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	got, err := ParsePDUSessionModificationCommand(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !reflect.DeepEqual(got.SessionAMBR, orig.SessionAMBR) ||
		!reflect.DeepEqual(got.QoSFlowDescriptions, orig.QoSFlowDescriptions) || got.ExtendedPCO != nil {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", got, orig)
	}
}

func assertMarshal(t *testing.T, fn func() ([]byte, error), want string) {
	t.Helper()

	b, err := fn()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if got := hex.EncodeToString(b); got != want {
		t.Fatalf("MarshalBinary =\n %s\nwant\n %s", got, want)
	}
}
