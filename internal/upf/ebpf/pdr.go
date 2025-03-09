package ebpf

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
)

// The BPF_ARRAY map type has no delete operation. The only way to delete an element is to replace it with a new one.

type PdrInfo struct {
	OuterHeaderRemoval uint8
	FarID              uint32
	QerID              uint32
	SdfFilter          *SdfFilter
}

type SdfFilter struct {
	Protocol     uint8 // 0: icmp, 1: ip, 2: tcp, 3: udp, 4: icmp6
	SrcAddress   IPWMask
	SrcPortRange PortRange
	DstAddress   IPWMask
	DstPortRange PortRange
}

type IPWMask struct {
	Type uint8 // 0: any, 1: ip4, 2: ip6
	IP   net.IP
	Mask net.IPMask
}

type PortRange struct {
	LowerBound uint16
	UpperBound uint16
}

func PreprocessN3PdrWithSdf(lookup func(interface{}, interface{}) error, key interface{}, pdrInfo PdrInfo) (N3EntrypointN3PdrInfo, error) {
	var defaultPdr N3EntrypointN3PdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombineN3PdrWithSdf(nil, pdrInfo), nil
	}

	return CombineN3PdrWithSdf(&defaultPdr, pdrInfo), nil
}

func PreprocessN6PdrWithSdf(lookup func(interface{}, interface{}) error, key interface{}, pdrInfo PdrInfo) (N6EntrypointPdrInfo, error) {
	var defaultPdr N6EntrypointPdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombineN6PdrWithSdf(nil, pdrInfo), nil
	}

	return CombineN6PdrWithSdf(&defaultPdr, pdrInfo), nil
}

