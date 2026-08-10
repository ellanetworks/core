// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"net/netip"
	"testing"
)

func mustPCO(t *testing.T, p ProtocolConfigurationOptions) []byte {
	t.Helper()

	raw, err := p.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	return raw
}

func TestProtocolConfigurationOptionsEncoding(t *testing.T) {
	v4 := []byte{8, 8, 8, 8}
	v6 := []byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	pco := mustPCO(t, NewProtocolConfigurationOptions([][]byte{v4, v6}, 1400))

	// Config-protocol octet, the IPv4 (0x000D) and IPv6 (0x0003) DNS containers,
	// then the IPv4 Link MTU (0x0010, 2 octets; 1400 = 0x0578).
	want := []byte{0x80, 0x00, 0x0D, 0x04, 8, 8, 8, 8, 0x00, 0x03, 0x10}
	want = append(want, v6...)
	want = append(want, 0x00, 0x10, 0x02, 0x05, 0x78)

	if !bytes.Equal(pco, want) {
		t.Fatalf("PCO = %x, want %x", pco, want)
	}

	mtuOnly := mustPCO(t, NewProtocolConfigurationOptions(nil, 1500))
	if !bytes.Equal(mtuOnly, []byte{0x80, 0x00, 0x10, 0x02, 0x05, 0xDC}) {
		t.Fatalf("MTU-only PCO = %x", mtuOnly)
	}

	if !NewProtocolConfigurationOptions(nil, 0).Empty() {
		t.Fatal("no DNS and no MTU should convey nothing")
	}
}

func TestParseProtocolConfigurationOptions(t *testing.T) {
	v4 := []byte{1, 1, 1, 1}
	v6 := []byte{0x20, 0x01, 0x48, 0x60, 0x48, 0x60, 0, 0, 0, 0, 0, 0, 0, 0, 0x88, 0x88}

	pco := mustPCO(t, NewProtocolConfigurationOptions([][]byte{v4, v6}, 1400))

	got, err := ParseProtocolConfigurationOptions(pco, PCONetworkToMS)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	dns := got.DNSServers()
	if len(dns) != 2 || dns[0] != netip.AddrFrom4([4]byte(v4)) || dns[1] != netip.AddrFrom16([16]byte(v6)) {
		t.Fatalf("DNS servers = %v, want [%x %x]", dns, v4, v6)
	}

	mtu, ok := got.IPv4LinkMTU()
	if !ok || mtu != 1400 {
		t.Fatalf("MTU = %d (present %v), want 1400", mtu, ok)
	}
}

// TestProtocolConfigurationOptionsPreservesUnknown confirms a container the
// accessors do not interpret survives decode and re-encodes byte-for-byte.
func TestProtocolConfigurationOptionsPreservesUnknown(t *testing.T) {
	raw := []byte{
		0x80,
		0x00, 0x0D, 0x04, 8, 8, 8, 8, // IPv4 DNS
		0xC0, 0x23, 0x03, 0xAA, 0xBB, 0xCC, // a container this library does not model
		0x00, 0x10, 0x02, 0x05, 0x78, // IPv4 Link MTU
	}

	got, err := ParseProtocolConfigurationOptions(raw, PCONetworkToMS)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(got.Containers) != 3 {
		t.Fatalf("containers = %d, want 3", len(got.Containers))
	}

	again, err := got.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Equal(again, raw) {
		t.Fatalf("re-encode = %#x, want %#x", again, raw)
	}
}

// TestProtocolConfigurationOptionsOverlongContainer confirms a container whose
// content cannot be length-prefixed is reported rather than silently truncated.
func TestProtocolConfigurationOptionsOverlongContainer(t *testing.T) {
	p := ProtocolConfigurationOptions{
		ConfigProtocol: PCOConfigProtocolPPP,
		Containers:     []PCOContainer{{ID: PCOContainerIPv4LinkMTU, Content: make([]byte, 256)}},
	}

	if _, err := p.MarshalBinary(); err == nil {
		t.Fatal("expected an over-long container to be rejected")
	}
}

