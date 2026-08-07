// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	libngap "github.com/ellanetworks/core/ngap"
)

func decodeSetupRequestGTPTunnel(t *testing.T, buf []byte) (uint32, libngap.TransportLayerAddress) {
	t.Helper()

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	tunnel := transfer.ULNGUUPTNLInformation.GTPTunnel

	return uint32(tunnel.GTPTEID), tunnel.TransportLayerAddress
}

func TestBuildPDUSessionResourceSetupRequestTransfer(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	addr := netip.MustParseAddr("10.3.0.2")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 42, addr, netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	teid, bs := decodeSetupRequestGTPTunnel(t, buf)

	if teid != 42 {
		t.Errorf("TEID: got %d, want 42", teid)
	}

	if len(bs)*8 != 32 {
		t.Fatalf("BitLength: got %d, want 32", len(bs)*8)
	}

	var ip [4]byte
	copy(ip[:], []byte(bs))

	if ip != [4]byte{10, 3, 0, 2} {
		t.Errorf("IP: got %v, want [10 3 0 2]", ip)
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_NilAmbr(t *testing.T) {
	_, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(nil, nil, 1, netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err == nil {
		t.Fatal("expected error for nil ambr")
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_IPv6Only(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	ipv6 := netip.MustParseAddr("2001:db8::1")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 7, netip.Addr{}, ipv6, libngap.PDUSessionTypeIPv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, bs := decodeSetupRequestGTPTunnel(t, buf)

	if len(bs)*8 != 128 {
		t.Fatalf("BitLength: got %d, want 128", len(bs)*8)
	}

	if len([]byte(bs)) != 16 {
		t.Fatalf("Bytes length: got %d, want 16", len([]byte(bs)))
	}

	v6 := ipv6.As16()
	for i, b := range []byte(bs) {
		if b != v6[i] {
			t.Errorf("IPv6 byte[%d]: got %02x, want %02x", i, b, v6[i])
		}
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_DualStack(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	ipv4 := netip.MustParseAddr("10.3.0.2")
	ipv6 := netip.MustParseAddr("2001:db8::1")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 99, ipv4, ipv6, libngap.PDUSessionTypeIPv4v6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, bs := decodeSetupRequestGTPTunnel(t, buf)

	if len(bs)*8 != 160 {
		t.Fatalf("BitLength: got %d, want 160", len(bs)*8)
	}

	if len([]byte(bs)) != 20 {
		t.Fatalf("Bytes length: got %d, want 20", len([]byte(bs)))
	}

	wantV4 := ipv4.As4()
	if [4]byte([]byte(bs)[0:4]) != wantV4 {
		t.Errorf("IPv4 part: got %v, want %v", []byte(bs)[0:4], wantV4)
	}

	wantV6 := ipv6.As16()
	for i, b := range []byte(bs)[4:20] {
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

	transfer, err := libngap.ParseHandoverCommandTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.DLForwardingUPTNLInformation.GTPTunnel
	teid := uint32(tunnel.GTPTEID)

	if teid != 99 {
		t.Errorf("TEID: got %d, want 99", teid)
	}

	bs := tunnel.TransportLayerAddress
	if len(bs)*8 != 32 {
		t.Fatalf("BitLength: got %d, want 32", len(bs)*8)
	}

	var ip [4]byte
	copy(ip[:], []byte(bs))

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

	transfer, err := libngap.ParseHandoverCommandTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bs := transfer.DLForwardingUPTNLInformation.GTPTunnel.TransportLayerAddress

	if len(bs)*8 != 160 {
		t.Fatalf("BitLength: got %d, want 160", len(bs)*8)
	}
}

func TestBuildPathSwitchRequestAcknowledgeTransfer(t *testing.T) {
	addr := netip.MustParseAddr("172.16.0.1")

	buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(7, addr, netip.Addr{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transfer, err := libngap.ParsePathSwitchRequestAcknowledgeTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.ULNGUUPTNLInformation.GTPTunnel
	teid := uint32(tunnel.GTPTEID)

	if teid != 7 {
		t.Errorf("TEID: got %d, want 7", teid)
	}

	bs := tunnel.TransportLayerAddress
	if len(bs)*8 != 32 {
		t.Fatalf("BitLength: got %d, want 32", len(bs)*8)
	}

	var ip [4]byte
	copy(ip[:], []byte(bs))

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

	transfer, err := libngap.ParsePathSwitchRequestAcknowledgeTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bs := transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress

	if len(bs)*8 != 128 {
		t.Fatalf("BitLength: got %d, want 128", len(bs)*8)
	}
}

// TS 38.413 bounds priorityLevelARP at 1..15. The library encoder refuses a 0 on
// its own, but only as "value out of range"; the builder checks first so an
// operator with a bad policy is told which field is wrong.
func TestBuildPDUSessionResourceSetupRequestTransferRejectsARPZero(t *testing.T) {
	ambr := &models.Ambr{Downlink: models.MustParseBitRate("1 Gbps"), Uplink: models.MustParseBitRate("1 Gbps")}
	qos := &models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 0}}

	_, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 1,
		netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err == nil {
		t.Fatal("ARP priority 0 encoded, want an error")
	}

	if !strings.Contains(err.Error(), "ARP priority level") {
		t.Errorf("err = %v, want it to name the offending field", err)
	}
}
