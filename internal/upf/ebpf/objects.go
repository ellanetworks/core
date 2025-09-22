package ebpf

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/RoaringBitmap/roaring"
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

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3Entrypoint xdp/n3_bpf.c -- -I. -O2 -Wall -Werror -g
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N6Entrypoint xdp/n6_bpf.c -- -I. -O2 -Wall -Werror -g

const (
	PinPath = "/sys/fs/bpf/upf_pipeline"
)

type BpfObjects struct {
	N3EntrypointObjects
	N6EntrypointObjects

	FarIDTracker *IDTracker
	QerIDTracker *IDTracker
	Masquerade   bool
}

func NewBpfObjects(farMapSize uint32, qerMapSize uint32, masquerade bool) *BpfObjects {
	return &BpfObjects{
		FarIDTracker: NewIDTracker(farMapSize),
		QerIDTracker: NewIDTracker(qerMapSize),
		Masquerade:   masquerade,
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

	n3Spec, err := LoadN3Entrypoint()
	if err != nil {
		logger.UpfLog.Error("failed to load N3 spec", zap.Error(err))
		return err
	}
	if err := bpfObjects.loadAndAssignFromSpec(n3Spec, &bpfObjects.N3EntrypointObjects, &collectionOptions); err != nil {
		logger.UpfLog.Error("failed to load N3 program", zap.Error(err))
		return err
	}

	n6Spec, err := LoadN6Entrypoint()
	if err != nil {
		logger.UpfLog.Error("failed to load N6 spec", zap.Error(err))
		return err
	}
	if err := bpfObjects.loadAndAssignFromSpec(n6Spec, &bpfObjects.N6EntrypointObjects, &collectionOptions); err != nil {
		logger.UpfLog.Error("failed to load N6 program", zap.Error(err))
		return err
	}

	return nil
}

func (bpfObjects *BpfObjects) loadAndAssignFromSpec(spec *ebpf.CollectionSpec, to any, opts *ebpf.CollectionOptions) error {
	if err := spec.Variables["masquerade"].Set(bpfObjects.Masquerade); err != nil {
		logger.UpfLog.Error("failed to set masquerade value", zap.Error(err))
		return err
	}
	if err := spec.LoadAndAssign(to, opts); err != nil {
		logger.UpfLog.Error("failed to load eBPF program", zap.Error(err))
		return err
	}
	return nil
}

func (bpfObjects *BpfObjects) Close() error {
	bpfObjects.unpinMaps()
	return CloseAllObjects(
		&bpfObjects.N3EntrypointObjects,
		&bpfObjects.N6EntrypointObjects,
	)
}

func (bpfObjects *BpfObjects) unpinMaps() {
	if err := bpfObjects.N3EntrypointMaps.UplinkRouteStats.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin uplink_route_stats map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N3EntrypointMaps.PdrsUplink.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin pdrs_uplink map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N6EntrypointMaps.DownlinkRouteStats.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin downlink_route_stats map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N6EntrypointMaps.PdrsDownlinkIp4.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin pdrs_downlink_ip4 map, state could be left behind: %v", zap.Error(err))
	}
	if err := bpfObjects.N6EntrypointMaps.PdrsDownlinkIp6.Unpin(); err != nil {
		logger.UpfLog.Warn("failed to unpin pdrs_downlink_ip6 map, state could be left behind: %v", zap.Error(err))
	}
}

func CloseAllObjects(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

type IDTracker struct {
	bitmap  *roaring.Bitmap
	maxSize uint32
}

func NewIDTracker(size uint32) *IDTracker {
	newBitmap := roaring.NewBitmap()
	newBitmap.Flip(0, uint64(size))

	return &IDTracker{
		bitmap:  newBitmap,
		maxSize: size,
	}
}

func (t *IDTracker) GetNext() (next uint32, err error) {
	i := t.bitmap.Iterator()
	if i.HasNext() {
		next := i.Next()
		t.bitmap.Remove(next)
		return next, nil
	}

	return 0, errors.New("pool is empty")
}

func (t *IDTracker) Release(id uint32) {
	if id >= t.maxSize {
		return
	}

	t.bitmap.Add(id)
}
