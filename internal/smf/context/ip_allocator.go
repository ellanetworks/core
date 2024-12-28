package context

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type IPAllocator struct {
	ipNetwork *net.IPNet
	imsiToIP  map[string]net.IP
	mutex     sync.Mutex
	storeFunc func(string, *net.IP) error // Function to store IPs
}

func NewIPAllocator(cidr string, storeFunc func(string, *net.IP) error) (*IPAllocator, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	return &IPAllocator{
		ipNetwork: ipnet,
		imsiToIP:  make(map[string]net.IP),
		storeFunc: storeFunc,
	}, nil
}

func (a *IPAllocator) Allocate(imsi string) (net.IP, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if ip, exists := a.imsiToIP[imsi]; exists {
		err := a.storeFunc(imsi, &ip)
		if err != nil {
			return nil, fmt.Errorf("couldn't store IP address for IMSI %s: %v", imsi, err)
		}
		return ip, nil
	}

	baseIP := a.ipNetwork.IP
	maskBits, totalBits := a.ipNetwork.Mask.Size()
	totalIPs := 1 << (totalBits - maskBits)

	for i := 1; i < totalIPs-1; i++ { // Skip network and broadcast addresses.
		ip := AddOffsetToIP(baseIP, i)
		if !a.isIPAllocated(ip) {
			a.imsiToIP[imsi] = ip
			err := a.storeFunc(imsi, &ip)
			if err != nil {
				return nil, fmt.Errorf("couldn't store IP address for IMSI %s: %v", imsi, err)
			}
			return ip, nil
		}
	}

	return nil, errors.New("no available IP addresses")
}

func (a *IPAllocator) Release(imsi string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if _, exists := a.imsiToIP[imsi]; !exists {
		return errors.New("no IP allocated for this IMSI")
	}

	delete(a.imsiToIP, imsi)
	return nil
}

func (a *IPAllocator) isIPAllocated(ip net.IP) bool {
	for _, allocatedIP := range a.imsiToIP {
		if allocatedIP.Equal(ip) {
			return true
		}
	}
	return false
}

func AddOffsetToIP(baseIP net.IP, offset int) net.IP {
	resultIP := make(net.IP, len(baseIP))
	copy(resultIP, baseIP)

	for i := len(resultIP) - 1; i >= 0; i-- {
		offset += int(resultIP[i])
		resultIP[i] = byte(offset)
		offset >>= 8
	}

	return resultIP
}