func TestRequestedProtocolConfigurationOptions(t *testing.T) {
	ids := []uint16{
		PCOContainerIPAddressAllocationViaNAS,
		PCOContainerDNSServerIPv4Address,
		PCOContainerDNSServerIPv6Address,
	}

	pco := mustPCO(t, NewRequestedProtocolConfigurationOptions(ids...))

	parsed, err := ParseProtocolConfigurationOptions(pco, PCONetworkToMS)
	if err != nil {
		t.Fatalf("ParseProtocolConfigurationOptions error: %v", err)
	}

	got := parsed.ContainerIDs()
	if len(got) != len(ids) {
		t.Fatalf("container count = %d, want %d", len(got), len(ids))
	}

	for i := range ids {
		if got[i] != ids[i] {
			t.Errorf("container %d = %#x, want %#x", i, got[i], ids[i])
		}
	}
}

// TestPCOTwoOctetLengthContainers pins the framing exception of TS 24.008
// figure 10.5.136: a handful of container identifiers carry a two-octet length,
// and the set is wider downlink than uplink. Framing one of them with a
// one-octet length desynchronizes every container after it.
func TestPCOTwoOctetLengthContainers(t *testing.T) {
	// 0x0023 (QoS rules) is two-octet downlink and one-octet uplink; 0x000D
	// (DNS server) is one-octet in both.
	downlink := []byte{
		PCOConfigProtocolPPP,
		0x00, 0x23, 0x00, 0x02, 0xaa, 0xbb, // two-octet length
		0x00, 0x0d, 0x04, 8, 8, 8, 8, // one-octet length
	}

	got, err := ParseProtocolConfigurationOptions(downlink, PCONetworkToMS)
	if err != nil {
		t.Fatalf("downlink: %v", err)
	}

	if len(got.Containers) != 2 || got.Containers[1].ID != PCOContainerDNSServerIPv4Address {
		t.Fatalf("downlink containers = %+v", got.Containers)
	}

	raw, err := got.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(raw, downlink) {
		t.Fatalf("downlink re-encode = % x, want % x", raw, downlink)
	}

	// The same identifier uplink takes a one-octet length.
	uplink := []byte{PCOConfigProtocolPPP, 0x00, 0x23, 0x02, 0xaa, 0xbb}

	up, err := ParseProtocolConfigurationOptions(uplink, PCOMSToNetwork)
	if err != nil {
		t.Fatalf("uplink: %v", err)
	}

	if len(up.Containers) != 1 || !bytes.Equal(up.Containers[0].Content, []byte{0xaa, 0xbb}) {
		t.Fatalf("uplink containers = %+v", up.Containers)
	}
}

func TestPCOPDUSessionID(t *testing.T) {
	container := func(content ...byte) ProtocolConfigurationOptions {
		return ProtocolConfigurationOptions{
			ConfigProtocol: PCOConfigProtocolPPP,
			Direction:      PCOMSToNetwork,
			Containers:     []PCOContainer{{ID: PCOContainerPDUSessionID, Content: content}},
		}
	}

	if id, ok := container(5).PDUSessionID(); !ok || id != 5 {
		t.Errorf("PDUSessionID() = %d, %v; want 5, true", id, ok)
	}

	downlink := container(5)
	downlink.Direction = PCONetworkToMS

	if _, ok := downlink.PDUSessionID(); ok {
		t.Error("PDUSessionID() read an identity out of a network-to-MS element")
	}

	for _, content := range [][]byte{{0}, {16}, {64}, {}, {5, 5}} {
		if _, ok := container(content...).PDUSessionID(); ok {
			t.Errorf("PDUSessionID() accepted container content % x", content)
		}
	}
}

func TestNewSNSSAIContainer(t *testing.T) {
	c, err := NewSNSSAIContainer([]byte{0x01, 0x00, 0x00, 0x7b}, PLMN{MCC: "001", MNC: "01"})
	if err != nil {
		t.Fatal(err)
	}

	if c.ID != PCOContainerSNSSAI {
		t.Errorf("container id = %#04x, want %#04x", c.ID, PCOContainerSNSSAI)
	}

	want := []byte{0x01, 0x00, 0x00, 0x7b, 0x00, 0xf1, 0x10}
	if !bytes.Equal(c.Content, want) {
		t.Errorf("content = % x, want % x", c.Content, want)
	}

	if _, err := NewSNSSAIContainer([]byte{0x01, 0x02, 0x03}, PLMN{MCC: "001", MNC: "01"}); err == nil {
		t.Error("a 3-octet S-NSSAI value part was accepted")
	}

	if _, err := NewSNSSAIContainer([]byte{0x01}, PLMN{MCC: "00", MNC: "01"}); err == nil {
		t.Error("a malformed PLMN was accepted")
	}
}
