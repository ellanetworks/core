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

func TestFarInfoFromModel_IPv4(t *testing.T) {
	far := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 100, "192.168.0.10", "")
	info := farInfoFromMerge(far, localIPv4, localIPv6, ebpf.FarInfo{})

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

func TestFarInfoFromModel_IPv6(t *testing.T) {
	far := buildFAR(models.OuterHeaderCreationGtpUUdpIpv6, 200, "", "2001:db8::cafe")
	info := farInfoFromMerge(far, localIPv4, localIPv6, ebpf.FarInfo{})

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
	info := farInfoFromMerge(far, localIPv4, localIPv6, ebpf.FarInfo{})

	wantLocal := ebpf.IPToIn6Addr(localIPv4)
	if info.LocalIP != wantLocal {
		t.Errorf("LocalIP (no address): got %v, want %v", info.LocalIP, wantLocal)
	}

	var zeroRemote [16]byte
	if info.RemoteIP != zeroRemote {
		t.Errorf("RemoteIP (no address): got %v, want zero", info.RemoteIP)
	}
}

func TestFarInfoFromModel_ApplyActionDrop(t *testing.T) {
	far := models.FAR{
		FARID:       2,
		ApplyAction: models.ApplyAction{Drop: true},
	}
	info := farInfoFromMerge(far, localIPv4, localIPv6, ebpf.FarInfo{})

	if info.Action != 0x01 {
		t.Errorf("Action (Drop): got %d, want 0x01", info.Action)
	}
}

func TestFarInfoFromMerge_IPv4ToIPv6(t *testing.T) {
	existingFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 10, "10.0.0.1", "")
	existing := farInfoFromMerge(existingFAR, localIPv4, localIPv6, ebpf.FarInfo{})

	updateFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv6, 20, "", "2001:db8::2")

	merged := farInfoFromMerge(updateFAR, localIPv4, localIPv6, existing)

	if merged.OuterHeaderCreation != 2 {
		t.Errorf("OuterHeaderCreation: got %d, want 2", merged.OuterHeaderCreation)
	}

	if merged.TeID != 20 {
		t.Errorf("TeID: got %d, want 20", merged.TeID)
	}

	wantLocal := ebpf.IPToIn6Addr(localIPv6)
	if merged.LocalIP != wantLocal {
		t.Errorf("LocalIP: got %v, want %v", merged.LocalIP, wantLocal)
	}

	wantRemote := ebpf.IPToIn6Addr(netip.MustParseAddr("2001:db8::2"))
	if merged.RemoteIP != wantRemote {
		t.Errorf("RemoteIP: got %v, want %v", merged.RemoteIP, wantRemote)
	}
}

func TestFarInfoFromMerge_NoUpdate(t *testing.T) {
	existingFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 42, "10.0.0.5", "")
	existing := farInfoFromMerge(existingFAR, localIPv4, localIPv6, ebpf.FarInfo{})

	update := models.FAR{
		FARID:       1,
		ApplyAction: models.ApplyAction{Forw: true},
	}

	merged := farInfoFromMerge(update, localIPv4, localIPv6, existing)

	if merged.TeID != existing.TeID {
		t.Errorf("TeID preserved: got %d, want %d", merged.TeID, existing.TeID)
	}

	if merged.RemoteIP != existing.RemoteIP {
		t.Errorf("RemoteIP preserved: got %v, want %v", merged.RemoteIP, existing.RemoteIP)
	}
}

func TestFarInfoFromMerge_S1UKeepsNoPSC(t *testing.T) {
	establishFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 10, "10.0.0.1", "")
	establishFAR.ForwardingParameters.OuterHeaderCreation.S1U = true

	existing := farInfoFromMerge(establishFAR, localIPv4, localIPv6, ebpf.FarInfo{})
	if existing.OuterHeaderCreation != 0x11 {
		t.Fatalf("OuterHeaderCreation after establish: got %#x, want 0x11", existing.OuterHeaderCreation)
	}

	updateFAR := buildFAR(models.OuterHeaderCreationGtpUUdpIpv4, 84, "10.0.0.1", "")
	updateFAR.ForwardingParameters.OuterHeaderCreation.S1U = true

	merged := farInfoFromMerge(updateFAR, localIPv4, localIPv6, existing)
	if merged.OuterHeaderCreation != 0x11 {
		t.Errorf("OuterHeaderCreation after modify: got %#x, want 0x11", merged.OuterHeaderCreation)
	}
}
