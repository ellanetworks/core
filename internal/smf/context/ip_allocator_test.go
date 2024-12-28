package context_test

import (
	"fmt"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/smf/context"
)

func TestIPAllocator_SingleIMSI(t *testing.T) {
	cidr := "192.168.1.0/24"

	mockStore := func(imsi string, ip *net.IP) error {
		return nil
	}

	allocator, err := context.NewIPAllocator(cidr, mockStore)
	if err != nil {
		t.Fatalf("failed to create IP allocator: %v", err)
	}

	imsi := "IMSI12345"

	ip, err := allocator.Allocate(imsi)
	if err != nil {
		t.Fatalf("failed to allocate IP: %v", err)
	}
	if ip == nil {
		t.Fatalf("allocated IP should not be nil")
	}

	_, ipNet, _ := net.ParseCIDR(cidr)
	if !ipNet.Contains(ip) {
		t.Fatalf("allocated IP should be within the CIDR range")
	}

	ip2, err := allocator.Allocate(imsi)
	if err != nil {
		t.Fatalf("failed to allocate IP again for the same IMSI: %v", err)
	}
	if !ip.Equal(ip2) {
		t.Fatalf("re-allocating the same IMSI should return the same IP")
	}

	err = allocator.Release(imsi)
	if err != nil {
		t.Fatalf("failed to release IP: %v", err)
	}

	ip3, err := allocator.Allocate(imsi)
	if err != nil {
		t.Fatalf("failed to allocate a new IP after release: %v", err)
	}
	if ip3 == nil {
		t.Fatalf("newly allocated IP should not be nil")
	}
	if !ipNet.Contains(ip3) {
		t.Fatalf("newly allocated IP should be within the CIDR range")
	}

	newImsi := "IMSI54321"
	ip4, err := allocator.Allocate(newImsi)
	if err != nil {
		t.Fatalf("failed to allocate IP for a new IMSI: %v", err)
	}
	if ip4 == nil {
		t.Fatalf("allocated IP for a new IMSI should not be nil")
	}

	if ip3.Equal(ip4) {
		t.Fatalf("allocated IP for a new IMSI should be different from the previous IMSI")
	}
}

func TestIPAllocator_ExhaustAllIPs(t *testing.T) {
	cidr := "192.168.1.0/24"

	mockStore := func(imsi string, ip *net.IP) error {
		return nil
	}

	allocator, err := context.NewIPAllocator(cidr, mockStore)
	if err != nil {
		t.Fatalf("failed to create IP allocator: %v", err)
	}

	_, ipNet, _ := net.ParseCIDR(cidr)
	maskBits, totalBits := ipNet.Mask.Size()
	totalIPs := 1 << (totalBits - maskBits)

	allocatedIPs := make(map[string]struct{})

	// Allocate all possible IPs in the range.
	for i := 1; i < totalIPs-1; i++ { // Skip network (0) and broadcast (-1) addresses.
		imsi := fmt.Sprintf("IMSI%d", i)
		ip, err := allocator.Allocate(imsi)
		if err != nil {
			t.Fatalf("failed to allocate IP for IMSI %s: %v", imsi, err)
		}

		if !ipNet.Contains(ip) {
			t.Fatalf("allocated IP %s is not within the CIDR range", ip.String())
		}

		ipStr := ip.String()
		if _, exists := allocatedIPs[ipStr]; exists {
			t.Fatalf("IP %s was allocated more than once", ipStr)
		}

		allocatedIPs[ipStr] = struct{}{}
	}

	_, err = allocator.Allocate("IMSIOverflow")
	if err == nil {
		t.Fatalf("allocator should return an error when the range is exhausted")
	}

	if len(allocatedIPs) != totalIPs-2 {
		t.Fatalf("expected %d allocated IPs, got %d", totalIPs-2, len(allocatedIPs))
	}
}
