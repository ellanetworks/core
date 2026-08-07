// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var (
	localIPv4 = netip.MustParseAddr("10.3.0.1")
	localIPv6 = netip.MustParseAddr("2001:db8::1")
)

// buildFAR is a helper that constructs a models.FAR with ForwardingParameters.
func buildFAR(ohcDesc uint16, teid uint32, ipv4Addr, ipv6Addr string) models.FAR {
	ohc := &models.OuterHeaderCreation{
		Description: ohcDesc,
		TEID:        teid,
	}

	if ipv4Addr != "" {
		ohc.IPv4Address = net.ParseIP(ipv4Addr).To4()
	}

	if ipv6Addr != "" {
		ohc.IPv6Address = net.ParseIP(ipv6Addr)
	}

	return models.FAR{
		FARID: 1,
		ApplyAction: models.ApplyAction{
			Forw: true,
		},
		ForwardingParameters: &models.ForwardingParameters{
			OuterHeaderCreation: ohc,
		},
	}
}

// TestFarInfoFromModel_IPv4 verifies that an IPv4 OHC FAR is encoded correctly.
func TestFarInfoFromModel_IPv4(t *testing.T) {
	far := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 100, "192.168.0.10", "")
	info := farInfoFromModel(far, localIPv4, localIPv6)

	// OHC byte: Description >> 8 = 256 >> 8 = 1
	if info.OuterHeaderCreation != 1 {
		t.Errorf("OuterHeaderCreation: got %d, want 1", info.OuterHeaderCreation)
	}

	if info.TeID != 100 {
		t.Errorf("TeID: got %d, want 100", info.TeID)
	}

	wantLocal := ebpf.IPToIn6Addr(localIPv4)
	if info.LocalIP != wantLocal {
		t.Errorf("LocalIP: got %v, want %v", info.LocalIP, wantLocal)
	}

	wantRemote := ebpf.IPToIn6Addr(netip.MustParseAddr("192.168.0.10"))
	if info.RemoteIP != wantRemote {
		t.Errorf("RemoteIP: got %v, want %v", info.RemoteIP, wantRemote)
	}
}

// TestFarInfoFromModel_IPv6 verifies that an IPv6 OHC FAR is encoded correctly.
func TestFarInfoFromModel_IPv6(t *testing.T) {
	far := buildFAR(models.OuterHeaderCreationGtpUUdpIpv6, 200, "", "2001:db8::cafe")
	info := farInfoFromModel(far, localIPv4, localIPv6)

	// OHC byte: Description >> 8 = 512 >> 8 = 2
	if info.OuterHeaderCreation != 2 {
		t.Errorf("OuterHeaderCreation: got %d, want 2", info.OuterHeaderCreation)
	}

	if info.TeID != 200 {
		t.Errorf("TeID: got %d, want 200", info.TeID)
	}

	wantLocal := ebpf.IPToIn6Addr(localIPv6)
	if info.LocalIP != wantLocal {
		t.Errorf("LocalIP: got %v, want %v", info.LocalIP, wantLocal)
	}

	wantRemote := ebpf.IPToIn6Addr(netip.MustParseAddr("2001:db8::cafe"))
	if info.RemoteIP != wantRemote {
		t.Errorf("RemoteIP: got %v, want %v", info.RemoteIP, wantRemote)
	}
}

// TestFarInfoFromModel_NoAddress verifies that a FAR whose OHC has no address
// yet (DL FAR before the gNB responds) defaults to the IPv4 local address and
// leaves RemoteIP as zero.
func TestFarInfoFromModel_NoAddress(t *testing.T) {
	far := models.FAR{
		FARID:       1,
		ApplyAction: models.ApplyAction{Forw: true},
		ForwardingParameters: &models.ForwardingParameters{
			OuterHeaderCreation: &models.OuterHeaderCreation{
				Description: models.OuterHeaderCreationGtpUUdpIpv4,
				TEID:        0,
			},
		},
	}
	info := farInfoFromModel(far, localIPv4, localIPv6)

	wantLocal := ebpf.IPToIn6Addr(localIPv4)
	if info.LocalIP != wantLocal {
		t.Errorf("LocalIP (no address): got %v, want %v", info.LocalIP, wantLocal)
	}

	var zeroRemote [16]byte
	if info.RemoteIP != zeroRemote {
		t.Errorf("RemoteIP (no address): got %v, want zero", info.RemoteIP)
	}
}

// TestFarInfoFromModel_ApplyActionDrop verifies Drop action encoding.
func TestFarInfoFromModel_ApplyActionDrop(t *testing.T) {
	far := models.FAR{
		FARID:       2,
		ApplyAction: models.ApplyAction{Drop: true},
	}
	info := farInfoFromModel(far, localIPv4, localIPv6)

	if info.Action != 0x01 {
		t.Errorf("Action (Drop): got %d, want 0x01", info.Action)
	}
}

