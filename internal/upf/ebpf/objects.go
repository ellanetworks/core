package ebpf

import (
	"errors"
	"io"
	"os"

	"github.com/RoaringBitmap/roaring"
	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
)

//
// Supported BPF_CFLAGS:
// 	- ENABLE_LOG:
//		- enables debug output to tracepipe (`bpftool prog tracelog`)
//
// Usage: export BPF_CFLAGS="-DENABLE_LOG"

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N3Entrypoint 	xdp/n3_bpf.c -- -I. -O2 -Wall -g
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf N6Entrypoint 	xdp/n6_bpf.c -- -I. -O2 -Wall -g

type BpfObjects struct {
	N3EntrypointObjects
	N6EntrypointObjects

	FarIDTracker *IDTracker
	QerIDTracker *IDTracker
}

func NewBpfObjects(farMapSize uint32, qerMapSize uint32) *BpfObjects {
	return &BpfObjects{
		FarIDTracker: NewIDTracker(farMapSize),
		QerIDTracker: NewIDTracker(qerMapSize),
	}
}

func (bpfObjects *BpfObjects) Load() error {
	pinPath := "/sys/fs/bpf/upf_pipeline"
	if err := os.MkdirAll(pinPath, 0o750); err != nil {
		logger.UpfLog.Infof("failed to create bpf fs subpath: %+v", err)
		return err
	}

	collectionOptions := ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			// Pin the map to the BPF filesystem and configure the
			// library to automatically re-write it in the BPF
			// program, so it can be re-used if it already exists or
			// create it if not
			PinPath: pinPath,
		},
	}

	return LoadAllObjects(&collectionOptions,
		Loader{LoadN3EntrypointObjects, &bpfObjects.N3EntrypointObjects},
		Loader{LoadN6EntrypointObjects, &bpfObjects.N6EntrypointObjects})
}

func (bpfObjects *BpfObjects) Close() error {
	return CloseAllObjects(
		&bpfObjects.N3EntrypointObjects,
		&bpfObjects.N6EntrypointObjects,
	)
}

type (
	LoaderFunc func(obj interface{}, opts *ebpf.CollectionOptions) error
	Loader     struct {
		LoaderFunc
		object interface{}
	}
)

func LoadAllObjects(opts *ebpf.CollectionOptions, loaders ...Loader) error {
	for _, loader := range loaders {
		if err := loader.LoaderFunc(loader.object, opts); err != nil {
			return err
		}
	}
	return nil
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
