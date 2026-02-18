package ebpf

import (
	"errors"
	"io"

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

func (bpfObjects *BpfObjects) Close() error {
	return CloseAllObjects(
		&bpfObjects.N3N6EntrypointObjects,
	)
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
