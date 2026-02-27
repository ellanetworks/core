package ebpf

import (
	"errors"
	"io"
	"sync"

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

type DataNotification struct {
	LocalSEID uint64
	PdrID     uint16
	QFI       uint8
}

type BpfObjects struct {
	N3N6EntrypointObjects

	FlowAccounting   bool
	Masquerade       bool
	N3InterfaceIndex uint32
	N6InterfaceIndex uint32
	N3Vlan           uint32
	N6Vlan           uint32
	pagingMu         sync.Mutex
	pagingList       map[DataNotification]bool
}

func NewBpfObjects(flowact bool, masquerade bool, n3ifindex int, n6ifindex int, n3vlan uint32, n6vlan uint32) *BpfObjects {
	return &BpfObjects{
		FlowAccounting:   flowact,
		Masquerade:       masquerade,
		N3InterfaceIndex: uint32(n3ifindex),
		N6InterfaceIndex: uint32(n6ifindex),
		N3Vlan:           n3vlan,
		N6Vlan:           n6vlan,
		pagingList:       make(map[DataNotification]bool),
	}
}

func (bpfObjects *BpfObjects) Load() error {
	n3n6Spec, err := LoadN3N6Entrypoint()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}

	if err := bpfObjects.loadAndAssignFromSpec(n3n6Spec, &bpfObjects.N3N6EntrypointObjects, nil); err != nil {
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
	if err := spec.Variables["flowact"].Set(bpfObjects.FlowAccounting); err != nil {
		logger.UpfLog.Error("failed to set flow accounting value", zap.Error(err))
		return err
	}

	if err := spec.Variables["masquerade"].Set(bpfObjects.Masquerade); err != nil {
		logger.UpfLog.Error("failed to set masquerade value", zap.Error(err))
		return err
	}

	if err := spec.Variables["n3_ifindex"].Set(bpfObjects.N3InterfaceIndex); err != nil {
		logger.UpfLog.Error("failed to set n3 interface index", zap.Error(err))
		return err
	}

	if err := spec.Variables["n6_ifindex"].Set(bpfObjects.N6InterfaceIndex); err != nil {
		logger.UpfLog.Error("failed to set n6 interface index", zap.Error(err))
		return err
	}

	if err := spec.Variables["n3_vlan"].Set(bpfObjects.N3Vlan); err != nil {
		logger.UpfLog.Error("failed to set n3 vlan id", zap.Error(err))
		return err
	}

	if err := spec.Variables["n6_vlan"].Set(bpfObjects.N6Vlan); err != nil {
		logger.UpfLog.Error("failed to set n6 vlan id", zap.Error(err))
		return err
	}

	if err := spec.LoadAndAssign(to, opts); err != nil {
		logger.UpfLog.Error("failed to load eBPF program", zap.Error(err))
		return err
	}

	return nil
}

// LoadWithMapReplacements reloads the eBPF programs with updated global
// variable values while preserving all existing maps. This allows changing
// settings like flow accounting or masquerade without disrupting subscriber
// sessions, since session state lives in the maps.
//
// The caller must call link.Update() with the new program to atomically
// swap the XDP program on the interface.
func (bpfObjects *BpfObjects) LoadWithMapReplacements() error {
	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		logger.UpfLog.Error("failed to load N3/N6 spec", zap.Error(err))
		return err
	}

	replacements := map[string]*ebpf.Map{
		"downlink_route_stats": bpfObjects.DownlinkRouteStats,
		"downlink_statistics":  bpfObjects.DownlinkStatistics,
		"far_map":              bpfObjects.FarMap,
		"flow_stats":           bpfObjects.FlowStats,
		"nat_ct":               bpfObjects.NatCt,
		"no_neigh_map":         bpfObjects.NoNeighMap,
		"nocp_map":             bpfObjects.NocpMap,
		"pdrs_downlink_ip4":    bpfObjects.PdrsDownlinkIp4,
		"pdrs_downlink_ip6":    bpfObjects.PdrsDownlinkIp6,
		"pdrs_uplink":          bpfObjects.PdrsUplink,
		"qer_map":              bpfObjects.QerMap,
		"uplink_route_stats":   bpfObjects.UplinkRouteStats,
		"uplink_statistics":    bpfObjects.UplinkStatistics,
		"urr_map":              bpfObjects.UrrMap,
	}

	opts := &ebpf.CollectionOptions{
		MapReplacements: replacements,
	}

	var newObjects N3N6EntrypointObjects
	if err := bpfObjects.loadAndAssignFromSpec(spec, &newObjects, opts); err != nil {
		logger.UpfLog.Error("failed to load N3/N6 program with map replacements", zap.Error(err))

		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			logger.UpfLog.Debug("verifier logs", zap.Error(ve))
		}

		return err
	}

	oldProg := bpfObjects.UpfN3N6EntrypointFunc

	bpfObjects.UpfN3N6EntrypointFunc = newObjects.UpfN3N6EntrypointFunc
	bpfObjects.N3N6EntrypointVariables = newObjects.N3N6EntrypointVariables

	// Close the cloned map fds from the new objects to avoid fd leaks.
	// These are duplicates of our existing map fds pointing to the same
	// kernel-side maps, so closing them is safe.
	if err := newObjects.N3N6EntrypointMaps.Close(); err != nil {
		logger.UpfLog.Warn("failed to close cloned map fds", zap.Error(err))
	}

	// Close the old program now that it's been replaced.
	if err := oldProg.Close(); err != nil {
		logger.UpfLog.Warn("failed to close old program", zap.Error(err))
	}

	return nil
}

func (bpfObjects *BpfObjects) Close() error {
	return CloseAllObjects(
		&bpfObjects.N3N6EntrypointObjects,
	)
}

func (bpfObjects *BpfObjects) IsAlreadyNotified(d DataNotification) bool {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	_, ok := bpfObjects.pagingList[d]

	return ok
}

func (bpfObjects *BpfObjects) MarkNotified(d DataNotification) {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

	bpfObjects.pagingList[d] = true
}

func (bpfObjects *BpfObjects) ClearNotified(seid uint64, pdrid uint16, qfi uint8) {
	bpfObjects.pagingMu.Lock()
	defer bpfObjects.pagingMu.Unlock()

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
