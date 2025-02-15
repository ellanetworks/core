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

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf N3Entrypoint 	xdp/n3_entrypoint.c -- -I. -O2 -Wall
//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf N6Entrypoint 	xdp/n6_entrypoint.c -- -I. -O2 -Wall

type BpfObjects struct {
	N3EntrypointObjects
	N6EntrypointObjects

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
			PinPath: pinPath,
		},
	}

	// Load N3Entrypoint objects
	if err := LoadAllObjects(&collectionOptions,
		Loader{LoadN3EntrypointObjects, &bpfObjects.N3EntrypointObjects}); err != nil {
		return err
	}
	// Load N6Entrypoint objects
	if err := LoadAllObjects(&collectionOptions,
		Loader{LoadN6EntrypointObjects, &bpfObjects.N6EntrypointObjects}); err != nil {
		return err
	}

	return nil
}

// Close both the N3 and N6 objects.
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

// LoadAllObjects runs each loader function with the provided options.
func LoadAllObjects(opts *ebpf.CollectionOptions, loaders ...Loader) error {
	for _, loader := range loaders {
		if err := loader.LoaderFunc(loader.object, opts); err != nil {
			return err
		}
	}
	return nil
}

// CloseAllObjects calls Close() on each provided io.Closer.
func CloseAllObjects(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// ResizeEbpfMap is unchanged.
func ResizeEbpfMap(eMap **ebpf.Map, eProg *ebpf.Program, newSize uint32) error {
	mapInfo, err := (*eMap).Info()
	if err != nil {
		logger.UpfLog.Infof("Failed get ebpf map info: %s", err)
		return err
	}
	mapInfo.MaxEntries = newSize
	// Create a new MapSpec using the information from MapInfo
	mapSpec := &ebpf.MapSpec{
		Name:       mapInfo.Name,
		Type:       mapInfo.Type,
		KeySize:    mapInfo.KeySize,
		ValueSize:  mapInfo.ValueSize,
		MaxEntries: mapInfo.MaxEntries,
		Flags:      mapInfo.Flags,
	}
	if err != nil {
		logger.UpfLog.Infof("Failed to close old ebpf map: %s, %+v", err, *eMap)
		return err
	}

	// Unpin the old map
	err = (*eMap).Unpin()
	if err != nil {
		logger.UpfLog.Infof("Failed to unpin old ebpf map: %s, %+v", err, *eMap)
		return err
	}

	// Close the old map
	err = (*eMap).Close()
	if err != nil {
		logger.UpfLog.Infof("Failed to close old ebpf map: %s, %+v", err, *eMap)
		return err
	}

	// Old map will be garbage collected sometime after this point

	*eMap, err = ebpf.NewMapWithOptions(mapSpec, ebpf.MapOptions{})
	if err != nil {
		logger.UpfLog.Infof("Failed to create resized ebpf map: %s", err)
		return err
	}
	err = eProg.BindMap(*eMap)
	if err != nil {
		logger.UpfLog.Infof("Failed to bind resized ebpf map: %s", err)
		return err
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
