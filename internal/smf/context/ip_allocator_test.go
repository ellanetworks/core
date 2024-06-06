package context_test

import (
	"fmt"
	"net"
	"testing"

	smf_context "github.com/yeastengine/ella/internal/smf/context"
)

func TestIPPoolAlloc(t *testing.T) {
	allocator, err := smf_context.NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Errorf("failed to allocate pool %v", err)
	}

	var allocAddresses []string
	// check if we can allocate 254 addresses
	for i := 1; i <= 254; i++ {
		ip, err := allocator.Allocate("")
		if err != nil {
			t.Errorf("failed to allocate pool %v", err)
		}
		fmt.Printf("Allocated address = %v\n", ip)
		allocAddresses = append(allocAddresses, ip.String())
	}

	// Test what happens if we releae all addresses
	for _, ips := range allocAddresses {
		ip := net.ParseIP(ips)
		allocator.Release("", ip)
	}

	// Check what happens if we try to release unknown address
	ip := net.ParseIP("192.168.2.1")
	allocator.Release("", ip)
}

func TestIPPoolAllocRelease(t *testing.T) {
	allocator, err := smf_context.NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Errorf("failed to allocate pool %v", err)
	}

	ip1 := net.ParseIP("192.168.1.1")
	for i := 1; i <= 255; i++ {
		ip, err := allocator.Allocate("")
		if err != nil {
			t.Errorf("failed to allocate pool %v", err)
		}
		fmt.Printf("Allocated address = %v\n", ip)
		if i == 1 {
			if ip.Equal(ip1) == false {
				t.Errorf("address not allocated in order ? allocated address %v", ip1)
			}
			allocator.Release("", ip)
		}
		if i == 2 {
			ip2 := net.ParseIP("192.168.1.2")
			if ip.Equal(ip2) == false {
				t.Errorf("address not allocated in order ? allocated address %v", ip2)
			}
		}
		// rollover, we should be using first address again
		if i == 255 && ip.Equal(ip1) != true {
			t.Errorf("Failed to allocate IP address = %v %v \n", ip, ip1)
		}
	}
}

func TestIPPoolAllocLeastRecentlyUsed(t *testing.T) {
	allocator, err := smf_context.NewIPAllocator("192.168.1.0/24")
	if err != nil {
		t.Errorf("failed to allocate pool %v", err)
	}

	ip1, err := allocator.Allocate("")
	if err != nil {
		t.Errorf("failed to allocate pool %v", err)
	}
	fmt.Printf("Allocated address = %v\n", ip1)
	allocator.Release("", ip1)
	ip2, err := allocator.Allocate("")
	if err != nil {
		t.Errorf("failed to allocate pool %v", err)
	}
	fmt.Printf("Allocated address = %v\n", ip2)

	// Same address is not allocate again..
	if ip1.Equal(ip2) {
		t.Errorf("ip1 %v & ip2 %v same ", ip1, ip2)
	}
}
