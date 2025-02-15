package ebpf

import (
	"fmt"
	"net"
)

// PdrInfo represents the generic PDR information.
type PdrInfo struct {
	OuterHeaderRemoval uint8
	FarId              uint32
	QerId              uint32
	SdfFilter          *SdfFilter
}

// SdfFilter holds SDF filtering criteria.
type SdfFilter struct {
	Protocol     uint8 // 0: icmp, 1: ip, 2: tcp, 3: udp, 4: icmp6
	SrcAddress   IpWMask
	SrcPortRange PortRange
	DstAddress   IpWMask
	DstPortRange PortRange
}

// IpWMask represents an IP address and mask.
type IpWMask struct {
	Type uint8 // 0: any, 1: ip4, 2: ip6
	Ip   net.IP
	Mask net.IPMask
}

// PortRange defines a range for ports.
type PortRange struct {
	LowerBound uint16
	UpperBound uint16
}

// ---------------- UPLINK (N3) Helpers ----------------

// PreprocessPdrWithSdfN3 looks up an existing uplink PDR and combines it with provided SDF info.
func PreprocessPdrWithSdfN3(lookup func(interface{}, interface{}) error, key interface{}, pdrInfo PdrInfo) (N3EntrypointPdrInfo, error) {
	var defaultPdr N3EntrypointPdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombinePdrWithSdfN3(nil, pdrInfo), nil
	}
	return CombinePdrWithSdfN3(&defaultPdr, pdrInfo), nil
}

// ToN3EntrypointPdrInfo converts a generic PdrInfo to an uplink PDR info.
func ToN3EntrypointPdrInfo(pdr PdrInfo) N3EntrypointPdrInfo {
	var pdrToStore N3EntrypointPdrInfo
	pdrToStore.OuterHeaderRemoval = pdr.OuterHeaderRemoval
	pdrToStore.FarId = pdr.FarId
	pdrToStore.QerId = pdr.QerId
	return pdrToStore
}

// CombinePdrWithSdfN3 combines a default uplink PDR with the SDF information.
func CombinePdrWithSdfN3(defaultPdr *N3EntrypointPdrInfo, sdfPdr PdrInfo) N3EntrypointPdrInfo {
	var pdrToStore N3EntrypointPdrInfo
	// Default mapping options.
	if defaultPdr != nil {
		pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
		pdrToStore.FarId = defaultPdr.FarId
		pdrToStore.QerId = defaultPdr.QerId
		pdrToStore.SdfMode = 2
	} else {
		pdrToStore.SdfMode = 1
	}

	// SDF mapping options.
	pdrToStore.SdfRules.SdfFilter.Protocol = sdfPdr.SdfFilter.Protocol
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Type = sdfPdr.SdfFilter.SrcAddress.Type
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.Ip)
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.Mask)
	pdrToStore.SdfRules.SdfFilter.SrcPort.LowerBound = sdfPdr.SdfFilter.SrcPortRange.LowerBound
	pdrToStore.SdfRules.SdfFilter.SrcPort.UpperBound = sdfPdr.SdfFilter.SrcPortRange.UpperBound
	pdrToStore.SdfRules.SdfFilter.DstAddr.Type = sdfPdr.SdfFilter.DstAddress.Type
	pdrToStore.SdfRules.SdfFilter.DstAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.DstAddress.Ip)
	pdrToStore.SdfRules.SdfFilter.DstAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.DstAddress.Mask)
	pdrToStore.SdfRules.SdfFilter.DstPort.LowerBound = sdfPdr.SdfFilter.DstPortRange.LowerBound
	pdrToStore.SdfRules.SdfFilter.DstPort.UpperBound = sdfPdr.SdfFilter.DstPortRange.UpperBound
	pdrToStore.SdfRules.OuterHeaderRemoval = sdfPdr.OuterHeaderRemoval
	pdrToStore.SdfRules.FarId = sdfPdr.FarId
	pdrToStore.SdfRules.QerId = sdfPdr.QerId
	return pdrToStore
}

