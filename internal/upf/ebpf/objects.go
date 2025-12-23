package ebpf

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

//
// Supported BPF_CFLAGS:
// 	- ENABLE_LOG:
//		- enables debug output to tracepipe (`bpftool prog tracelog`)
//
// Usage: export BPF_CFLAGS="-DENABLE_LOG"

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3N6Entrypoint xdp/n3n6_bpf.c -- -I. -O2 -Wall -Werror -g

const (
	PinPath = "/sys/fs/bpf/upf_pipeline"
)

type DataNotification struct {
	LocalSEID uint64
	PdrID     uint16
	QFI       uint8
}

type BpfObjects struct {
	N3N6EntrypointObjects

	Masquerade       bool
	N3InterfaceIndex uint32
	N6InterfaceIndex uint32
	N3Vlan           uint32
	N6Vlan           uint32
	pagingList       map[DataNotification]bool
}

func NewBpfObjects(masquerade bool, n3ifindex int, n6ifindex int, n3vlan uint32, n6vlan uint32) *BpfObjects {
	return &BpfObjects{
		Masquerade:       masquerade,
		N3InterfaceIndex: uint32(n3ifindex),
		N6InterfaceIndex: uint32(n6ifindex),
		N3Vlan:           n3vlan,
		N6Vlan:           n6vlan,
		pagingList:       make(map[DataNotification]bool),
	}
}

func PinMaps() error {
	if err := os.MkdirAll(PinPath, 0o750); err != nil {
		return fmt.Errorf("failed to create bpf fs subpath: %w", err)
	}

	return nil
}

func (bpfObjects *BpfObjects) Load() error {
	collectionOptions := ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			// Pin the map to the BPF filesystem and configure the
			// library to automatically re-write it in the BPF
			// program, so it can be re-used if it already exists or
			// create it if not
			PinPath: PinPath,
		},
	}

	n3n6Spec, err := LoadN3N6Entrypoint()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}
	if err := bpfObjects.loadAndAssignFromSpec(n3n6Spec, &bpfObjects.N3N6EntrypointObjects, &collectionOptions); err != nil {
		logger.UpfLog.Error("failed to load N3/N6 program", zap.Error(err))
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			logger.UpfLog.Debug("verifier logs", zap.Error(ve))
		}
		return err
	}

	return nil
}

func (bpfObjects *BpfObjects) loadAndAssignFromSpec(spec *ebpf.CollectionSpec, to any, opts *ebpf.CollectionOptions) error {
	if err := spec.Variables["masquerade"].Set(bpfObjects.Masquerade); err != nil {
		return fmt.Errorf("failed to set masquerade value: %w", err)
	}
	if err := spec.Variables["n3_ifindex"].Set(bpfObjects.N3InterfaceIndex); err != nil {
		return fmt.Errorf("failed to set n3 interface index: %w", err)
	}
	if err := spec.Variables["n6_ifindex"].Set(bpfObjects.N6InterfaceIndex); err != nil {
		return fmt.Errorf("failed to set n6 interface index: %w", err)
	}
	if err := spec.Variables["n3_vlan"].Set(bpfObjects.N3Vlan); err != nil {
		return fmt.Errorf("failed to set n3 vlan id: %w", err)
	}
	if err := spec.Variables["n6_vlan"].Set(bpfObjects.N6Vlan); err != nil {
		return fmt.Errorf("failed to set n6 vlan id: %w", err)
	}
	if err := spec.LoadAndAssign(to, opts); err != nil {
		return fmt.Errorf("failed to load eBPF program: %w", err)
	}
	return nil
}

func (bpfObjects *BpfObjects) Close() error {
	bpfObjects.unpinMaps()
	return CloseAllObjects(
		&bpfObjects.N3N6EntrypointObjects,
	)
}

func (bpfObjects *BpfObjects) unpinMaps() {
	if err := bpfObjects.N3N6EntrypointMaps.UplinkRouteStats.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin uplink_route_stats map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3N6EntrypointMaps.PdrsUplink.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin pdrs_uplink map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3N6EntrypointMaps.DownlinkRouteStats.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin downlink_route_stats map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp4.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin pdrs_downlink_ip4 map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3N6EntrypointMaps.PdrsDownlinkIp6.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin pdrs_downlink_ip6 map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3N6EntrypointMaps.DownlinkStatistics.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin downlink_statistics map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3N6EntrypointMaps.UplinkStatistics.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin uplink_statistics map, state could be left behind: %v", zap.Error(err))
	}
}

func (bpfObjects *BpfObjects) IsAlreadyNotified(d DataNotification) bool {
	_, ok := bpfObjects.pagingList[d]
	return ok
}

func (bpfObjects *BpfObjects) MarkNotified(d DataNotification) {
	bpfObjects.pagingList[d] = true
}

func (bpfObjects *BpfObjects) ClearNotified(seid uint64, pdrid uint16, qfi uint8) {
	delete(bpfObjects.pagingList, DataNotification{LocalSEID: seid, PdrID: pdrid, QFI: qfi})
}

func CloseAllObjects(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}
