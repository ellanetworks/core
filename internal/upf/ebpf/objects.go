package ebpf

import (
	"errors"
	"io"
	"os"

	"github.com/RoaringBitmap/roaring"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/config"

	"github.com/cilium/ebpf"
)

//
// Supported BPF_CFLAGS:
// 	- ENABLE_LOG:
//		- enables debug output to tracepipe (`bpftool prog tracelog`)
// 	- ENABLE_ROUTE_CACHE
//		- enable routing decision cache
//

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf IpEntrypoint 	xdp/n3n6_entrypoint.c -- -I. -O2 -Wall -g

type BpfObjects struct {
	IpEntrypointObjects

	FarIdTracker *IdTracker
	QerIdTracker *IdTracker
}

func NewBpfObjects() *BpfObjects {
	return &BpfObjects{
		FarIdTracker: NewIdTracker(config.Conf.FarMapSize),
		QerIdTracker: NewIdTracker(config.Conf.QerMapSize),
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
		Loader{LoadIpEntrypointObjects, &bpfObjects.IpEntrypointObjects})
}

func (bpfObjects *BpfObjects) Close() error {
	return CloseAllObjects(
		&bpfObjects.IpEntrypointObjects,
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

type IdTracker struct {
	bitmap  *roaring.Bitmap
	maxSize uint32
}

func NewIdTracker(size uint32) *IdTracker {
	newBitmap := roaring.NewBitmap()
	newBitmap.Flip(0, uint64(size))

	return &IdTracker{
		bitmap:  newBitmap,
		maxSize: size,
	}
}

func (t *IdTracker) GetNext() (next uint32, err error) {
	i := t.bitmap.Iterator()
	if i.HasNext() {
		next := i.Next()
		t.bitmap.Remove(next)
		return next, nil
	}

	return 0, errors.New("pool is empty")
}

func (t *IdTracker) Release(id uint32) {
	if id >= t.maxSize {
		return
	}

	t.bitmap.Add(id)
}
