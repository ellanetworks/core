package ebpf

import (
	"fmt"
	"net"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
)

type PdrInfo struct {
	OuterHeaderRemoval uint8
	FarId              uint32
	QerId              uint32
	SdfFilter          *SdfFilter
}

type SdfFilter struct {
	Protocol     uint8 // 0: icmp, 1: ip, 2: tcp, 3: udp, 4: icmp6
	SrcAddress   IpWMask
	SrcPortRange PortRange
	DstAddress   IpWMask
	DstPortRange PortRange
}

type IpWMask struct {
	Type uint8 // 0: any, 1: ip4, 2: ip6
	Ip   net.IP
	Mask net.IPMask
}

type PortRange struct {
	LowerBound uint16
	UpperBound uint16
}

type FarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	Teid                  uint32
	RemoteIP              uint32
	LocalIP               uint32
	TransportLevelMarking uint16
}

// PreprocessN3PdrWithSdf looks up the existing N3 PDR for a given key and combines it with the SDF settings.
func PreprocessN3PdrWithSdf(lookup func(interface{}, interface{}) error, key interface{}, pdrInfo PdrInfo) (N3EntrypointPdrInfo, error) {
	var defaultPdr N3EntrypointPdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombineN3PdrWithSdf(nil, pdrInfo), nil
	}
	return CombineN3PdrWithSdf(&defaultPdr, pdrInfo), nil
}

