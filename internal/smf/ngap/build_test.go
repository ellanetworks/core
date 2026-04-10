// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ngap_test

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func decodeSetupRequestGTPTunnel(t *testing.T, buf []byte) (uint32, aper.BitString) {
	t.Helper()

	var transfer ngapType.PDUSessionResourceSetupRequestTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	for _, ie := range transfer.ProtocolIEs.List {
		if ie.Id.Value != ngapType.ProtocolIEIDULNGUUPTNLInformation {
			continue
		}

		tunnel := ie.Value.ULNGUUPTNLInformation.GTPTunnel
		teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)

		return teid, tunnel.TransportLayerAddress.Value
	}

	t.Fatal("ULNGUUPTNLInformation IE not found")

	return 0, aper.BitString{}
}

func TestBuildPDUSessionResourceSetupRequestTransfer(t *testing.T) {
	ambr := &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	addr := netip.MustParseAddr("10.3.0.2")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 42, addr, netip.Addr{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	teid, bs := decodeSetupRequestGTPTunnel(t, buf)

	if teid != 42 {
		t.Errorf("TEID: got %d, want 42", teid)
	}

	if bs.BitLength != 32 {
		t.Fatalf("BitLength: got %d, want 32", bs.BitLength)
	}

	var ip [4]byte
	copy(ip[:], bs.Bytes)

	if ip != [4]byte{10, 3, 0, 2} {
		t.Errorf("IP: got %v, want [10 3 0 2]", ip)
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_NilAmbr(t *testing.T) {
	_, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(nil, nil, 1, netip.MustParseAddr("1.2.3.4"), netip.Addr{})
	if err == nil {
		t.Fatal("expected error for nil ambr")
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_IPv6Only(t *testing.T) {
	ambr := &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	ipv6 := netip.MustParseAddr("2001:db8::1")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 7, netip.Addr{}, ipv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, bs := decodeSetupRequestGTPTunnel(t, buf)

	if bs.BitLength != 128 {
		t.Fatalf("BitLength: got %d, want 128", bs.BitLength)
	}

	if len(bs.Bytes) != 16 {
		t.Fatalf("Bytes length: got %d, want 16", len(bs.Bytes))
	}

	v6 := ipv6.As16()
	for i, b := range bs.Bytes {
		if b != v6[i] {
			t.Errorf("IPv6 byte[%d]: got %02x, want %02x", i, b, v6[i])
		}
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_DualStack(t *testing.T) {
	ambr := &models.Ambr{Uplink: "100 Mbps", Downlink: "200 Mbps"}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	ipv4 := netip.MustParseAddr("10.3.0.2")
	ipv6 := netip.MustParseAddr("2001:db8::1")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 99, ipv4, ipv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, bs := decodeSetupRequestGTPTunnel(t, buf)

	if bs.BitLength != 160 {
		t.Fatalf("BitLength: got %d, want 160", bs.BitLength)
	}

	if len(bs.Bytes) != 20 {
		t.Fatalf("Bytes length: got %d, want 20", len(bs.Bytes))
	}

	wantV4 := ipv4.As4()
	if [4]byte(bs.Bytes[0:4]) != wantV4 {
		t.Errorf("IPv4 part: got %v, want %v", bs.Bytes[0:4], wantV4)
	}

	wantV6 := ipv6.As16()
	for i, b := range bs.Bytes[4:20] {
		if b != wantV6[i] {
			t.Errorf("IPv6 byte[%d]: got %02x, want %02x", i, b, wantV6[i])
		}
	}
}

func TestBuildHandoverCommandTransfer(t *testing.T) {
	addr := netip.MustParseAddr("192.168.1.100")

	buf, err := ngap.BuildHandoverCommandTransfer(99, addr, netip.Addr{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var transfer ngapType.HandoverCommandTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.DLForwardingUPTNLInformation.GTPTunnel
	teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)

	if teid != 99 {
		t.Errorf("TEID: got %d, want 99", teid)
	}

	bs := tunnel.TransportLayerAddress.Value
	if bs.BitLength != 32 {
		t.Fatalf("BitLength: got %d, want 32", bs.BitLength)
	}

	var ip [4]byte
	copy(ip[:], bs.Bytes)

	if ip != [4]byte{192, 168, 1, 100} {
		t.Errorf("IP: got %v, want [192 168 1 100]", ip)
	}
}

func TestBuildHandoverCommandTransfer_DualStack(t *testing.T) {
	ipv4 := netip.MustParseAddr("10.1.2.3")
	ipv6 := netip.MustParseAddr("2001:db8::2")

	buf, err := ngap.BuildHandoverCommandTransfer(55, ipv4, ipv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var transfer ngapType.HandoverCommandTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bs := transfer.DLForwardingUPTNLInformation.GTPTunnel.TransportLayerAddress.Value

	if bs.BitLength != 160 {
		t.Fatalf("BitLength: got %d, want 160", bs.BitLength)
	}
}

func TestBuildPathSwitchRequestAcknowledgeTransfer(t *testing.T) {
	addr := netip.MustParseAddr("172.16.0.1")

	buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(7, addr, netip.Addr{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var transfer ngapType.PathSwitchRequestAcknowledgeTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.ULNGUUPTNLInformation.GTPTunnel
	teid := binary.BigEndian.Uint32(tunnel.GTPTEID.Value)

	if teid != 7 {
		t.Errorf("TEID: got %d, want 7", teid)
	}

	bs := tunnel.TransportLayerAddress.Value
	if bs.BitLength != 32 {
		t.Fatalf("BitLength: got %d, want 32", bs.BitLength)
	}

	var ip [4]byte
	copy(ip[:], bs.Bytes)

	if ip != [4]byte{172, 16, 0, 1} {
		t.Errorf("IP: got %v, want [172 16 0 1]", ip)
	}
}

func TestBuildPathSwitchRequestAcknowledgeTransfer_IPv6Only(t *testing.T) {
	ipv6 := netip.MustParseAddr("2001:db8::3")

	buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(3, netip.Addr{}, ipv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var transfer ngapType.PathSwitchRequestAcknowledgeTransfer
	if err := aper.UnmarshalWithParams(buf, &transfer, "valueExt"); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bs := transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value

	if bs.BitLength != 128 {
		t.Fatalf("BitLength: got %d, want 128", bs.BitLength)
	}
}