func (bpfObjects *BpfObjects) PutPdrUplink(teid uint32, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Put PDR Uplink: teid=%d, pdrInfo=%+v", teid, pdrInfo)
	var pdrToStore N3EntrypointN3PdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN3PdrWithSdf(bpfObjects.N3EntrypointMaps.N3PdrMapUplinkIp4.Lookup, teid, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN3EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N3EntrypointMaps.N3PdrMapUplinkIp4.Put(teid, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) PutPdrDownlink(ipv4 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Put PDR Downlink: ipv4=%s, pdrInfo=%+v", ipv4, pdrInfo)
	var pdrToStore N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN6PdrWithSdf(bpfObjects.N6EntrypointMaps.PdrMapDownlinkIp4.Lookup, ipv4, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N6EntrypointMaps.PdrMapDownlinkIp4.Put(ipv4, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) DeletePdrUplink(teid uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete PDR Uplink: teid=%d", teid)
	return bpfObjects.N3EntrypointMaps.N3PdrMapUplinkIp4.Delete(teid)
}

func (bpfObjects *BpfObjects) DeletePdrDownlink(ipv4 net.IP) error {
	logger.UpfLog.Debugf("EBPF: Delete PDR Downlink: ipv4=%s", ipv4)
	return bpfObjects.N6EntrypointMaps.PdrMapDownlinkIp4.Delete(ipv4)
}

func (bpfObjects *BpfObjects) PutDownlinkPdrIP6(ipv6 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debugf("EBPF: Put PDR Ipv6 Downlink: ipv6=%s, pdrInfo=%+v", ipv6, pdrInfo)
	var pdrToStore N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN6PdrWithSdf(bpfObjects.N6EntrypointMaps.PdrMapDownlinkIp6.Lookup, ipv6, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N6EntrypointMaps.PdrMapDownlinkIp6.Put(ipv6, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) DeleteDownlinkPdrIP6(ipv6 net.IP) error {
	logger.UpfLog.Debugf("EBPF: Delete PDR Ipv6 Downlink: ipv6=%s", ipv6)
	return bpfObjects.N6EntrypointMaps.PdrMapDownlinkIp6.Delete(ipv6)
}

type FarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	TeID                  uint32
	RemoteIP              uint32
	LocalIP               uint32
	TransportLevelMarking uint16
}

func (f FarInfo) MarshalJSON() ([]byte, error) {
	remoteIP := make(net.IP, 4)
	localIP := make(net.IP, 4)
	binary.LittleEndian.PutUint32(remoteIP, f.RemoteIP)
	binary.LittleEndian.PutUint32(localIP, f.LocalIP)
	data := map[string]interface{}{
		"action":                  f.Action,
		"outer_header_creation":   f.OuterHeaderCreation,
		"teid":                    f.TeID,
		"remote_ip":               remoteIP.String(),
		"local_ip":                localIP.String(),
		"transport_level_marking": f.TransportLevelMarking,
	}
	return json.Marshal(data)
}

func (bpfObjects *BpfObjects) NewFar(farInfo FarInfo) (uint32, error) {
	internalID, err := bpfObjects.FarIDTracker.GetNext()
	if err != nil {
		return 0, err
	}
	logger.UpfLog.Debugf("EBPF: Put FAR: internalID=%d, qerInfo=%+v", internalID, farInfo)
	err = bpfObjects.N3EntrypointMaps.FarMap.Put(internalID, unsafe.Pointer(&farInfo))
	if err != nil {
		return 0, err
	}
	err = bpfObjects.N6EntrypointMaps.FarMap.Put(internalID, unsafe.Pointer(&farInfo))
	if err != nil {
		return 0, err
	}
	return internalID, nil
}

func (bpfObjects *BpfObjects) UpdateFar(internalID uint32, farInfo FarInfo) error {
	logger.UpfLog.Debugf("EBPF: Update FAR: internalID=%d, farInfo=%+v", internalID, farInfo)
	err := bpfObjects.N3EntrypointMaps.FarMap.Update(internalID, unsafe.Pointer(&farInfo), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	err = bpfObjects.N6EntrypointMaps.FarMap.Update(internalID, unsafe.Pointer(&farInfo), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	return nil
}

func (bpfObjects *BpfObjects) DeleteFar(internalID uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete FAR: internalID=%d", internalID)
	bpfObjects.FarIDTracker.Release(internalID)
	err := bpfObjects.N3EntrypointMaps.FarMap.Update(internalID, unsafe.Pointer(&FarInfo{}), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	err = bpfObjects.N6EntrypointMaps.FarMap.Update(internalID, unsafe.Pointer(&FarInfo{}), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	return nil
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

func (bpfObjects *BpfObjects) NewQer(qerInfo QerInfo) (uint32, error) {
	internalID, err := bpfObjects.QerIDTracker.GetNext()
	if err != nil {
		return 0, err
	}
	logger.UpfLog.Debugf("EBPF: Put QER: internalID=%d, qerInfo=%+v", internalID, qerInfo)
	err = bpfObjects.N3EntrypointMaps.QerMap.Put(internalID, unsafe.Pointer(&qerInfo))
	if err != nil {
		return 0, err
	}
	err = bpfObjects.N6EntrypointMaps.QerMap.Put(internalID, unsafe.Pointer(&qerInfo))
	if err != nil {
		return 0, err
	}
	return internalID, nil
}

func (bpfObjects *BpfObjects) UpdateQer(internalID uint32, qerInfo QerInfo) error {
	logger.UpfLog.Debugf("EBPF: Update QER: internalID=%d, qerInfo=%+v", internalID, qerInfo)
	err := bpfObjects.N3EntrypointMaps.QerMap.Update(internalID, unsafe.Pointer(&qerInfo), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	err = bpfObjects.N6EntrypointMaps.QerMap.Update(internalID, unsafe.Pointer(&qerInfo), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	return nil
}

func (bpfObjects *BpfObjects) DeleteQer(internalID uint32) error {
	logger.UpfLog.Debugf("EBPF: Delete QER: internalID=%d", internalID)
	bpfObjects.QerIDTracker.Release(internalID)
	err := bpfObjects.N3EntrypointMaps.QerMap.Update(internalID, unsafe.Pointer(&QerInfo{}), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	err = bpfObjects.N6EntrypointMaps.QerMap.Update(internalID, unsafe.Pointer(&QerInfo{}), ebpf.UpdateExist)
	if err != nil {
		return err
	}
	return nil
}

func CombineN3PdrWithSdf(defaultPdr *N3EntrypointN3PdrInfo, sdfPdr PdrInfo) N3EntrypointN3PdrInfo {
	var pdrToStore N3EntrypointN3PdrInfo
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
	pdrToStore.N3SdfRules.N3SdfFilter.Protocol = sdfPdr.SdfFilter.Protocol
	pdrToStore.N3SdfRules.N3SdfFilter.SrcAddr.Type = sdfPdr.SdfFilter.SrcAddress.Type
	pdrToStore.N3SdfRules.N3SdfFilter.SrcAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.IP)
	pdrToStore.N3SdfRules.N3SdfFilter.SrcAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.Mask)
	pdrToStore.N3SdfRules.N3SdfFilter.SrcPort.LowerBound = sdfPdr.SdfFilter.SrcPortRange.LowerBound
	pdrToStore.N3SdfRules.N3SdfFilter.SrcPort.UpperBound = sdfPdr.SdfFilter.SrcPortRange.UpperBound
	pdrToStore.N3SdfRules.N3SdfFilter.DstAddr.Type = sdfPdr.SdfFilter.DstAddress.Type
	pdrToStore.N3SdfRules.N3SdfFilter.DstAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.DstAddress.IP)
	pdrToStore.N3SdfRules.N3SdfFilter.DstAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.DstAddress.Mask)
	pdrToStore.N3SdfRules.N3SdfFilter.DstPort.LowerBound = sdfPdr.SdfFilter.DstPortRange.LowerBound
	pdrToStore.N3SdfRules.N3SdfFilter.DstPort.UpperBound = sdfPdr.SdfFilter.DstPortRange.UpperBound
	pdrToStore.N3SdfRules.OuterHeaderRemoval = sdfPdr.OuterHeaderRemoval
	pdrToStore.N3SdfRules.FarId = sdfPdr.FarID
	pdrToStore.N3SdfRules.QerId = sdfPdr.QerID
	return pdrToStore
}

func CombineN6PdrWithSdf(defaultPdr *N6EntrypointPdrInfo, sdfPdr PdrInfo) N6EntrypointPdrInfo {
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
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.IP)
	pdrToStore.SdfRules.SdfFilter.SrcAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.SrcAddress.Mask)
	pdrToStore.SdfRules.SdfFilter.SrcPort.LowerBound = sdfPdr.SdfFilter.SrcPortRange.LowerBound
	pdrToStore.SdfRules.SdfFilter.SrcPort.UpperBound = sdfPdr.SdfFilter.SrcPortRange.UpperBound
	pdrToStore.SdfRules.SdfFilter.DstAddr.Type = sdfPdr.SdfFilter.DstAddress.Type
	pdrToStore.SdfRules.SdfFilter.DstAddr.Ip = Copy16Ip(sdfPdr.SdfFilter.DstAddress.IP)
	pdrToStore.SdfRules.SdfFilter.DstAddr.Mask = Copy16Ip(sdfPdr.SdfFilter.DstAddress.Mask)
	pdrToStore.SdfRules.SdfFilter.DstPort.LowerBound = sdfPdr.SdfFilter.DstPortRange.LowerBound
	pdrToStore.SdfRules.SdfFilter.DstPort.UpperBound = sdfPdr.SdfFilter.DstPortRange.UpperBound
	pdrToStore.SdfRules.OuterHeaderRemoval = sdfPdr.OuterHeaderRemoval
	pdrToStore.SdfRules.FarId = sdfPdr.FarID
	pdrToStore.SdfRules.QerId = sdfPdr.QerID
	return pdrToStore
}

func ToN3EntrypointPdrInfo(defaultPdr PdrInfo) N3EntrypointN3PdrInfo {
	var pdrToStore N3EntrypointN3PdrInfo
	pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
	pdrToStore.FarId = defaultPdr.FarID
	pdrToStore.QerId = defaultPdr.QerID
	return pdrToStore
}

func ToN6EntrypointPdrInfo(defaultPdr PdrInfo) N6EntrypointPdrInfo {
	var pdrToStore N6EntrypointPdrInfo
	pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
	pdrToStore.FarId = defaultPdr.FarID
	pdrToStore.QerId = defaultPdr.QerID
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
