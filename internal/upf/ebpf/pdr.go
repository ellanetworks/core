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

func (bpfObjects *BpfObjects) PutPdrUplink(teid uint32, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("Put PDR Uplink", zap.Uint32("teid", teid), zap.Any("pdrInfo", pdrInfo))

	pdrToStore := ToN3N6EntrypointPdrInfo(pdrInfo)

	return bpfObjects.N3N6EntrypointMaps.PdrsUplink.Put(teid, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) PutPdrDownlink(ipv4 net.IP, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("Put PDR Downlink", zap.String("ipv4", ipv4.String()), zap.Any("pdrInfo", pdrInfo))

	pdrToStore := ToN3N6EntrypointPdrInfo(pdrInfo)

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

	pdrToStore := ToN3N6EntrypointPdrInfo(pdrInfo)

	return bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp6.Put(ipv6, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) DeleteDownlinkPdrIP6(ipv6 net.IP) error {
	logger.UpfLog.Debug("Delete PDR Ipv6 Downlink", zap.String("ipv6", ipv6.String()))
	return bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp6.Delete(ipv6)
}

type FarInfo struct {
	Action              uint8
	OuterHeaderCreation uint8
	TeID                uint32
	RemoteIP            uint32
	LocalIP             uint32
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

func addRemoteIPToNeigh(ctx context.Context, remoteIP uint32) {
	ip_bytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(ip_bytes, remoteIP)
	ip := net.IP(ip_bytes)

	if err := kernel.AddNeighbour(ctx, ip); err != nil {
		logger.UpfLog.Warn("could not add gnb IP to neighbour list", zap.String("IP", ip.String()), zap.Error(err))
	}
}