// A forwarding rule is stated in full on every apply, so a rule whose transport
// changes family carries the new one and nothing of the old.
func TestFarInfoFromModel_IPv4ToIPv6(t *testing.T) {
	ipv4FAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 10, "10.0.0.1", "")
	if before := farInfoFromModel(ipv4FAR, localIPv4, localIPv6); before.TeID != 10 {
		t.Fatalf("TeID over IPv4 transport: got %d, want 10", before.TeID)
	}

	ipv6FAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv6, 20, "", "2001:db8::2")

	info := farInfoFromModel(ipv6FAR, localIPv4, localIPv6)

	if info.OuterHeaderCreation != 2 {
		t.Errorf("OuterHeaderCreation: got %d, want 2", info.OuterHeaderCreation)
	}

	if info.TeID != 20 {
		t.Errorf("TeID: got %d, want 20", info.TeID)
	}

	wantLocal := ebpf.IPToIn6Addr(localIPv6)
	if info.LocalIP != wantLocal {
		t.Errorf("LocalIP: got %v, want %v", info.LocalIP, wantLocal)
	}

	wantRemote := ebpf.IPToIn6Addr(netip.MustParseAddr("2001:db8::2"))
	if info.RemoteIP != wantRemote {
		t.Errorf("RemoteIP: got %v, want %v", info.RemoteIP, wantRemote)
	}
}

// A forwarding rule with no forwarding parameters names no tunnel endpoint.
// Nothing of a rule the session held before survives the statement, so an
// endpoint the control plane withdrew cannot be left forwarding.
func TestFarInfoFromModel_NoForwardingParametersNamesNoEndpoint(t *testing.T) {
	bound := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 42, "10.0.0.5", "")
	if before := farInfoFromModel(bound, localIPv4, localIPv6); before.TeID != 42 {
		t.Fatalf("TeID after bind: got %d, want 42", before.TeID)
	}

	withdrawn := models.FAR{
		FARID:       1,
		ApplyAction: models.ApplyAction{Forw: true},
	}

	info := farInfoFromModel(withdrawn, localIPv4, localIPv6)

	var zeroIP [16]byte

	if info.TeID != 0 || info.OuterHeaderCreation != 0 || info.RemoteIP != zeroIP || info.LocalIP != zeroIP {
		t.Errorf("withdrawn forwarding parameters left an endpoint: %+v", info)
	}
}

// The downlink suspend (buffer + notify with the outer header creation
// withdrawn) must name no tunnel endpoint: an endpoint left behind keeps the
// datapath forwarding to an access the session has left.
func TestFarInfoFromModel_SuspendedDownlinkNamesNoEndpoint(t *testing.T) {
	boundFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 0x9001, "10.0.0.9", "")
	boundFAR.ForwardingParameters.OuterHeaderCreation.S1U = true

	if before := farInfoFromModel(boundFAR, localIPv4, localIPv6); before.TeID != 0x9001 {
		t.Fatalf("TeID after bind: got %#x, want 0x9001", before.TeID)
	}

	suspend := models.FAR{
		FARID:                1,
		ApplyAction:          models.ApplyAction{Buff: true, Nocp: true},
		ForwardingParameters: &models.ForwardingParameters{},
	}

	info := farInfoFromModel(suspend, localIPv4, localIPv6)

	if info.Action != 0x0c {
		t.Errorf("Action: got %#x, want 0x0c", info.Action)
	}

	if info.OuterHeaderCreation != 0 {
		t.Errorf("OuterHeaderCreation: got %#x, want 0", info.OuterHeaderCreation)
	}

	if info.TeID != 0 {
		t.Errorf("TeID: got %#x, want 0", info.TeID)
	}

	var zeroIP [16]byte

	if info.RemoteIP != zeroIP {
		t.Errorf("RemoteIP: got %v, want zero", info.RemoteIP)
	}

	if info.LocalIP != zeroIP {
		t.Errorf("LocalIP: got %v, want zero", info.LocalIP)
	}
}

func TestFarInfoFromModel_S1UKeepsNoPSC(t *testing.T) {
	establishFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 10, "10.0.0.1", "")
	establishFAR.ForwardingParameters.OuterHeaderCreation.S1U = true

	if info := farInfoFromModel(establishFAR, localIPv4, localIPv6); info.OuterHeaderCreation != 0x11 {
		t.Fatalf("OuterHeaderCreation after establish: got %#x, want 0x11", info.OuterHeaderCreation)
	}

	rebindFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 84, "10.0.0.1", "")
	rebindFAR.ForwardingParameters.OuterHeaderCreation.S1U = true

	if info := farInfoFromModel(rebindFAR, localIPv4, localIPv6); info.OuterHeaderCreation != 0x11 {
		t.Errorf("OuterHeaderCreation after rebind: got %#x, want 0x11", info.OuterHeaderCreation)
	}
}
