package context

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
)

type IPAllocator struct {
	ipNetwork *net.IPNet
	g         *_IDPool
}

func NewIPAllocator(cidr string) (*IPAllocator, error) {
	allocator := &IPAllocator{}

	if _, ipnet, err := net.ParseCIDR(cidr); err != nil {
		return nil, err
	} else {
		allocator.ipNetwork = ipnet
	}
	allocator.g = newIDPool(1, 1<<int64(32-maskBits(allocator.ipNetwork.Mask))-2)

	return allocator, nil
}

func maskBits(mask net.IPMask) int {
	var cnt int
	for _, b := range mask {
		for ; b != 0; b /= 2 {
			if b%2 != 0 {
				cnt++
			}
		}
	}
	return cnt
}

// IPAddrWithOffset add offset on base ip
func IPAddrWithOffset(ip net.IP, offset int) net.IP {
	retIP := make(net.IP, len(ip))
	copy(retIP, ip)

	var carry int
	for i := len(retIP) - 1; i >= 0; i-- {
		if offset == 0 {
			break
		}

		val := int(retIP[i]) + carry + offset%256
		retIP[i] = byte(val % 256)
		carry = val / 256

		offset /= 256
	}

	return retIP
}

// IPAddrOffset calculate the input ip with base ip offset
func IPAddrOffset(in, base net.IP) int {
	offset := 0
	exp := 1
	for i := len(base) - 1; i >= 0; i-- {
		offset += int(in[i]-base[i]) * exp
		exp *= 256
	}
	return offset
}

// Allocate will allocate the IP address and returns it
func (a *IPAllocator) Allocate(imsi string) (net.IP, error) {
	// check if static IP already reserved for this IMSI
	if a.g.staticIps != nil {
		staticIps := *a.g.staticIps
		if ipStr := staticIps[imsi]; ipStr != "" {
			return net.ParseIP(ipStr).To4(), nil
		}
	}

	if offset, err := a.g.allocate(); err != nil {
		return nil, errors.New("ip allocation failed" + err.Error())
	} else {
		smfCountStr := os.Getenv("SMF_COUNT")
		if smfCountStr == "" {
			smfCountStr = "1"
		}
		smfCount, _ := strconv.Atoi(smfCountStr)
		ip := IPAddrWithOffset(a.ipNetwork.IP, int(offset)+(smfCount-1)*5000)
		fmt.Printf("unique id - ip %v \n", ip)
		fmt.Printf("unique id - offset %v \n", offset)
		fmt.Printf("unique id - smfCount %v \n", smfCount)
		return ip, nil
	}
}

func (a *IPAllocator) ReserveStaticIps(ips *map[string]string) {
	a.g.staticIps = ips
	for _, ipStr := range *ips {
		if ip := net.ParseIP(ipStr).To4(); ip != nil {
			// block static IPs in pool to avoid dynamic allocation
			a.BlockIp(ip)
		}
	}
}

func (a *IPAllocator) BlockIp(ip net.IP) {
	offset := IPAddrOffset(ip, a.ipNetwork.IP)
	a.g.block(int64(offset))
}

func (a *IPAllocator) Release(imsi string, ip net.IP) {
	// Don't release static IPs
	if a.g.staticIps != nil {
		staticIps := *a.g.staticIps
		if ipStr := staticIps[imsi]; ipStr != "" {
			return
		}
	}

	offset := IPAddrOffset(ip, a.ipNetwork.IP)
	a.g.release(int64(offset))
}

type _IDPool struct {
	staticIps *map[string]string // map of [imsi]ip
	isUsed    map[int64]bool
	minValue  int64
	maxValue  int64
	index     int64
	lock      sync.Mutex
}

func newIDPool(minValue int64, maxValue int64) (idPool *_IDPool) {
	idPool = new(_IDPool)
	idPool.minValue = minValue
	idPool.maxValue = maxValue
	idPool.isUsed = make(map[int64]bool)
	idPool.index = 1
	return
}

func (i *_IDPool) allocate() (id int64, err error) {
	i.lock.Lock()
	defer i.lock.Unlock()

	for id = i.index; id <= i.maxValue; id++ {
		if _, exist := i.isUsed[id]; !exist {
			i.isUsed[id] = true
			i.index = (id % i.maxValue) + 1
			return id, nil
		}
	}

	for id = 1; id < i.index; id++ {
		if _, exist := i.isUsed[id]; !exist {
			i.isUsed[id] = true
			i.index = id + 1
			return id, nil
		}
	}

	return 0, errors.New("no available value range to allocate id")
}

func (i *_IDPool) block(id int64) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.isUsed[id] = true
}

func (i *_IDPool) release(id int64) {
	i.lock.Lock()
	defer i.lock.Unlock()
	delete(i.isUsed, id)
}