// ---------------- DOWNLINK (N6) Helpers ----------------

// PreprocessPdrWithSdfN6 looks up an existing downlink PDR and combines it with provided SDF info.
func PreprocessPdrWithSdfN6(lookup func(interface{}, interface{}) error, key interface{}, pdrInfo PdrInfo) (N6EntrypointPdrInfo, error) {
	var defaultPdr N6EntrypointPdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombinePdrWithSdfN6(nil, pdrInfo), nil
	}
	return CombinePdrWithSdfN6(&defaultPdr, pdrInfo), nil
}

// ToN6EntrypointPdrInfo converts a generic PdrInfo to a downlink PDR info.
func ToN6EntrypointPdrInfo(pdr PdrInfo) N6EntrypointPdrInfo {
	var pdrToStore N6EntrypointPdrInfo
	pdrToStore.OuterHeaderRemoval = pdr.OuterHeaderRemoval
	pdrToStore.FarId = pdr.FarId
	pdrToStore.QerId = pdr.QerId
	return pdrToStore
}

// CombinePdrWithSdfN6 combines a default downlink PDR with the SDF information.
func CombinePdrWithSdfN6(defaultPdr *N6EntrypointPdrInfo, sdfPdr PdrInfo) N6EntrypointPdrInfo {
	var pdrToStore N6EntrypointPdrInfo
	// Default mapping options.
	if defaultPdr != nil {
		pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
		pdrToStore.FarId = defaultPdr.FarId
		pdrToStore.QerId = defaultPdr.QerId
		pdrToStore.SdfMode = 2
	} else {
		pdrToStore.SdfMode = 1
	}

	// SDF mapping options.
	pdrToStore.SdfRules.SdfFilter.Protocol = sdfPdr.SdfFilter.Protocol
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Type = sdfPdr.SdfFilter.SrcAddress.Type
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.Ip)
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.Mask)
	pdrToStore.SdfRules.SdfFilter.SrcPort.LowerBound = sdfPdr.SdfFilter.SrcPortRange.LowerBound
	pdrToStore.SdfRules.SdfFilter.SrcPort.UpperBound = sdfPdr.SdfFilter.SrcPortRange.UpperBound
	pdrToStore.SdfRules.SdfFilter.DstAddr.Type = sdfPdr.SdfFilter.DstAddress.Type
	pdrToStore.SdfRules.SdfFilter.DstAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.DstAddress.Ip)
	pdrToStore.SdfRules.SdfFilter.DstAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.DstAddress.Mask)
	pdrToStore.SdfRules.SdfFilter.DstPort.LowerBound = sdfPdr.SdfFilter.DstPortRange.LowerBound
	pdrToStore.SdfRules.SdfFilter.DstPort.UpperBound = sdfPdr.SdfFilter.DstPortRange.UpperBound
	pdrToStore.SdfRules.OuterHeaderRemoval = sdfPdr.OuterHeaderRemoval
	pdrToStore.SdfRules.FarId = sdfPdr.FarId
	pdrToStore.SdfRules.QerId = sdfPdr.QerId
	return pdrToStore
}

func Copy16Ip[T ~[]byte](arr T) [16]byte {
	const Ipv4len = 4
	const Ipv6len = 16
	var c [Ipv6len]byte
	var arrLen int
	if len(arr) == Ipv4len {
		arrLen = Ipv4len
	} else if len(arr) == Ipv6len {
		arrLen = Ipv6len
	} else if len(arr) == 0 || arr == nil {
		return c
	}
	for i := 0; i < arrLen; i++ {
		c[i] = (arr)[arrLen-1-i]
	}
	return c
}

func (sdfFilter *SdfFilter) String() string {
	return fmt.Sprintf("%+v", *sdfFilter)
}