func (bpfObjects *BpfObjects) PutPdrUplink(teid uint32, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Put PDR Uplink: teid=%d, pdrInfo=%+v", teid, pdrInfo)
	var pdrToStore N3EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN3PdrWithSdf(bpfObjects.N3EntrypointObjects.PdrMapUplinkIp4.Lookup, teid, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN3EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N3EntrypointObjects.PdrMapUplinkIp4.Put(teid, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) UpdatePdrUplink(teid uint32, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Update PDR Uplink: teid=%d, pdrInfo=%+v", teid, pdrInfo)
	var pdrToStore N3EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN3PdrWithSdf(bpfObjects.N3EntrypointObjects.PdrMapUplinkIp4.Lookup, teid, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN3EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N3EntrypointObjects.PdrMapUplinkIp4.Update(teid, unsafe.Pointer(&pdrToStore), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeletePdrUplink(teid uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete PDR Uplink: teid=%d", teid)
	return bpfObjects.N3EntrypointObjects.PdrMapUplinkIp4.Delete(teid)
}

func ToN3EntrypointPdrInfo(pdrInfo PdrInfo) N3EntrypointPdrInfo {
	var pdrToStore N3EntrypointPdrInfo
	pdrToStore.OuterHeaderRemoval = pdrInfo.OuterHeaderRemoval
	pdrToStore.FarId = pdrInfo.FarId
	pdrToStore.QerId = pdrInfo.QerId
	return pdrToStore
}

func CombineN3PdrWithSdf(defaultPdr *N3EntrypointPdrInfo, sdfPdr PdrInfo) N3EntrypointPdrInfo {
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

// --- DOWNLINK (N6) Functions ---

// PreprocessN6PdrWithSdf looks up the existing N6 PDR for a given key and combines it with the SDF settings.
func PreprocessN6PdrWithSdf(lookup func(interface{}, interface{}) error, key interface{}, pdrInfo PdrInfo) (N6EntrypointPdrInfo, error) {
	var defaultPdr N6EntrypointPdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombineN6PdrWithSdf(nil, pdrInfo), nil
	}
	return CombineN6PdrWithSdf(&defaultPdr, pdrInfo), nil
}

func (bpfObjects *BpfObjects) PutPdrDownlink(ipv4 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Put PDR Downlink: ipv4=%s, pdrInfo=%+v", ipv4, pdrInfo)
	var pdrToStore N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN6PdrWithSdf(bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp4.Lookup, ipv4, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp4.Put(ipv4, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) NewUplinkFar(farInfo FarInfo) (uint32, error) {
	internalId, err := bpfObjects.FarIdTracker.GetNext()
	if err != nil {
		return 0, err
	}
	logger.UpfLog.Debugf("EBPF: Put FAR Uplink: internalId=%d, qerInfo=%+v", internalId, farInfo)
	return internalId, bpfObjects.N3EntrypointObjects.FarMap.Put(internalId, unsafe.Pointer(&farInfo))
}

func (bpfObjects *BpfObjects) UpdateUplinkFar(internalId uint32, farInfo FarInfo) error {
	logger.UpfLog.Debugf("EBPF: Update Uplink FAR: internalId=%d, qerInfo=%+v", internalId, farInfo)
	return bpfObjects.N3EntrypointObjects.FarMap.Update(internalId, unsafe.Pointer(&farInfo), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeleteUplinkFar(intenalId uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete Uplink FAR: intenalId=%d", intenalId)
	bpfObjects.FarIdTracker.Release(intenalId)
	return bpfObjects.N3EntrypointObjects.FarMap.Delete(intenalId)
}

func (bpfObjects *BpfObjects) NewDownlinkFar(farInfo FarInfo) (uint32, error) {
	internalId, err := bpfObjects.FarIdTracker.GetNext()
	if err != nil {
		return 0, err
	}
	logger.UpfLog.Debugf("EBPF: Put FAR Downlink: internalId=%d, qerInfo=%+v", internalId, farInfo)
	return internalId, bpfObjects.N6EntrypointObjects.FarMap.Put(internalId, unsafe.Pointer(&farInfo))
}

func (bpfObjects *BpfObjects) UpdateDownlinkFar(internalId uint32, farInfo FarInfo) error {
	logger.UpfLog.Debugf("EBPF: Update Downlink FAR: internalId=%d, qerInfo=%+v", internalId, farInfo)
	return bpfObjects.N6EntrypointObjects.FarMap.Update(internalId, unsafe.Pointer(&farInfo), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeleteDownlinkFar(intenalId uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete Downlink FAR: intenalId=%d", intenalId)
	bpfObjects.FarIdTracker.Release(intenalId)
	return bpfObjects.N6EntrypointObjects.FarMap.Delete(intenalId)
}

type QerInfo struct {
	GateStatusUL uint8
	GateStatusDL uint8
	Qfi          uint8
	MaxBitrateUL uint32
	MaxBitrateDL uint32
	StartUL      uint64
	StartDL      uint64
}

func (bpfObjects *BpfObjects) NewUplinkQer(qerInfo QerInfo) (uint32, error) {
	internalId, err := bpfObjects.QerIdTracker.GetNext()
	if err != nil {
		return 0, err
	}
	logger.UpfLog.Debugf("EBPF: Put QER Uplink: internalId=%d, qerInfo=%+v", internalId, qerInfo)
	return internalId, bpfObjects.N3EntrypointObjects.QerMap.Put(internalId, unsafe.Pointer(&qerInfo))
}

func (bpfObjects *BpfObjects) UpdateUplinkQer(internalId uint32, qerInfo QerInfo) error {
	logger.UpfLog.Debugf("EBPF: Update Uplink QER: internalId=%d, qerInfo=%+v", internalId, qerInfo)
	return bpfObjects.N3EntrypointObjects.QerMap.Update(internalId, unsafe.Pointer(&qerInfo), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeleteUplinkQer(internalId uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete Uplink QER: internalId=%d", internalId)
	bpfObjects.QerIdTracker.Release(internalId)
	return bpfObjects.N3EntrypointObjects.QerMap.Delete(internalId)
}

func (bpfObjects *BpfObjects) NewDownlinkQer(qerInfo QerInfo) (uint32, error) {
	internalId, err := bpfObjects.QerIdTracker.GetNext()
	if err != nil {
		return 0, err
	}
	logger.UpfLog.Debugf("EBPF: Put QER Downlink: internalId=%d, qerInfo=%+v", internalId, qerInfo)
	return internalId, bpfObjects.N6EntrypointObjects.QerMap.Put(internalId, unsafe.Pointer(&qerInfo))
}

func (bpfObjects *BpfObjects) UpdateDownlinkQer(internalId uint32, qerInfo QerInfo) error {
	logger.UpfLog.Debugf("EBPF: Update Downlink QER: internalId=%d, qerInfo=%+v", internalId, qerInfo)
	return bpfObjects.N6EntrypointObjects.QerMap.Update(internalId, unsafe.Pointer(&qerInfo), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeleteDownlinkQer(internalId uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete Downlink QER: internalId=%d", internalId)
	bpfObjects.QerIdTracker.Release(internalId)
	return bpfObjects.N6EntrypointObjects.QerMap.Delete(internalId)
}

func (bpfObjects *BpfObjects) UpdatePdrDownlink(ipv4 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Update PDR Downlink: ipv4=%s, pdrInfo=%+v", ipv4, pdrInfo)
	var pdrToStore N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN6PdrWithSdf(bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp4.Lookup, ipv4, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp4.Update(ipv4, unsafe.Pointer(&pdrToStore), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeletePdrDownlink(ipv4 net.IP) error {
	logger.UpfLog.Debugf("EBPF: Delete PDR Downlink: ipv4=%s", ipv4)
	return bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp4.Delete(ipv4)
}

func (bpfObjects *BpfObjects) PutDownlinkPdrIp6(ipv6 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Put PDR Ipv6 Downlink: ipv6=%s, pdrInfo=%+v", ipv6, pdrInfo)
	var pdrToStore N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN6PdrWithSdf(bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp6.Lookup, ipv6, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp6.Put(ipv6, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) UpdateDownlinkPdrIp6(ipv6 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Update PDR Ipv6 Downlink: ipv6=%s, pdrInfo=%+v", ipv6, pdrInfo)
	var pdrToStore N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN6PdrWithSdf(bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp6.Lookup, ipv6, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp6.Update(ipv6, unsafe.Pointer(&pdrToStore), ebpf.UpdateExist)
}

func (bpfObjects *BpfObjects) DeleteDownlinkPdrIp6(ipv6 net.IP) error {
	logger.UpfLog.Debugf("EBPF: Delete PDR Ipv6 Downlink: ipv6=%s", ipv6)
	return bpfObjects.N6EntrypointObjects.PdrMapDownlinkIp6.Delete(ipv6)
}

func ToN6EntrypointPdrInfo(pdrInfo PdrInfo) N6EntrypointPdrInfo {
	var pdrToStore N6EntrypointPdrInfo
	pdrToStore.OuterHeaderRemoval = pdrInfo.OuterHeaderRemoval
	pdrToStore.FarId = pdrInfo.FarId
	pdrToStore.QerId = pdrInfo.QerId
	return pdrToStore
}

func CombineN6PdrWithSdf(defaultPdr *N6EntrypointPdrInfo, sdfPdr PdrInfo) N6EntrypointPdrInfo {
	var pdrToStore N6EntrypointPdrInfo
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
	pdrToStore.SdfRules.QerId = sdfPdr.QerId // (example assignment; adjust as needed)
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
