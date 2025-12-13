package ebpf

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// The BPF_ARRAY map type has no delete operation. The only way to delete an element is to replace it with a new one.

type PdrInfo struct {
	SEID               uint64
	OuterHeaderRemoval uint8
	PdrID              uint32
	FarID              uint32
	QerID              uint32
	UrrID              uint32
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

func PreprocessN3N6PdrWithSdf(lookup func(any, any) error, key any, pdrInfo PdrInfo) (N3N6EntrypointPdrInfo, error) {
	var defaultPdr N3N6EntrypointPdrInfo
	if err := lookup(key, &defaultPdr); err != nil {
		return CombineN3N6PdrWithSdf(nil, pdrInfo), nil
	}

	return CombineN3N6PdrWithSdf(&defaultPdr, pdrInfo), nil
}

func (bpfObjects *BpfObjects) PutPdrUplink(teid uint32, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("Put PDR Uplink", zap.Uint32("teid", teid), zap.Any("pdrInfo", pdrInfo))
	var pdrToStore N3N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN3N6PdrWithSdf(bpfObjects.N3N6EntrypointMaps.PdrsUplink.Lookup, teid, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN3N6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N3N6EntrypointMaps.PdrsUplink.Put(teid, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) PutPdrDownlink(ipv4 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("Put PDR Downlink", zap.String("ipv4", ipv4.String()), zap.Any("pdrInfo", pdrInfo))
	var pdrToStore N3N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN3N6PdrWithSdf(bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp4.Lookup, ipv4, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN3N6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp4.Put(ipv4, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) DeletePdrUplink(teid uint32) error {
	logger.UpfLog.Debug("Delete PDR Uplink", zap.Uint32("teid", teid))
	return bpfObjects.N3N6EntrypointMaps.PdrsUplink.Delete(teid)
}

func (bpfObjects *BpfObjects) DeletePdrDownlink(ipv4 net.IP) error {
	logger.UpfLog.Debug("Delete PDR Downlink", zap.String("ipv4", ipv4.String()))
	return bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp4.Delete(ipv4)
}

func (bpfObjects *BpfObjects) PutDownlinkPdrIP6(ipv6 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("EBPF: Put PDR Ipv6 Downlink", zap.String("ipv6", ipv6.String()), zap.Any("pdrInfo", pdrInfo))
	var pdrToStore N3N6EntrypointPdrInfo
	var err error
	if pdrInfo.SdfFilter != nil {
		if pdrToStore, err = PreprocessN3N6PdrWithSdf(bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp6.Lookup, ipv6, pdrInfo); err != nil {
			return err
		}
	} else {
		pdrToStore = ToN3N6EntrypointPdrInfo(pdrInfo)
	}
	return bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp6.Put(ipv6, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) DeleteDownlinkPdrIP6(ipv6 net.IP) error {
	logger.UpfLog.Debug("Delete PDR Ipv6 Downlink", zap.String("ipv6", ipv6.String()))
	return bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp6.Delete(ipv6)
}

type FarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	TeID                  uint32
	RemoteIP              uint32
	LocalIP               uint32
	TransportLevelMarking uint16
}

func (bpfObjects *BpfObjects) NewFar(ctx context.Context, farID uint32, farInfo FarInfo) error {
	go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

	err := bpfObjects.N3N6EntrypointMaps.FarMap.Put(farID, unsafe.Pointer(&farInfo))
	if err != nil {
		return fmt.Errorf("failed to put FAR: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) UpdateFar(ctx context.Context, id uint32, farInfo FarInfo) error {
	go addRemoteIPToNeigh(ctx, farInfo.RemoteIP)

	err := bpfObjects.N3N6EntrypointMaps.FarMap.Update(id, unsafe.Pointer(&farInfo), ebpf.UpdateExist)
	if err != nil {
		return fmt.Errorf("failed to update FAR: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) DeleteFar(id uint32) error {
	err := bpfObjects.N3N6EntrypointMaps.FarMap.Update(id, unsafe.Pointer(&FarInfo{}), ebpf.UpdateExist)
	if err != nil {
		return fmt.Errorf("failed to delete FAR: %w", err)
	}

	return nil
}

type QerInfo struct {
	GateStatusUL uint8
	GateStatusDL uint8
	Qfi          uint8
	MaxBitrateUL uint64
	MaxBitrateDL uint64
	StartUL      uint64
	StartDL      uint64
}

func (bpfObjects *BpfObjects) NewQer(id uint32, qerInfo QerInfo) error {
	err := bpfObjects.N3N6EntrypointMaps.QerMap.Put(id, unsafe.Pointer(&qerInfo))
	if err != nil {
		return fmt.Errorf("failed to create QER: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) UpdateQer(id uint32, qerInfo QerInfo) error {
	err := bpfObjects.N3N6EntrypointMaps.QerMap.Update(id, unsafe.Pointer(&qerInfo), ebpf.UpdateExist)
	if err != nil {
		return fmt.Errorf("failed to update QER: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) DeleteQer(id uint32) error {
	err := bpfObjects.N3N6EntrypointMaps.QerMap.Update(id, unsafe.Pointer(&QerInfo{}), ebpf.UpdateExist)
	if err != nil {
		return fmt.Errorf("failed to delete QER: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) NewUrr(id uint32) error {
	zeroVals := make([]uint64, runtime.NumCPU())

	err := bpfObjects.N3N6EntrypointMaps.UrrMap.Put(id, zeroVals)
	if err != nil {
		return fmt.Errorf("failed to put urr id %d: %w", id, err)
	}

	return nil
}

func CombineN3N6PdrWithSdf(defaultPdr *N3N6EntrypointPdrInfo, sdfPdr PdrInfo) N3N6EntrypointPdrInfo {
	var pdrToStore N3N6EntrypointPdrInfo
	// Default mapping options.
	if defaultPdr != nil {
		pdrToStore.LocalSeid = defaultPdr.LocalSeid
		pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
		pdrToStore.PdrId = defaultPdr.PdrId
		pdrToStore.FarId = defaultPdr.FarId
		pdrToStore.QerId = defaultPdr.QerId
		pdrToStore.UrrId = defaultPdr.UrrId
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
	pdrToStore.SdfRules.UrrId = sdfPdr.UrrID
	return pdrToStore
}

func ToN3N6EntrypointPdrInfo(defaultPdr PdrInfo) N3N6EntrypointPdrInfo {
	var pdrToStore N3N6EntrypointPdrInfo
	pdrToStore.LocalSeid = defaultPdr.SEID
	pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
	pdrToStore.PdrId = defaultPdr.PdrID
	pdrToStore.FarId = defaultPdr.FarID
	pdrToStore.QerId = defaultPdr.QerID
	pdrToStore.UrrId = defaultPdr.UrrID
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

func addRemoteIPToNeigh(ctx context.Context, remoteIP uint32) {
	ip_bytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(ip_bytes, remoteIP)
	ip := net.IP(ip_bytes)

	if err := kernel.AddNeighbour(ctx, ip); err != nil {
		logger.UpfLog.Warn("could not add gnb IP to neighbour list", zap.String("IP", ip.String()), zap.Error(err))
	}
}
